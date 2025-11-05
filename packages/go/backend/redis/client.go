package redis

import (
	"bufio"
	"context"
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

const defaultTimeout = 5 * time.Second

type Client struct {
	addr   string
	dialer net.Dialer

	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

type Reply struct {
	Type  byte
	Text  string
	Array []Reply
	IsNil bool
}

func NewClient(addr string) (*Client, error) {
	resolved, err := resolveAddr(addr)
	if err != nil {
		return nil, err
	}
	return &Client{addr: resolved}, nil
}

func (c *Client) Do(ctx context.Context, args ...string) (Reply, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConn(ctx); err != nil {
		return Reply{}, err
	}

	deadline := deadlineFromContext(ctx)
	if err := c.conn.SetDeadline(deadline); err != nil {
		c.reset()
		return Reply{}, err
	}

	if err := writeCommand(c.writer, args); err != nil {
		c.reset()
		return Reply{}, err
	}
	if err := c.writer.Flush(); err != nil {
		c.reset()
		return Reply{}, err
	}

	reply, err := readReply(c.reader)
	if err != nil {
		if shouldReset(err) {
			c.reset()
		}
		return Reply{}, err
	}
	if reply.Type == '-' {
		return Reply{}, fmt.Errorf("redis error: %s", reply.Text)
	}

	_ = c.conn.SetDeadline(time.Time{})
	return reply, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reset()
}

func (c *Client) Subscribe(ctx context.Context, channel string) (*PubSub, error) {
	resolved, err := resolveAddr(c.addr)
	if err != nil {
		return nil, err
	}

	conn, err := c.dialer.DialContext(ctx, "tcp", resolved)
	if err != nil {
		return nil, fmt.Errorf("redis dial: %w", err)
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	deadline := deadlineFromContext(ctx)
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := writeCommand(writer, []string{"SUBSCRIBE", channel}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	reply, err := readReply(reader)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if reply.Type == '-' {
		_ = conn.Close()
		return nil, fmt.Errorf("redis error: %s", reply.Text)
	}
	if reply.Type != '*' || len(reply.Array) < 3 || !strings.EqualFold(reply.Array[0].Text, "subscribe") {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected subscribe reply: %#v", reply)
	}

	_ = conn.SetDeadline(time.Time{})

	streamCtx, cancel := context.WithCancel(ctx)
	ps := &PubSub{
		conn:     conn,
		reader:   reader,
		writer:   writer,
		channel:  channel,
		messages: make(chan Message, 8),
		errors:   make(chan error, 1),
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	go ps.run(streamCtx)
	return ps, nil
}

func (c *Client) ensureConn(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}

	resolved, err := resolveAddr(c.addr)
	if err != nil {
		return err
	}

	conn, err := c.dialer.DialContext(ctx, "tcp", resolved)
	if err != nil {
		return fmt.Errorf("redis dial: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)
	return nil
}

func (c *Client) reset() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.reader = nil
		c.writer = nil
		return err
	}
	return nil
}

type Message struct {
	Kind    string
	Channel string
	Payload string
}

type PubSub struct {
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	channel   string
	messages  chan Message
	errors    chan error
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}

func (ps *PubSub) Messages() <-chan Message {
	return ps.messages
}

func (ps *PubSub) Errors() <-chan error {
	return ps.errors
}

func (ps *PubSub) Close() error {
	var closeErr error
	ps.closeOnce.Do(func() {
		ps.cancel()
		closeErr = ps.conn.Close()
		<-ps.done
	})
	return closeErr
}

func (ps *PubSub) run(ctx context.Context) {
	defer close(ps.done)
	defer close(ps.messages)
	defer close(ps.errors)

	for {
		if ctx.Err() != nil {
			return
		}
		if err := ps.conn.SetReadDeadline(time.Now().Add(defaultTimeout)); err != nil {
			ps.reportError(err)
			return
		}

		reply, err := readReply(ps.reader)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			ps.reportError(err)
			return
		}

		if reply.Type != '*' || len(reply.Array) < 3 {
			continue
		}
		kind := strings.ToLower(reply.Array[0].Text)
		channel := reply.Array[1].Text

		switch kind {
		case "message", "pmessage":
			payload := reply.Array[2].Text
			msg := Message{Kind: kind, Channel: channel, Payload: payload}
			select {
			case ps.messages <- msg:
			case <-ctx.Done():
				return
			}
		case "subscribe", "psubscribe", "unsubscribe", "punsubscribe":
			continue
		default:
			continue
		}
	}
}

func (ps *PubSub) reportError(err error) {
	select {
	case ps.errors <- err:
	default:
	}
}

func deadlineFromContext(ctx context.Context) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline
	}
	return time.Now().Add(defaultTimeout)
}

func resolveAddr(addr string) (string, error) {
	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		u, err := url.Parse(addr)
		if err != nil {
			return "", fmt.Errorf("invalid redis url: %w", err)
		}
		if u.Host == "" {
			return "", fmt.Errorf("redis url missing host")
		}
		return u.Host, nil
	}
	return addr, nil
}

func writeCommand(w *bufio.Writer, args []string) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return fmt.Errorf("redis write: %w", err)
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return fmt.Errorf("redis write: %w", err)
		}
	}
	return nil
}

func readReply(r *bufio.Reader) (Reply, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		if err == io.EOF {
			return Reply{}, io.EOF
		}
		return Reply{}, fmt.Errorf("redis read: %w", err)
	}

	switch prefix {
	case '+', '-', ':':
		line, err := readLine(r)
		if err != nil {
			return Reply{}, err
		}
		return Reply{Type: prefix, Text: line}, nil
	case '$':
		line, err := readLine(r)
		if err != nil {
			return Reply{}, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return Reply{}, fmt.Errorf("redis bulk length: %w", err)
		}
		if length == -1 {
			return Reply{Type: '$', IsNil: true}, nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return Reply{}, fmt.Errorf("redis bulk read: %w", err)
		}
		return Reply{Type: '$', Text: string(buf[:length])}, nil
	case '*':
		line, err := readLine(r)
		if err != nil {
			return Reply{}, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return Reply{}, fmt.Errorf("redis array length: %w", err)
		}
		if length == -1 {
			return Reply{Type: '*', IsNil: true}, nil
		}
		values := make([]Reply, 0, length)
		for i := 0; i < length; i++ {
			value, err := readReply(r)
			if err != nil {
				return Reply{}, err
			}
			values = append(values, value)
		}
		return Reply{Type: '*', Array: values}, nil
	default:
		return Reply{}, fmt.Errorf("unexpected redis reply type: %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("redis read line: %w", err)
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

func shouldReset(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return false
}
