package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	redisclient "streamlation/packages/backend/redis"
)

const IngestionQueueName = "streamlation:ingestion:sessions"

type RedisIngestionEnqueuer struct {
	client *redisclient.Client
}

func NewRedisIngestionEnqueuer(addr string) (*RedisIngestionEnqueuer, error) {
	client, err := redisclient.NewClient(addr)
	if err != nil {
		return nil, err
	}
	return &RedisIngestionEnqueuer{client: client}, nil
}

func (e *RedisIngestionEnqueuer) EnqueueIngestion(ctx context.Context, sessionID string) error {
	payload, err := json.Marshal(map[string]string{"session_id": sessionID})
	if err != nil {
		return fmt.Errorf("marshal ingestion payload: %w", err)
	}
	if _, err := e.client.Do(ctx, "LPUSH", IngestionQueueName, string(payload)); err != nil {
		return fmt.Errorf("enqueue ingestion: %w", err)
	}
	return nil
}

func (e *RedisIngestionEnqueuer) Close() error {
	return e.client.Close()
}

type IngestionJob struct {
	SessionID string `json:"session_id"`
}

type RedisIngestionConsumer struct {
	client *redisclient.Client
}

func NewRedisIngestionConsumer(addr string) (*RedisIngestionConsumer, error) {
	client, err := redisclient.NewClient(addr)
	if err != nil {
		return nil, err
	}
	return &RedisIngestionConsumer{client: client}, nil
}

func (c *RedisIngestionConsumer) Pop(ctx context.Context, timeout time.Duration) (*IngestionJob, error) {
	ctxWithDeadline, cancel := ensureTimeout(ctx, timeout)
	defer cancel()

	seconds := int(timeout.Seconds())
	if timeout > 0 && seconds == 0 {
		seconds = 1
	}
	if timeout <= 0 {
		seconds = 0
	}

	reply, err := c.client.Do(ctxWithDeadline, "BRPOP", IngestionQueueName, strconv.Itoa(seconds))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, nil
		}
		return nil, fmt.Errorf("dequeue ingestion: %w", err)
	}

	if reply.IsNil {
		return nil, nil
	}
	if reply.Type != '*' || len(reply.Array) != 2 {
		return nil, fmt.Errorf("unexpected BRPOP reply: %#v", reply)
	}

	payload := reply.Array[1]
	if payload.IsNil {
		return nil, nil
	}

	var job IngestionJob
	if err := json.Unmarshal([]byte(payload.Text), &job); err != nil {
		return nil, fmt.Errorf("decode ingestion payload: %w", err)
	}
	if job.SessionID == "" {
		return nil, fmt.Errorf("ingestion payload missing session_id")
	}
	return &job, nil
}

func (c *RedisIngestionConsumer) Close() error {
	return c.client.Close()
}

func ensureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}

	extra := timeout + defaultTimeout
	if timeout <= 0 {
		extra = defaultTimeout
	}
	return context.WithTimeout(ctx, extra)
}

const defaultTimeout = 5 * time.Second
