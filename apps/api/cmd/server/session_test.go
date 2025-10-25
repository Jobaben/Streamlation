package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSessionHandler_Success(t *testing.T) {
	manager := newTranslationSessionManager()
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

	handler := createSessionHandler(manager, logger)
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
}

func TestCreateSessionHandler_InvalidPayload(t *testing.T) {
	manager := newTranslationSessionManager()
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewBufferString("{"))
	rr := httptest.NewRecorder()

	handler := createSessionHandler(manager, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestCreateSessionHandler_Duplicate(t *testing.T) {
	manager := newTranslationSessionManager()
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	session := TranslationSession{
		ID: "duplicate12",
		Source: TranslationSource{
			Type: "rtmp",
			URI:  "rtmp://localhost/live",
		},
		TargetLanguage: "fr",
		Options:        TranslationOptions{},
	}

	if err := manager.create(session); err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}

	payload := map[string]any{
		"id":             session.ID,
		"source":         map[string]any{"type": "rtmp", "uri": "rtmp://localhost/live"},
		"targetLanguage": "fr",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler := createSessionHandler(manager, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rr.Code)
	}
}

func TestGetSessionHandler_NotFound(t *testing.T) {
	manager := newTranslationSessionManager()
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodGet, "/sessions/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	handler := getSessionHandler(manager, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestGetSessionHandler_Success(t *testing.T) {
	manager := newTranslationSessionManager()
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	session := TranslationSession{
		ID: "existing1",
		Source: TranslationSource{
			Type: "dash",
			URI:  "https://example.com/manifest.mpd",
		},
		TargetLanguage: "de",
		Options:        TranslationOptions{ModelProfile: "cpu-basic"},
	}

	if err := manager.create(session); err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sessions/existing1", nil)
	req.SetPathValue("id", "existing1")
	rr := httptest.NewRecorder()

	handler := getSessionHandler(manager, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var got TranslationSession
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got.ID != session.ID {
		t.Fatalf("expected session ID %s, got %s", session.ID, got.ID)
	}
}
