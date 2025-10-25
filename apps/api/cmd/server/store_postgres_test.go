package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildInsertSessionQuery(t *testing.T) {
	session := TranslationSession{
		ID:             "abc123",
		Source:         TranslationSource{Type: "hls", URI: "https://example.com/a.m3u8"},
		TargetLanguage: "en",
		Options:        TranslationOptions{EnableDubbing: true, LatencyToleranceMs: 1500, ModelProfile: "cpu-basic"},
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

func TestPostgresSessionStore_CreateDuplicate(t *testing.T) {
	expectedQuery := ""
	client := &stubPGExecutor{
		execFunc: func(_ context.Context, query string) error {
			expectedQuery = query
			return &pgError{Code: "23505", Message: "duplicate"}
		},
	}

	store := NewPostgresSessionStore(client)
	session := TranslationSession{
		ID:             "dup",
		Source:         TranslationSource{Type: "hls", URI: "https://example.com"},
		TargetLanguage: "fr",
		Options:        TranslationOptions{},
	}

	err := store.Create(context.Background(), session)
	if !errors.Is(err, ErrSessionExists) {
		t.Fatalf("expected ErrSessionExists, got %v", err)
	}

	if expectedQuery == "" {
		t.Fatal("expected query to be executed")
	}
}

func TestPostgresSessionStore_Get(t *testing.T) {
	client := &stubPGExecutor{
		queryRowFunc: func(_ context.Context, query string) ([]string, error) {
			if !strings.Contains(query, "WHERE id = 'known'") {
				t.Fatalf("unexpected query: %s", query)
			}
			return []string{"known", "hls", "https://example.com", "es", "t", "3000", "gpu-accelerated"}, nil
		},
	}

	store := NewPostgresSessionStore(client)
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

func TestPostgresSessionStore_GetNotFound(t *testing.T) {
	client := &stubPGExecutor{queryRowFunc: func(context.Context, string) ([]string, error) { return nil, nil }}
	store := NewPostgresSessionStore(client)
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestPostgresSessionStore_Delete(t *testing.T) {
	var executed bool
	client := &stubPGExecutor{execFunc: func(context.Context, string) error {
		executed = true
		return nil
	}}

	store := NewPostgresSessionStore(client)
	if err := store.Delete(context.Background(), "id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected delete query execution")
	}
}

type stubPGExecutor struct {
	execFunc     func(context.Context, string) error
	queryRowFunc func(context.Context, string) ([]string, error)
}

func (s *stubPGExecutor) Exec(ctx context.Context, query string) error {
	if s.execFunc != nil {
		return s.execFunc(ctx, query)
	}
	return nil
}

func (s *stubPGExecutor) QueryRow(ctx context.Context, query string) ([]string, error) {
	if s.queryRowFunc != nil {
		return s.queryRowFunc(ctx, query)
	}
	return nil, nil
}
