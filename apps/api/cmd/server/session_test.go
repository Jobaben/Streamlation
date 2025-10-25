package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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

	handler := createSessionHandler(store, enqueuer, logger)
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
}

func TestCreateSessionHandler_InvalidPayload(t *testing.T) {
	store := &stubSessionStore{}
	enqueuer := &stubEnqueuer{}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewBufferString("{"))
	rr := httptest.NewRecorder()

	handler := createSessionHandler(store, enqueuer, logger)
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

	handler := createSessionHandler(store, enqueuer, logger)
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

	handler := createSessionHandler(store, enqueuer, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	if deleted != "rollback42" {
		t.Fatalf("expected rollback for session rollback42, got %s", deleted)
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

type stubSessionStore struct {
	createFunc func(context.Context, TranslationSession) error
	getFunc    func(context.Context, string) (TranslationSession, error)
	deleteFunc func(context.Context, string) error
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

type stubEnqueuer struct {
	enqueueFunc func(context.Context, string) error
}

func (e *stubEnqueuer) EnqueueIngestion(ctx context.Context, sessionID string) error {
	if e.enqueueFunc != nil {
		return e.enqueueFunc(ctx, sessionID)
	}
	return nil
}
