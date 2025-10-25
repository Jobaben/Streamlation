package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	sessionpkg "streamlation/packages/backend/session"
)

func TestBuildInsertSessionQuery(t *testing.T) {
	session := sessionpkg.TranslationSession{
		ID:             "abc123",
		Source:         sessionpkg.TranslationSource{Type: "hls", URI: "https://example.com/a.m3u8"},
		TargetLanguage: "en",
		Options:        sessionpkg.TranslationOptions{EnableDubbing: true, LatencyToleranceMs: 1500, ModelProfile: "cpu-basic"},
	}

	query := buildInsertSessionQuery(session)
	if !strings.Contains(query, "INSERT INTO translation_sessions") {
		t.Fatalf("unexpected query: %s", query)
	}
	if !strings.Contains(query, "'abc123'") {
		t.Fatalf("expected id literal in query: %s", query)
	}
	if !strings.Contains(query, "TRUE") {
		t.Fatalf("expected TRUE literal in query: %s", query)
	}
}

func TestSessionStore_CreateDuplicate(t *testing.T) {
	expectedQuery := ""
	client := &stubExecutor{
		execFunc: func(_ context.Context, query string) error {
			expectedQuery = query
			return &Error{Code: "23505", Message: "duplicate"}
		},
	}

	store := NewSessionStore(client)
	session := sessionpkg.TranslationSession{
		ID:             "dup",
		Source:         sessionpkg.TranslationSource{Type: "hls", URI: "https://example.com"},
		TargetLanguage: "fr",
		Options:        sessionpkg.TranslationOptions{},
	}

	err := store.Create(context.Background(), session)
	if !errors.Is(err, ErrSessionExists) {
		t.Fatalf("expected ErrSessionExists, got %v", err)
	}

	if expectedQuery == "" {
		t.Fatal("expected query to be executed")
	}
}

func TestSessionStore_Get(t *testing.T) {
	client := &stubExecutor{
		queryRowFunc: func(_ context.Context, query string) ([]string, error) {
			if !strings.Contains(query, "WHERE id = 'known'") {
				t.Fatalf("unexpected query: %s", query)
			}
			return []string{"known", "hls", "https://example.com", "es", "t", "3000", "gpu-accelerated"}, nil
		},
	}

	store := NewSessionStore(client)
	session, err := store.Get(context.Background(), "known")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.Options.ModelProfile != "gpu-accelerated" {
		t.Fatalf("unexpected model profile: %s", session.Options.ModelProfile)
	}
	if !session.Options.EnableDubbing {
		t.Fatal("expected enable dubbing to be true")
	}
	if session.Options.LatencyToleranceMs != 3000 {
		t.Fatalf("unexpected latency: %d", session.Options.LatencyToleranceMs)
	}
}

func TestSessionStore_GetNotFound(t *testing.T) {
	client := &stubExecutor{queryRowFunc: func(context.Context, string) ([]string, error) { return nil, nil }}
	store := NewSessionStore(client)
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	var executed bool
	client := &stubExecutor{execFunc: func(context.Context, string) error {
		executed = true
		return nil
	}}

	store := NewSessionStore(client)
	if err := store.Delete(context.Background(), "id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected delete query execution")
	}
}

type stubExecutor struct {
	execFunc     func(context.Context, string) error
	queryRowFunc func(context.Context, string) ([]string, error)
}

func (s *stubExecutor) Exec(ctx context.Context, query string) error {
	if s.execFunc != nil {
		return s.execFunc(ctx, query)
	}
	return nil
}

func (s *stubExecutor) QueryRow(ctx context.Context, query string) ([]string, error) {
	if s.queryRowFunc != nil {
		return s.queryRowFunc(ctx, query)
	}
	return nil, nil
}
