package main

import (
	"context"
	"testing"
	"time"

	postgres "streamlation/packages/backend/postgres"
	queuepkg "streamlation/packages/backend/queue"
	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"
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

	var events []statuspkg.SessionStatusEvent
	publisher := &stubStatusPublisher{publishFunc: func(_ context.Context, event statuspkg.SessionStatusEvent) error {
		events = append(events, event)
		return nil
	}}

	pipeline := &stubPipeline{runFunc: func(ctx context.Context, session sessionpkg.TranslationSession, emit func(statuspkg.SessionStatusEvent) error) error {
		if session.ID != "job-1" {
			t.Fatalf("unexpected session passed to pipeline: %s", session.ID)
		}
		if err := emit(statuspkg.SessionStatusEvent{SessionID: session.ID, Stage: "media", State: "normalizing"}); err != nil {
			return err
		}
		return emit(statuspkg.SessionStatusEvent{SessionID: session.ID, Stage: "translation", State: "generating"})
	}}

	processor := &ingestionProcessor{store: store, consumer: consumer, publisher: publisher, pipeline: pipeline, logger: logger}

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

	if len(events) != 4 {
		t.Fatalf("expected four status events, got %d", len(events))
	}
	if events[0].State != "dequeued" || events[0].Stage != "ingestion" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].State != "ready" || events[1].Stage != "ingestion" {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
	if events[2].Stage != "media" || events[2].State != "normalizing" {
		t.Fatalf("unexpected third event: %#v", events[2])
	}
	if events[3].Stage != "translation" || events[3].State != "generating" {
		t.Fatalf("unexpected fourth event: %#v", events[3])
	}
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

	var events []statuspkg.SessionStatusEvent
	publisher := &stubStatusPublisher{publishFunc: func(_ context.Context, event statuspkg.SessionStatusEvent) error {
		events = append(events, event)
		return nil
	}}

	pipeline := &stubPipeline{runFunc: func(context.Context, sessionpkg.TranslationSession, func(statuspkg.SessionStatusEvent) error) error {
		return nil
	}}

	processor := &ingestionProcessor{store: store, consumer: consumer, publisher: publisher, pipeline: pipeline, logger: logger}

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

	if len(events) != 2 {
		t.Fatalf("expected two status events, got %d", len(events))
	}
	if events[0].State != "dequeued" {
		t.Fatalf("expected dequeued event first, got %#v", events[0])
	}
	if events[1].State != "not_found" {
		t.Fatalf("expected not_found event, got %#v", events[1])
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

type stubStatusPublisher struct {
	publishFunc func(context.Context, statuspkg.SessionStatusEvent) error
}

func (s *stubStatusPublisher) Publish(ctx context.Context, event statuspkg.SessionStatusEvent) error {
	if s.publishFunc != nil {
		return s.publishFunc(ctx, event)
	}
	return nil
}

type stubPipeline struct {
	runFunc func(context.Context, sessionpkg.TranslationSession, func(statuspkg.SessionStatusEvent) error) error
}

func (s *stubPipeline) Run(ctx context.Context, session sessionpkg.TranslationSession, emit func(statuspkg.SessionStatusEvent) error) error {
	if s.runFunc != nil {
		return s.runFunc(ctx, session, emit)
	}
	return nil
}
