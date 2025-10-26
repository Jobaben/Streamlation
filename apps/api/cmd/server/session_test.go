package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	statuspkg "streamlation/packages/backend/status"
)

func TestCreateSessionHandler_Success(t *testing.T) {
	var stored TranslationSession
	store := &stubSessionStore{
		createFunc: func(_ context.Context, session TranslationSession) error {
			stored = session
			return nil
		},
		getFunc: func(_ context.Context, id string) (TranslationSession, error) {
			return stored, nil
		},
		deleteFunc: func(context.Context, string) error { return nil },
	}
	var enqueued string
	enqueuer := &stubEnqueuer{enqueueFunc: func(_ context.Context, sessionID string) error {
		enqueued = sessionID
		return nil
	}}

	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	payload := map[string]any{
		"id":             "session123",
		"source":         map[string]any{"type": "hls", "uri": "https://example.com/stream.m3u8"},
		"targetLanguage": "es",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	var events []statuspkg.SessionStatusEvent
	publisher := &stubStatusPublisher{publishFunc: func(_ context.Context, event statuspkg.SessionStatusEvent) error {
		events = append(events, event)
		return nil
	}}

	handler := createSessionHandler(store, enqueuer, publisher, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var session TranslationSession
	if err := json.Unmarshal(rr.Body.Bytes(), &session); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if session.Options.ModelProfile != "cpu-basic" {
		t.Fatalf("expected default model profile, got %s", session.Options.ModelProfile)
	}

	if enqueued != "session123" {
		t.Fatalf("expected session to be enqueued, got %s", enqueued)
	}

	if len(events) != 2 {
		t.Fatalf("expected two status events, got %d", len(events))
	}
	if events[0].Stage != "session" || events[0].State != "registered" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].Stage != "ingestion" || events[1].State != "queued" {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
}

func TestCreateSessionHandler_InvalidPayload(t *testing.T) {
	store := &stubSessionStore{}
	enqueuer := &stubEnqueuer{}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewBufferString("{"))
	rr := httptest.NewRecorder()

	publisher := &stubStatusPublisher{}
	handler := createSessionHandler(store, enqueuer, publisher, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestCreateSessionHandler_Duplicate(t *testing.T) {
	store := &stubSessionStore{
		createFunc: func(context.Context, TranslationSession) error {
			return ErrSessionExists
		},
	}
	enqueuer := &stubEnqueuer{}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	payload := map[string]any{
		"id":             "duplicate12",
		"source":         map[string]any{"type": "rtmp", "uri": "rtmp://localhost/live"},
		"targetLanguage": "fr",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	publisher := &stubStatusPublisher{}
	handler := createSessionHandler(store, enqueuer, publisher, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rr.Code)
	}
}

func TestCreateSessionHandler_EnqueueFailureRollsBack(t *testing.T) {
	var deleted string
	store := &stubSessionStore{
		createFunc: func(context.Context, TranslationSession) error { return nil },
		deleteFunc: func(_ context.Context, id string) error {
			deleted = id
			return nil
		},
	}
	enqueuer := &stubEnqueuer{enqueueFunc: func(context.Context, string) error {
		return errors.New("enqueue failed")
	}}

	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	payload := map[string]any{
		"id":             "rollback42",
		"source":         map[string]any{"type": "hls", "uri": "https://example.com/stream.m3u8"},
		"targetLanguage": "it",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	var failureEvent statuspkg.SessionStatusEvent
	publisher := &stubStatusPublisher{publishFunc: func(_ context.Context, event statuspkg.SessionStatusEvent) error {
		failureEvent = event
		return nil
	}}

	handler := createSessionHandler(store, enqueuer, publisher, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	if deleted != "rollback42" {
		t.Fatalf("expected rollback for session rollback42, got %s", deleted)
	}

	if failureEvent.State != "error" || failureEvent.Stage != "ingestion" {
		t.Fatalf("expected failure status event, got %#v", failureEvent)
	}
}

func TestGetSessionHandler_NotFound(t *testing.T) {
	store := &stubSessionStore{
		getFunc: func(context.Context, string) (TranslationSession, error) {
			return TranslationSession{}, ErrSessionNotFound
		},
	}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodGet, "/sessions/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	handler := getSessionHandler(store, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestGetSessionHandler_Success(t *testing.T) {
	expected := TranslationSession{
		ID: "existing1",
		Source: TranslationSource{
			Type: "dash",
			URI:  "https://example.com/manifest.mpd",
		},
		TargetLanguage: "de",
		Options: TranslationOptions{
			EnableDubbing:      true,
			LatencyToleranceMs: 1500,
			ModelProfile:       "gpu-accelerated",
		},
	}
	store := &stubSessionStore{
		getFunc: func(context.Context, string) (TranslationSession, error) {
			return expected, nil
		},
	}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodGet, "/sessions/existing1", nil)
	req.SetPathValue("id", "existing1")
	rr := httptest.NewRecorder()

	handler := getSessionHandler(store, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var got TranslationSession
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got != expected {
		t.Fatalf("unexpected session: %#v", got)
	}
}

func TestListSessionsHandler_Success(t *testing.T) {
	expected := []TranslationSession{{
		ID:             "s1",
		Source:         TranslationSource{Type: "hls", URI: "https://example.com"},
		TargetLanguage: "es",
	}}

	store := &stubSessionStore{listFunc: func(context.Context, int) ([]TranslationSession, error) {
		return expected, nil
	}}

	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	rr := httptest.NewRecorder()

	handler := listSessionsHandler(store, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var sessions []TranslationSession
	if err := json.Unmarshal(rr.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(sessions) != 1 || sessions[0].ID != "s1" {
		t.Fatalf("unexpected sessions: %#v", sessions)
	}
}

func TestListSessionsHandler_InvalidLimit(t *testing.T) {
	store := &stubSessionStore{}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodGet, "/sessions?limit=abc", nil)
	rr := httptest.NewRecorder()

	handler := listSessionsHandler(store, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

type stubSessionStore struct {
	createFunc func(context.Context, TranslationSession) error
	getFunc    func(context.Context, string) (TranslationSession, error)
	deleteFunc func(context.Context, string) error
	listFunc   func(context.Context, int) ([]TranslationSession, error)
}

func (s *stubSessionStore) Create(ctx context.Context, session TranslationSession) error {
	if s.createFunc != nil {
		return s.createFunc(ctx, session)
	}
	return nil
}

func (s *stubSessionStore) Get(ctx context.Context, id string) (TranslationSession, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return TranslationSession{}, nil
}

func (s *stubSessionStore) Delete(ctx context.Context, id string) error {
	if s.deleteFunc != nil {
		return s.deleteFunc(ctx, id)
	}
	return nil
}

func (s *stubSessionStore) List(ctx context.Context, limit int) ([]TranslationSession, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx, limit)
	}
	return nil, nil
}

type stubEnqueuer struct {
	enqueueFunc func(context.Context, string) error
}

func (e *stubEnqueuer) EnqueueIngestion(ctx context.Context, sessionID string) error {
	if e.enqueueFunc != nil {
		return e.enqueueFunc(ctx, sessionID)
	}
	return nil
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
