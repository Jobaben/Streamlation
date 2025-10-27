package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	queuepkg "streamlation/packages/backend/queue"
	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"

	"go.uber.org/zap"
)

type stubSessionStore struct {
	session sessionpkg.TranslationSession
	err     error
}

func (s *stubSessionStore) Get(context.Context, string) (sessionpkg.TranslationSession, error) {
	return s.session, s.err
}

type stubIngestor struct {
	err error
	got sessionpkg.TranslationSession
}

func (s *stubIngestor) Ingest(ctx context.Context, session sessionpkg.TranslationSession) error {
	s.got = session
	return s.err
}

type capturingPublisher struct {
	mu     sync.Mutex
	events []statuspkg.SessionStatusEvent
}

func (c *capturingPublisher) Publish(_ context.Context, event statuspkg.SessionStatusEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
	return nil
}

func (c *capturingPublisher) Events() []statuspkg.SessionStatusEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]statuspkg.SessionStatusEvent(nil), c.events...)
}

func TestHandleJobPublishesCompletedEvent(t *testing.T) {
	publisher := &capturingPublisher{}
	ingestor := &stubIngestor{}
	store := &stubSessionStore{session: sessionpkg.TranslationSession{ID: "abc"}}
	worker := &IngestionWorker{
		sessions:  store,
		publisher: publisher,
		ingestor:  ingestor,
		logger:    newTestLogger(t),
	}

	worker.handleJob(context.Background(), &queuepkg.IngestionJob{SessionID: "abc"})

	if ingestor.got.ID != "abc" {
		t.Fatalf("expected ingestor to receive session, got %+v", ingestor.got)
	}

	events := publisher.Events()
	if len(events) != 2 {
		t.Fatalf("expected two status events, got %d", len(events))
	}
	if events[0].State != "started" {
		t.Fatalf("expected first event state 'started', got %s", events[0].State)
	}
	if events[1].State != "completed" {
		t.Fatalf("expected second event state 'completed', got %s", events[1].State)
	}
}

func TestHandleJobWhenIngestFails(t *testing.T) {
	publisher := &capturingPublisher{}
	ingestor := &stubIngestor{err: errors.New("ingest failed")}
	store := &stubSessionStore{session: sessionpkg.TranslationSession{ID: "abc"}}
	worker := &IngestionWorker{
		sessions:  store,
		publisher: publisher,
		ingestor:  ingestor,
		logger:    newTestLogger(t),
	}

	worker.handleJob(context.Background(), &queuepkg.IngestionJob{SessionID: "abc"})

	events := publisher.Events()
	if len(events) != 2 {
		t.Fatalf("expected two status events, got %d", len(events))
	}
	last := events[len(events)-1]
	if last.State != "error" {
		t.Fatalf("expected error event, got %s", last.State)
	}
}

func TestHandleJobWhenSessionMissing(t *testing.T) {
	publisher := &capturingPublisher{}
	store := &stubSessionStore{err: errors.New("not found")}
	worker := &IngestionWorker{
		sessions:  store,
		publisher: publisher,
		ingestor:  &stubIngestor{},
		logger:    newTestLogger(t),
	}

	worker.handleJob(context.Background(), &queuepkg.IngestionJob{SessionID: "missing"})

	events := publisher.Events()
	if len(events) != 1 {
		t.Fatalf("expected one status event, got %d", len(events))
	}
	if events[0].State != "error" {
		t.Fatalf("expected error event, got %s", events[0].State)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := &stubQueue{jobs: []*queuepkg.IngestionJob{{SessionID: "abc"}}}
	store := &stubSessionStore{session: sessionpkg.TranslationSession{ID: "abc"}}
	publisher := &capturingPublisher{}
	ingestor := &stubIngestor{}
	worker := NewIngestionWorker(queue, store, publisher, ingestor, newTestLogger(t), 5*time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker.Run did not return after cancel")
	}
}

type stubQueue struct {
	jobs []*queuepkg.IngestionJob
	err  error
}

func (s *stubQueue) Pop(ctx context.Context, timeout time.Duration) (*queuepkg.IngestionJob, error) {
	if s.err != nil {
		return nil, s.err
	}
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

func newTestLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	cfg := zap.NewProductionConfig()
	logger, err := cfg.Build()
	if err != nil {
		t.Fatalf("failed to build logger: %v", err)
	}
	t.Cleanup(func() {
		_ = logger.Sync()
	})
	return logger.Sugar()
}
