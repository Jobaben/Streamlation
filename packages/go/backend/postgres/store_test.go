package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	sessionpkg "streamlation/packages/backend/session"
)

func TestSessionStore_CreateDuplicate(t *testing.T) {
	var executedQuery string
	var executedArgs []any
	client := &stubExecutor{
		execFunc: func(_ context.Context, query string, args ...any) error {
			executedQuery = query
			executedArgs = append([]any(nil), args...)
			return &Error{Code: "23505", Message: "duplicate"}
		},
	}

	store := NewSessionStore(client)
	session := sessionpkg.TranslationSession{
		ID:             "dup",
		Source:         sessionpkg.TranslationSource{Type: "hls", URI: "https://example.com"},
		TargetLanguage: "fr",
		Options:        sessionpkg.TranslationOptions{EnableDubbing: true, LatencyToleranceMs: 1200, ModelProfile: "cpu-basic"},
	}

	err := store.Create(context.Background(), session)
	if !errors.Is(err, ErrSessionExists) {
		t.Fatalf("expected ErrSessionExists, got %v", err)
	}

	if !strings.Contains(executedQuery, "INSERT INTO translation_sessions") {
		t.Fatalf("unexpected insert query: %s", executedQuery)
	}
	if len(executedArgs) != 7 {
		t.Fatalf("expected 7 args, got %d", len(executedArgs))
	}
	if executedArgs[0] != session.ID || executedArgs[1] != session.Source.Type {
		t.Fatalf("unexpected args: %v", executedArgs)
	}
}

func TestSessionStore_Get(t *testing.T) {
	client := &stubExecutor{
		queryRowFunc: func(_ context.Context, query string, args ...any) row {
			if !strings.Contains(query, "WHERE id = $1") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 1 || args[0] != "known" {
				t.Fatalf("unexpected args: %v", args)
			}
			return stubRow{scanFunc: func(dest ...any) error {
				*(dest[0].(*string)) = "known"
				*(dest[1].(*string)) = "hls"
				*(dest[2].(*string)) = "https://example.com"
				*(dest[3].(*string)) = "es"
				*(dest[4].(*bool)) = true
				*(dest[5].(*int32)) = 3000
				*(dest[6].(*string)) = "gpu-accelerated"
				return nil
			}}
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
	client := &stubExecutor{
		queryRowFunc: func(context.Context, string, ...any) row {
			return stubRow{scanFunc: func(...any) error { return sql.ErrNoRows }}
		},
	}
	store := NewSessionStore(client)
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	var executedQuery string
	var executedArgs []any
	client := &stubExecutor{execFunc: func(_ context.Context, query string, args ...any) error {
		executedQuery = query
		executedArgs = append([]any(nil), args...)
		return nil
	}}

	store := NewSessionStore(client)
	if err := store.Delete(context.Background(), "id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(executedQuery, "DELETE FROM translation_sessions") {
		t.Fatalf("unexpected delete query: %s", executedQuery)
	}
	if len(executedArgs) != 1 || executedArgs[0] != "id" {
		t.Fatalf("unexpected args: %v", executedArgs)
	}
}

func TestSessionStore_List(t *testing.T) {
	var executedQuery string
	var executedArgs []any
	client := &stubExecutor{
		queryFunc: func(_ context.Context, query string, args ...any) (rows, error) {
			executedQuery = query
			executedArgs = append([]any(nil), args...)
			return &stubRows{scanFuncs: []func(...any) error{
				func(dest ...any) error {
					*(dest[0].(*string)) = "id1"
					*(dest[1].(*string)) = "hls"
					*(dest[2].(*string)) = "https://example.com/1"
					*(dest[3].(*string)) = "es"
					*(dest[4].(*bool)) = true
					*(dest[5].(*int32)) = 1500
					*(dest[6].(*string)) = "cpu-basic"
					return nil
				},
			}}, nil
		},
	}

	store := NewSessionStore(client)
	sessions, err := store.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "id1" {
		t.Fatalf("unexpected session id: %s", sessions[0].ID)
	}
	if !strings.Contains(executedQuery, "ORDER BY created_at DESC") {
		t.Fatalf("unexpected list query: %s", executedQuery)
	}
	if len(executedArgs) != 1 || executedArgs[0] != 50 {
		t.Fatalf("expected default limit argument, got %v", executedArgs)
	}
}

type stubExecutor struct {
	execFunc     func(context.Context, string, ...any) error
	queryRowFunc func(context.Context, string, ...any) row
	queryFunc    func(context.Context, string, ...any) (rows, error)
}

func (s *stubExecutor) Exec(ctx context.Context, query string, args ...any) error {
	if s.execFunc != nil {
		return s.execFunc(ctx, query, args...)
	}
	return nil
}

func (s *stubExecutor) QueryRow(ctx context.Context, query string, args ...any) row {
	if s.queryRowFunc != nil {
		return s.queryRowFunc(ctx, query, args...)
	}
	return stubRow{}
}

func (s *stubExecutor) Query(ctx context.Context, query string, args ...any) (rows, error) {
	if s.queryFunc != nil {
		return s.queryFunc(ctx, query, args...)
	}
	return &stubRows{}, nil
}

type stubRow struct {
	scanFunc func(...any) error
}

func (r stubRow) Scan(dest ...any) error {
	if r.scanFunc != nil {
		return r.scanFunc(dest...)
	}
	return nil
}

type stubRows struct {
	scanFuncs []func(...any) error
	idx       int
	err       error
}

func (r *stubRows) Close() {}

func (r *stubRows) Err() error { return r.err }

func (r *stubRows) Next() bool {
	if r.idx >= len(r.scanFuncs) {
		return false
	}
	r.idx++
	return true
}

func (r *stubRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.scanFuncs) {
		return errors.New("scan called out of sequence")
	}
	return r.scanFuncs[r.idx-1](dest...)
}
