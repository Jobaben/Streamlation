package queue

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const IngestionQueueName = "streamlation:ingestion:sessions"

type RedisIngestionEnqueuer struct {
	addr string
}

func NewRedisIngestionEnqueuer(addr string) *RedisIngestionEnqueuer {
	return &RedisIngestionEnqueuer{addr: addr}
}

func (e *RedisIngestionEnqueuer) EnqueueIngestion(ctx context.Context, sessionID string) error {
	payload, err := json.Marshal(map[string]string{"session_id": sessionID})
	if err != nil {
		return fmt.Errorf("marshal ingestion payload: %w", err)
	}
	return pushToRedis(ctx, e.addr, IngestionQueueName, string(payload))
}

type IngestionJob struct {
	SessionID string `json:"session_id"`
}

type RedisIngestionConsumer struct {
	addr string
}

func NewRedisIngestionConsumer(addr string) *RedisIngestionConsumer {
	return &RedisIngestionConsumer{addr: addr}
}

func (c *RedisIngestionConsumer) Pop(ctx context.Context, timeout time.Duration) (*IngestionJob, error) {
	payload, err := brpop(ctx, c.addr, IngestionQueueName, timeout)
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, nil
	}

	var job IngestionJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return nil, fmt.Errorf("decode ingestion payload: %w", err)
	}
	if job.SessionID == "" {
		return nil, fmt.Errorf("ingestion payload missing session_id")
	}
	return &job, nil
}

func pushToRedis(ctx context.Context, addr, queue, payload string) error {
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

	command := buildRESPCommand([]string{"LPUSH", queue, payload})

	if _, err := conn.Write(command); err != nil {
		return fmt.Errorf("redis write: %w", err)
	}

	reader := bufio.NewReader(conn)
	if err := readSimpleIntegerReply(reader); err != nil {
		return err
	}
	return nil
}

func brpop(ctx context.Context, addr, queue string, timeout time.Duration) (string, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("redis dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(timeout + 2*time.Second)
	}
	_ = conn.SetDeadline(deadline)

	seconds := int(timeout.Seconds())
	if timeout <= 0 {
		seconds = 0
	}
	command := buildRESPCommand([]string{"BRPOP", queue, strconv.Itoa(seconds)})

	if _, err := conn.Write(command); err != nil {
		return "", fmt.Errorf("redis write: %w", err)
	}

	reader := bufio.NewReader(conn)
	value, err := readBRPOPReply(reader)
	if err != nil {
		return "", err
	}
	return value, nil
}

func buildRESPCommand(args []string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&buf, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return buf.Bytes()
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
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("redis unexpected reply: %w", err)
		}
		return fmt.Errorf("unexpected redis reply: %c%s", prefix, msg)
	}
}

func readBRPOPReply(r *bufio.Reader) (string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return "", fmt.Errorf("redis read: %w", err)
	}

	switch prefix {
	case '*':
		line, err := readLine(r)
		if err != nil {
			return "", err
		}
		count, err := strconv.Atoi(line)
		if err != nil {
			return "", fmt.Errorf("redis invalid multibulk length: %w", err)
		}
		if count == -1 {
			return "", nil
		}
		if count != 2 {
			return "", fmt.Errorf("unexpected multibulk length: %d", count)
		}

		if _, err := readBulkString(r); err != nil {
			return "", err
		}
		value, err := readBulkString(r)
		if err != nil {
			return "", err
		}
		return value, nil
	case '-':
		msg, err := r.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("redis error read: %w", err)
		}
		return "", fmt.Errorf("redis error: %s", strings.TrimSpace(msg))
	default:
		msg, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("redis unexpected reply: %w", err)
		}
		return "", fmt.Errorf("unexpected redis reply: %c%s", prefix, msg)
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
	if !strings.HasPrefix(line, "$") {
		return "", fmt.Errorf("redis bulk length prefix: %s", line)
	}
	length, err := strconv.Atoi(line[1:])
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
