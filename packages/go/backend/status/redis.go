package status

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RedisStatusPublisher struct {
	addr string
}

func NewRedisStatusPublisher(addr string) *RedisStatusPublisher {
	return &RedisStatusPublisher{addr: addr}
}

func (p *RedisStatusPublisher) Publish(ctx context.Context, event SessionStatusEvent) error {
	if event.SessionID == "" {
		return fmt.Errorf("session id required")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal status event: %w", err)
	}
	return publishToRedis(ctx, p.addr, channelName(event.SessionID), string(payload))
}

type RedisStatusSubscriber struct {
	addr string
}

func NewRedisStatusSubscriber(addr string) *RedisStatusSubscriber {
	return &RedisStatusSubscriber{addr: addr}
}

func (s *RedisStatusSubscriber) Subscribe(ctx context.Context, sessionID string) (StatusStream, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("redis dial: %w", err)
	}

	stream := &redisStatusStream{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		events:    make(chan SessionStatusEvent, 8),
		errors:    make(chan error, 1),
		sessionID: sessionID,
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	command := buildRESPCommand([]string{"SUBSCRIBE", channelName(sessionID)})
	if _, err := conn.Write(command); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("redis write subscribe: %w", err)
	}

	// Remove deadlines so we can block on reads while still honoring context cancellation via conn.Close().
	_ = conn.SetDeadline(time.Time{})

	go stream.run(ctx)
	return stream, nil
}

type StatusStream interface {
	Events() <-chan SessionStatusEvent
	Errors() <-chan error
	Close() error
}

type redisStatusStream struct {
	conn      net.Conn
	reader    *bufio.Reader
	events    chan SessionStatusEvent
	errors    chan error
	closeOnce sync.Once
	sessionID string
}

func (s *redisStatusStream) Events() <-chan SessionStatusEvent {
	return s.events
}

func (s *redisStatusStream) Errors() <-chan error {
	return s.errors
}

func (s *redisStatusStream) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.conn.Close()
		close(s.events)
		close(s.errors)
	})
	return err
}

func (s *redisStatusStream) run(ctx context.Context) {
	defer func() { _ = s.Close() }()

	for {
		if ctx.Err() != nil {
			return
		}

		if err := s.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			s.reportError(fmt.Errorf("redis set read deadline: %w", err))
			return
		}

		msg, err := readPubSubMessage(s.reader)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			s.reportError(err)
			return
		}

		switch msg.kind {
		case "subscribe", "psubscribe":
			continue
		case "message":
			var event SessionStatusEvent
			if err := json.Unmarshal([]byte(msg.payload), &event); err != nil {
				s.reportError(fmt.Errorf("decode status event: %w", err))
				continue
			}
			if event.SessionID == "" {
				event.SessionID = s.sessionID
			}
			select {
			case s.events <- event:
			case <-ctx.Done():
				return
			}
		default:
			continue
		}
	}
}

func (s *redisStatusStream) reportError(err error) {
	select {
	case s.errors <- err:
	default:
	}
}

func publishToRedis(ctx context.Context, addr, channel, payload string) error {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("redis dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	command := buildRESPCommand([]string{"PUBLISH", channel, payload})
	if _, err := conn.Write(command); err != nil {
		return fmt.Errorf("redis write: %w", err)
	}

	reader := bufio.NewReader(conn)
	if err := readSimpleIntegerReply(reader); err != nil {
		return err
	}
	return nil
}

type respValue struct {
	kind byte
	text string
}

type pubSubMessage struct {
	kind    string
	channel string
	payload string
}

func readPubSubMessage(r *bufio.Reader) (pubSubMessage, error) {
	values, err := readArray(r)
	if err != nil {
		return pubSubMessage{}, err
	}
	if len(values) != 3 {
		return pubSubMessage{}, fmt.Errorf("unexpected pubsub message length: %d", len(values))
	}

	kind := values[0].text
	channel := values[1].text
	payload := values[2].text

	return pubSubMessage{kind: kind, channel: channel, payload: payload}, nil
}

func readArray(r *bufio.Reader) ([]respValue, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("redis read: %w", err)
	}
	if prefix != '*' {
		return nil, fmt.Errorf("expected array prefix, got %q", prefix)
	}

	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	length, err := strconv.Atoi(line)
	if err != nil {
		return nil, fmt.Errorf("redis array length: %w", err)
	}

	values := make([]respValue, 0, length)
	for i := 0; i < length; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("redis read: %w", err)
		}
		switch b {
		case '$':
			text, err := readBulkString(r)
			if err != nil {
				return nil, err
			}
			values = append(values, respValue{kind: '$', text: text})
		case ':':
			line, err := readLine(r)
			if err != nil {
				return nil, err
			}
			values = append(values, respValue{kind: ':', text: line})
		case '+':
			line, err := readLine(r)
			if err != nil {
				return nil, err
			}
			values = append(values, respValue{kind: '+', text: line})
		default:
			return nil, fmt.Errorf("unexpected RESP type: %q", b)
		}
	}
	return values, nil
}

func buildRESPCommand(args []string) []byte {
	var buf strings.Builder
	fmt.Fprintf(&buf, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&buf, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return []byte(buf.String())
}

func readSimpleIntegerReply(r *bufio.Reader) error {
	prefix, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("redis read: %w", err)
	}
	switch prefix {
	case ':':
		if _, err := r.ReadString('\n'); err != nil {
			return fmt.Errorf("redis read integer: %w", err)
		}
		return nil
	case '-':
		msg, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("redis error read: %w", err)
		}
		return fmt.Errorf("redis error: %s", strings.TrimSpace(msg))
	default:
		msg, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("redis unexpected reply: %w", err)
		}
		return fmt.Errorf("unexpected redis reply: %c%s", prefix, msg)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("redis read line: %w", err)
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

func readBulkString(r *bufio.Reader) (string, error) {
	line, err := readLine(r)
	if err != nil {
		return "", err
	}
	length, err := strconv.Atoi(line)
	if err != nil {
		return "", fmt.Errorf("redis bulk length: %w", err)
	}
	if length == -1 {
		return "", nil
	}
	buf := make([]byte, length+2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("redis bulk read: %w", err)
	}
	return string(buf[:length]), nil
}
