package main

import (
	"context"
	"testing"
	"time"

	postgres "streamlation/packages/backend/postgres"
	queuepkg "streamlation/packages/backend/queue"
	sessionpkg "streamlation/packages/backend/session"
)

func TestGetDatabaseURLDefault(t *testing.T) {
	t.Setenv("WORKER_DATABASE_URL", "")
	if got := getDatabaseURL(); got != defaultDatabaseURL {
		t.Fatalf("expected default database URL, got %s", got)
	}
}

func TestGetRedisAddrDefault(t *testing.T) {
	t.Setenv("WORKER_REDIS_ADDR", "")
	if got := getRedisAddr(); got != defaultRedisAddr {
		t.Fatalf("expected default redis addr, got %s", got)
	}
}

func TestIngestionProcessorProcessesJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan string, 1)
	store := &stubSessionStore{
		getFunc: func(context.Context, string) (sessionpkg.TranslationSession, error) {
			result <- "job-1"
			return sessionpkg.TranslationSession{
				ID:             "job-1",
				Source:         sessionpkg.TranslationSource{Type: "hls", URI: "https://example.com/stream.m3u8"},
				TargetLanguage: "es",
			}, nil
		},
	}

	consumer := &stubConsumer{
		jobs: []*queuepkg.IngestionJob{{SessionID: "job-1"}},
	}

	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	processor := &ingestionProcessor{store: store, consumer: consumer, logger: logger}

	done := make(chan struct{})
	go func() {
		processor.Run(ctx)
		close(done)
	}()

	select {
	case id := <-result:
		if id != "job-1" {
			t.Fatalf("unexpected session id: %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job processing")
	}

	cancel()
	<-done
}

func TestIngestionProcessorHandlesMissingSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionRequested := make(chan struct{}, 1)

	store := &stubSessionStore{
		getFunc: func(context.Context, string) (sessionpkg.TranslationSession, error) {
			sessionRequested <- struct{}{}
			return sessionpkg.TranslationSession{}, postgres.ErrSessionNotFound
		},
	}
	consumer := &stubConsumer{jobs: []*queuepkg.IngestionJob{{SessionID: "missing"}}}

	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	processor := &ingestionProcessor{store: store, consumer: consumer, logger: logger}

	done := make(chan struct{})
	go func() {
		processor.Run(ctx)
		close(done)
	}()

	select {
	case <-sessionRequested:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session lookup")
	}

	cancel()
	<-done

	if remaining := len(consumer.jobs); remaining != 0 {
		t.Fatalf("expected all jobs to be consumed, %d remaining", remaining)
	}
}

type stubSessionStore struct {
	getFunc func(context.Context, string) (sessionpkg.TranslationSession, error)
}

func (s *stubSessionStore) Get(ctx context.Context, id string) (sessionpkg.TranslationSession, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return sessionpkg.TranslationSession{}, nil
}

type stubConsumer struct {
	jobs []*queuepkg.IngestionJob
}

func (s *stubConsumer) Pop(ctx context.Context, timeout time.Duration) (*queuepkg.IngestionJob, error) {
	if len(s.jobs) == 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(timeout):
			return nil, nil
		}
	}
	job := s.jobs[0]
	s.jobs = s.jobs[1:]
	return job, nil
}
