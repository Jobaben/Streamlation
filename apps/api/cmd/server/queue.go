package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const ingestionQueueName = "streamlation:ingestion:sessions"

// RedisIngestionEnqueuer pushes session identifiers onto a Redis list for downstream processing.
type RedisIngestionEnqueuer struct {
	addr string
}

// NewRedisIngestionEnqueuer constructs an ingestion enqueuer targeting the provided Redis address.
func NewRedisIngestionEnqueuer(addr string) *RedisIngestionEnqueuer {
	return &RedisIngestionEnqueuer{addr: addr}
}

// EnqueueIngestion appends a job payload to the ingestion queue.
func (e *RedisIngestionEnqueuer) EnqueueIngestion(ctx context.Context, sessionID string) error {
	payload, err := json.Marshal(map[string]string{"session_id": sessionID})
	if err != nil {
		return fmt.Errorf("marshal ingestion payload: %w", err)
	}
	return pushToRedis(ctx, e.addr, ingestionQueueName, string(payload))
}

func pushToRedis(ctx context.Context, addr, queue, payload string) error {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("redis dial: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

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
		return fmt.Errorf("redis error: %s", msg)
	default:
		msg, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("redis unexpected reply: %w", err)
		}
		return fmt.Errorf("unexpected redis reply: %c%s", prefix, msg)
	}
}
