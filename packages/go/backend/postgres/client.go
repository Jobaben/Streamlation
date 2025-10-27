package postgres

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	mu   sync.Mutex
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
}

type Config struct {
	addr     string
	user     string
	database string
}

func NewClient(ctx context.Context, databaseURL string) (*Client, error) {
	cfg, err := parseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", cfg.addr)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	client := &Client{
		conn: conn,
		r:    bufio.NewReader(conn),
		w:    bufio.NewWriter(conn),
	}

	if err := client.startup(ctx, cfg.user, cfg.database); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return client, nil
}

func parseConfig(databaseURL string) (Config, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return Config{}, fmt.Errorf("invalid database url: %w", err)
	}

	switch u.Scheme {
	case "postgres", "postgresql":
	default:
		return Config{}, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}

	port := u.Port()
	if port == "" {
		port = "5432"
	}

	user := u.User.Username()
	if user == "" {
		user = "postgres"
	}

	database := strings.TrimPrefix(u.Path, "/")
	if database == "" {
		database = user
	}

	if mode := u.Query().Get("sslmode"); mode != "" && mode != "disable" {
		return Config{}, fmt.Errorf("unsupported sslmode: %s", mode)
	}

	return Config{addr: net.JoinHostPort(host, port), user: user, database: database}, nil
}

func (c *Client) startup(ctx context.Context, user, database string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.writeStartup(user, database); err != nil {
		return err
	}

	for {
		typ, payload, err := c.readMessage(ctx)
		if err != nil {
			return err
		}

		switch typ {
		case 'R':
			if err := handleAuthentication(payload); err != nil {
				return err
			}
		case 'S', 'K', 'N':
			continue
		case 'E':
			return parseErrorResponse(payload)
		case 'Z':
			return nil
		default:
			continue
		}
	}
}

func (c *Client) writeStartup(user, database string) error {
	const protocolVersion = 196608 // 3.0

	var payload bytes.Buffer
	writeCString(&payload, "user")
	writeCString(&payload, user)
	writeCString(&payload, "database")
	writeCString(&payload, database)
	payload.WriteByte(0)

	totalLen := int32(payload.Len() + 4 + 4)
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], uint32(totalLen))
	binary.BigEndian.PutUint32(header[4:8], protocolVersion)

	if _, err := c.w.Write(header); err != nil {
		return err
	}
	if _, err := c.w.Write(payload.Bytes()); err != nil {
		return err
	}
	return c.w.Flush()
}

func handleAuthentication(payload []byte) error {
	if len(payload) < 4 {
		return errors.New("invalid authentication message")
	}
	authType := binary.BigEndian.Uint32(payload[:4])
	if authType != 0 {
		return fmt.Errorf("unsupported authentication method: %d", authType)
	}
	return nil
}

func writeCString(buf *bytes.Buffer, value string) {
	buf.WriteString(value)
	buf.WriteByte(0)
}

func (c *Client) readMessage(ctx context.Context) (byte, []byte, error) {
	if err := c.applyDeadline(ctx); err != nil {
		return 0, nil, err
	}

	header := make([]byte, 5)
	if _, err := io.ReadFull(c.r, header); err != nil {
		return 0, nil, err
	}

	typ := header[0]
	length := int(binary.BigEndian.Uint32(header[1:5])) - 4
	if length < 0 {
		return 0, nil, fmt.Errorf("invalid message length: %d", length)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(c.r, payload); err != nil {
		return 0, nil, err
	}

	return typ, payload, nil
}

func (c *Client) applyDeadline(ctx context.Context) error {
	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			return err
		}
	} else {
		if err := c.conn.SetDeadline(time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) simpleQuery(ctx context.Context, query string) (*queryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.applyDeadline(ctx); err != nil {
		return nil, err
	}

	if err := c.writeQuery(query); err != nil {
		return nil, err
	}

	res := &queryResult{}

	for {
		typ, payload, err := c.readMessage(ctx)
		if err != nil {
			return nil, err
		}

		switch typ {
		case 'T':
			res.columnCount = parseRowDescription(payload)
		case 'D':
			row, err := parseDataRow(payload)
			if err != nil {
				return nil, err
			}
			res.rows = append(res.rows, row)
		case 'C':
			res.commandTag = parseCommandComplete(payload)
		case 'E':
			pgErr := parseErrorResponse(payload)
			if err := c.discardUntilReady(ctx); err != nil {
				return nil, err
			}
			return nil, pgErr
		case 'Z':
			return res, nil
		case 'N', 'S', '1', '2', '3', 'I':
			continue
		default:
			continue
		}
	}
}

func (c *Client) writeQuery(query string) error {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte('Q')
	body := append([]byte(query), 0)
	totalLen := int32(len(body) + 4)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(totalLen))
	buf.Write(tmp)
	buf.Write(body)

	if _, err := c.w.Write(buf.Bytes()); err != nil {
		return err
	}
	return c.w.Flush()
}

func (c *Client) discardUntilReady(ctx context.Context) error {
	for {
		typ, _, err := c.readMessage(ctx)
		if err != nil {
			return err
		}
		if typ == 'Z' {
			return nil
		}
	}
}

func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	prepared, err := prepareQuery(query, args...)
	if err != nil {
		return err
	}

	_, err = c.simpleQuery(ctx, prepared)
	return err
}

