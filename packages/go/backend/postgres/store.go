package postgres

import (
	"context"
	"database/sql"
	"errors"

	sessionpkg "streamlation/packages/backend/session"
)

type row interface {
	Scan(dest ...any) error
}

type rows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}

type executor interface {
	Exec(ctx context.Context, query string, args ...any) error
	QueryRow(ctx context.Context, query string, args ...any) row
	Query(ctx context.Context, query string, args ...any) (rows, error)
}

const (
	insertSessionSQL = `INSERT INTO translation_sessions (
        id,
        source_type,
        source_uri,
        target_language,
        enable_dubbing,
        latency_tolerance_ms,
        model_profile
) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	getSessionSQL    = `SELECT id, source_type, source_uri, target_language, enable_dubbing, latency_tolerance_ms, model_profile FROM translation_sessions WHERE id = $1`
	deleteSessionSQL = `DELETE FROM translation_sessions WHERE id = $1`
	listSessionsSQL  = `SELECT id, source_type, source_uri, target_language, enable_dubbing, latency_tolerance_ms, model_profile FROM translation_sessions ORDER BY created_at DESC LIMIT $1`
)

func NewSessionStore(client executor) *SessionStore {
	return &SessionStore{client: client}
}

type SessionStore struct {
	client executor
}

func (s *SessionStore) Create(ctx context.Context, session sessionpkg.TranslationSession) error {
	err := s.client.Exec(ctx, insertSessionSQL,
		session.ID,
		session.Source.Type,
		session.Source.URI,
		session.TargetLanguage,
		session.Options.EnableDubbing,
		session.Options.LatencyToleranceMs,
		session.Options.ModelProfile,
	)
	if err != nil {
		var pgErr *Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSessionExists
		}
		return err
	}
	return nil
}

func (s *SessionStore) Get(ctx context.Context, id string) (sessionpkg.TranslationSession, error) {
	result, err := scanSession(s.client.QueryRow(ctx, getSessionSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sessionpkg.TranslationSession{}, ErrSessionNotFound
		}
		return sessionpkg.TranslationSession{}, err
	}
	return result, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	return s.client.Exec(ctx, deleteSessionSQL, id)
}

func (s *SessionStore) List(ctx context.Context, limit int) ([]sessionpkg.TranslationSession, error) {
	if limit <= 0 {
		limit = 50
	}

	rs, err := s.client.Query(ctx, listSessionsSQL, limit)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	sessions := make([]sessionpkg.TranslationSession, 0)
	for rs.Next() {
		session, err := scanSession(rs)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	if err := rs.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

func scanSession(scanner interface{ Scan(dest ...any) error }) (sessionpkg.TranslationSession, error) {
	var (
		id             string
		sourceType     string
		sourceURI      string
		targetLanguage string
		enableDubbing  bool
		latency        int32
		modelProfile   string
	)

	if err := scanner.Scan(&id, &sourceType, &sourceURI, &targetLanguage, &enableDubbing, &latency, &modelProfile); err != nil {
		return sessionpkg.TranslationSession{}, err
	}

	return sessionpkg.TranslationSession{
		ID: id,
		Source: sessionpkg.TranslationSource{
			Type: sourceType,
			URI:  sourceURI,
		},
		TargetLanguage: targetLanguage,
		Options: sessionpkg.TranslationOptions{
			EnableDubbing:      enableDubbing,
			LatencyToleranceMs: int(latency),
			ModelProfile:       modelProfile,
		},
	}, nil
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

var (
	ErrSessionExists   = errors.New("session already exists")
	ErrSessionNotFound = errors.New("session not found")
)
