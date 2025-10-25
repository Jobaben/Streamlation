package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	sessionpkg "streamlation/packages/backend/session"
)

type executor interface {
	Exec(ctx context.Context, query string) error
	QueryRow(ctx context.Context, query string) ([]string, error)
}

func NewSessionStore(client executor) *SessionStore {
	return &SessionStore{client: client}
}

type SessionStore struct {
	client executor
}

func (s *SessionStore) Create(ctx context.Context, session sessionpkg.TranslationSession) error {
	query := buildInsertSessionQuery(session)
	if err := s.client.Exec(ctx, query); err != nil {
		var pgErr *Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSessionExists
		}
		return err
	}
	return nil
}

func (s *SessionStore) Get(ctx context.Context, id string) (sessionpkg.TranslationSession, error) {
	query := fmt.Sprintf("SELECT id, source_type, source_uri, target_language, enable_dubbing, latency_tolerance_ms, model_profile FROM translation_sessions WHERE id = %s LIMIT 1", quoteLiteral(id))
	row, err := s.client.QueryRow(ctx, query)
	if err != nil {
		return sessionpkg.TranslationSession{}, err
	}
	if row == nil {
		return sessionpkg.TranslationSession{}, ErrSessionNotFound
	}

	if len(row) != 7 {
		return sessionpkg.TranslationSession{}, fmt.Errorf("unexpected column count: %d", len(row))
	}

	latency, err := strconv.Atoi(row[5])
	if err != nil {
		return sessionpkg.TranslationSession{}, fmt.Errorf("invalid latency value: %w", err)
	}

	enableDubbing := parseBool(row[4])

	session := sessionpkg.TranslationSession{
		ID: row[0],
		Source: sessionpkg.TranslationSource{
			Type: row[1],
			URI:  row[2],
		},
		TargetLanguage: row[3],
		Options: sessionpkg.TranslationOptions{
			EnableDubbing:      enableDubbing,
			LatencyToleranceMs: latency,
			ModelProfile:       row[6],
		},
	}

	return session, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	query := fmt.Sprintf("DELETE FROM translation_sessions WHERE id = %s", quoteLiteral(id))
	return s.client.Exec(ctx, query)
}

func EnsureSessionSchema(ctx context.Context, client executor) error {
	const ddl = `CREATE TABLE IF NOT EXISTS translation_sessions (
id TEXT PRIMARY KEY,
source_type TEXT NOT NULL,
source_uri TEXT NOT NULL,
target_language TEXT NOT NULL,
enable_dubbing BOOLEAN NOT NULL,
latency_tolerance_ms INTEGER NOT NULL,
model_profile TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
	return client.Exec(ctx, ddl)
}

func buildInsertSessionQuery(session sessionpkg.TranslationSession) string {
	values := []string{
		quoteLiteral(session.ID),
		quoteLiteral(session.Source.Type),
		quoteLiteral(session.Source.URI),
		quoteLiteral(session.TargetLanguage),
		boolLiteral(session.Options.EnableDubbing),
		strconv.Itoa(session.Options.LatencyToleranceMs),
		quoteLiteral(session.Options.ModelProfile),
	}

	return fmt.Sprintf(
		"INSERT INTO translation_sessions (id, source_type, source_uri, target_language, enable_dubbing, latency_tolerance_ms, model_profile) VALUES (%s)",
		strings.Join(values, ", "),
	)
}

func quoteLiteral(value string) string {
	escaped := strings.ReplaceAll(value, "'", "''")
	return "'" + escaped + "'"
}

func boolLiteral(v bool) string {
	if v {
		return "TRUE"
	}
	return "FALSE"
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "t", "true", "1", "y", "yes":
		return true
	default:
		return false
	}
}

var (
	// ErrSessionExists is returned when attempting to create a session that already exists.
	ErrSessionExists = errors.New("session already exists")

	// ErrSessionNotFound is returned when a session cannot be found in storage.
	ErrSessionNotFound = errors.New("session not found")
)