func (c *Client) QueryRow(ctx context.Context, query string, args ...any) row {
	prepared, err := prepareQuery(query, args...)
	if err != nil {
		return simpleRow{err: err}
	}

	res, err := c.simpleQuery(ctx, prepared)
	if err != nil {
		return simpleRow{err: err}
	}
	if len(res.rows) == 0 {
		return simpleRow{err: sql.ErrNoRows}
	}
	return simpleRow{values: res.rows[0]}
}

func (c *Client) Query(ctx context.Context, query string, args ...any) (rows, error) {
	prepared, err := prepareQuery(query, args...)
	if err != nil {
		return nil, err
	}

	res, err := c.simpleQuery(ctx, prepared)
	if err != nil {
		return nil, err
	}

	return &simpleRows{rows: res.rows}, nil
}

type simpleRow struct {
	values []string
	err    error
}

func (r simpleRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignValues(r.values, dest...)
}

type simpleRows struct {
	rows [][]string
	idx  int
	err  error
}

func (r *simpleRows) Close() {}

func (r *simpleRows) Err() error {
	return r.err
}

func (r *simpleRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *simpleRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("scan called out of sequence")
	}
	if err := assignValues(r.rows[r.idx-1], dest...); err != nil {
		r.err = err
		return err
	}
	return nil
}

func prepareQuery(query string, args ...any) (string, error) {
	if len(args) == 0 {
		return query, nil
	}

	var b strings.Builder
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch != '$' {
			b.WriteByte(ch)
			continue
		}

		j := i + 1
		for j < len(query) && query[j] >= '0' && query[j] <= '9' {
			j++
		}
		if j == i+1 {
			b.WriteByte(ch)
			continue
		}

		idx, err := strconv.Atoi(query[i+1 : j])
		if err != nil {
			return "", fmt.Errorf("invalid placeholder %q: %w", query[i:j], err)
		}
		if idx <= 0 || idx > len(args) {
			return "", fmt.Errorf("missing parameter for $%d", idx)
		}

		encoded, err := encodeParam(args[idx-1])
		if err != nil {
			return "", err
		}
		b.WriteString(encoded)
		i = j - 1
	}

	return b.String(), nil
}

func encodeParam(arg any) (string, error) {
	switch v := arg.(type) {
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'", nil
	case []byte:
		return "'" + strings.ReplaceAll(string(v), "'", "''") + "'", nil
	case bool:
		if v {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int:
		return strconv.Itoa(v), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	default:
		return "", fmt.Errorf("unsupported parameter type %T", arg)
	}
}

func assignValues(values []string, dest ...any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("column count mismatch: have %d values, want %d", len(values), len(dest))
	}

	for i, d := range dest {
		switch ptr := d.(type) {
		case *string:
			*ptr = values[i]
		case *bool:
			*ptr = parseBoolLiteral(values[i])
		case *int32:
			n, err := strconv.Atoi(values[i])
			if err != nil {
				return fmt.Errorf("invalid integer value: %w", err)
			}
			*ptr = int32(n)
		case *int:
			n, err := strconv.Atoi(values[i])
			if err != nil {
				return fmt.Errorf("invalid integer value: %w", err)
			}
			*ptr = n
		default:
			return fmt.Errorf("unsupported scan destination %T", d)
		}
	}

	return nil
}

func parseBoolLiteral(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "t", "true", "1", "y", "yes":
		return true
	default:
		return false
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Close()
}

type queryResult struct {
	columnCount int
	rows        [][]string
	commandTag  string
}

func parseRowDescription(payload []byte) int {
	if len(payload) < 2 {
		return 0
	}
	count := int(binary.BigEndian.Uint16(payload[:2]))
	return count
}

func parseDataRow(payload []byte) ([]string, error) {
	if len(payload) < 2 {
		return nil, errors.New("invalid data row")
	}
	fields := int(binary.BigEndian.Uint16(payload[:2]))
	values := make([]string, 0, fields)
	idx := 2
	for i := 0; i < fields; i++ {
		if idx+4 > len(payload) {
			return nil, errors.New("malformed data row")
		}
		lengthVal := int32(binary.BigEndian.Uint32(payload[idx : idx+4]))
		idx += 4
		if lengthVal == -1 {
			values = append(values, "")
			continue
		}
		length := int(lengthVal)
		if length < 0 || idx+length > len(payload) {
			return nil, errors.New("malformed data row length")
		}
		values = append(values, string(payload[idx:idx+length]))
		idx += length
	}
	return values, nil
}

func parseCommandComplete(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	return strings.TrimRight(string(payload), "\x00")
}

type Error struct {
	Severity string
	Code     string
	Message  string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("postgres error: %s", e.Code)
}

func parseErrorResponse(payload []byte) error {
	err := &Error{}
	idx := 0
	for idx < len(payload) {
		fieldType := payload[idx]
		idx++
		if fieldType == 0 {
			break
		}
		end := bytes.IndexByte(payload[idx:], 0)
		if end == -1 {
			end = len(payload) - idx
		}
		value := string(payload[idx : idx+end])
		idx += end + 1

		switch fieldType {
		case 'S':
			err.Severity = value
		case 'C':
			err.Code = value
		case 'M':
			err.Message = value
		}
	}
	return err
}
