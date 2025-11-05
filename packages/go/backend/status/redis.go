package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	redisclient "streamlation/packages/backend/redis"
)

type RedisStatusPublisher struct {
	client *redisclient.Client
}

func NewRedisStatusPublisher(addr string) (*RedisStatusPublisher, error) {
	client, err := redisclient.NewClient(addr)
	if err != nil {
		return nil, err
	}
	return &RedisStatusPublisher{client: client}, nil
}

func (p *RedisStatusPublisher) Publish(ctx context.Context, event SessionStatusEvent) error {
	if event.SessionID == "" {
		return fmt.Errorf("session id required")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal status event: %w", err)
	}
	if _, err := p.client.Do(ctx, "PUBLISH", channelName(event.SessionID), string(payload)); err != nil {
		return fmt.Errorf("publish status event: %w", err)
	}
	return nil
}

func (p *RedisStatusPublisher) Close() error {
	return p.client.Close()
}

type RedisStatusSubscriber struct {
	client *redisclient.Client
}

func NewRedisStatusSubscriber(addr string) (*RedisStatusSubscriber, error) {
	client, err := redisclient.NewClient(addr)
	if err != nil {
		return nil, err
	}
	return &RedisStatusSubscriber{client: client}, nil
}

func (s *RedisStatusSubscriber) Subscribe(ctx context.Context, sessionID string) (StatusStream, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session id required")
	}
	pubsub, err := s.client.Subscribe(ctx, channelName(sessionID))
	if err != nil {
		return nil, err
	}

	stream := &redisStatusStream{
		pubsub:    pubsub,
		sessionID: sessionID,
		events:    make(chan SessionStatusEvent, 8),
		errors:    make(chan error, 1),
		done:      make(chan struct{}),
	}
	go stream.run()
	return stream, nil
}

func (s *RedisStatusSubscriber) Close() error {
	return s.client.Close()
}

type StatusStream interface {
	Events() <-chan SessionStatusEvent
	Errors() <-chan error
	Close() error
}

type redisStatusStream struct {
	pubsub    *redisclient.PubSub
	sessionID string
	events    chan SessionStatusEvent
	errors    chan error
	done      chan struct{}
	closeOnce sync.Once
}

func (s *redisStatusStream) Events() <-chan SessionStatusEvent {
	return s.events
}

func (s *redisStatusStream) Errors() <-chan error {
	return s.errors
}

func (s *redisStatusStream) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		closeErr = s.pubsub.Close()
		<-s.done
	})
	return closeErr
}

func (s *redisStatusStream) run() {
	defer close(s.done)
	defer close(s.events)
	defer close(s.errors)

	for {
		select {
		case msg, ok := <-s.pubsub.Messages():
			if !ok {
				return
			}
			if msg.Kind != "message" && msg.Kind != "pmessage" {
				continue
			}
			var event SessionStatusEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				s.reportError(fmt.Errorf("decode status event: %w", err))
				continue
			}
			if event.SessionID == "" {
				event.SessionID = s.sessionID
			}
			s.events <- event
		case err, ok := <-s.pubsub.Errors():
			if !ok {
				return
			}
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				return
			}
			s.reportError(err)
		}
	}
}

func (s *redisStatusStream) reportError(err error) {
	select {
	case s.errors <- err:
	default:
	}
}
