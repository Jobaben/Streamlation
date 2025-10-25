package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	postgres "streamlation/packages/backend/postgres"
	sessionpkg "streamlation/packages/backend/session"

	"go.uber.org/zap"
)

var (
	sessionIDPattern      = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,64}$`)
	targetLanguagePattern = regexp.MustCompile(`^[a-z]{2}$`)

	allowedSourceTypes = map[string]struct{}{
		"hls":  {},
		"dash": {},
		"rtmp": {},
		"file": {},
	}

	allowedModelProfiles = map[string]struct{}{
		"cpu-basic":       {},
		"cpu-advanced":    {},
		"gpu-accelerated": {},
	}
)

type TranslationSession = sessionpkg.TranslationSession
type TranslationSource = sessionpkg.TranslationSource
type TranslationOptions = sessionpkg.TranslationOptions

type translationSessionInput struct {
	ID             string                   `json:"id"`
	Source         *TranslationSource       `json:"source"`
	TargetLanguage string                   `json:"targetLanguage"`
	Options        *translationOptionsInput `json:"options"`
}

type translationOptionsInput struct {
	EnableDubbing      *bool   `json:"enableDubbing"`
	LatencyToleranceMs *int    `json:"latencyToleranceMs"`
	ModelProfile       *string `json:"modelProfile"`
}

// SessionStore persists and retrieves translation sessions.
type SessionStore interface {
	Create(ctx context.Context, session TranslationSession) error
	Get(ctx context.Context, id string) (TranslationSession, error)
	Delete(ctx context.Context, id string) error
}

var (
	ErrSessionExists   = postgres.ErrSessionExists
	ErrSessionNotFound = postgres.ErrSessionNotFound
)

// IngestionEnqueuer enqueues ingestion jobs for downstream processing.
type IngestionEnqueuer interface {
	EnqueueIngestion(ctx context.Context, sessionID string) error
}

func createSessionHandler(store SessionStore, enqueuer IngestionEnqueuer, logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		defer func() {
			if err := r.Body.Close(); err != nil {
				logger.Errorw("failed to close request body", "error", err)
			}
		}()

		var input translationSessionInput
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(w, logger, http.StatusBadRequest, fmt.Errorf("invalid payload: %w", err))
			return
		}

		session, err := normalizeAndValidateSession(input)
		if err != nil {
			writeError(w, logger, http.StatusBadRequest, err)
			return
		}

		ctx := r.Context()

		if err := store.Create(ctx, session); err != nil {
			if errors.Is(err, ErrSessionExists) {
				writeError(w, logger, http.StatusConflict, err)
				return
			}
			writeError(w, logger, http.StatusInternalServerError, fmt.Errorf("failed to persist session: %w", err))
			return
		}

		if err := enqueuer.EnqueueIngestion(ctx, session.ID); err != nil {
			logger.Errorw("failed to enqueue ingestion job", "error", err, "sessionID", session.ID)
			if deleteErr := store.Delete(ctx, session.ID); deleteErr != nil {
				logger.Errorw("failed to roll back session after enqueue error", "error", deleteErr, "sessionID", session.ID)
			}
			writeError(w, logger, http.StatusInternalServerError, errors.New("failed to enqueue ingestion job"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(session); err != nil {
			logger.Errorw("failed to encode response", "error", err)
		}
	}
}

func getSessionHandler(store SessionStore, logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := r.PathValue("id")
		if id == "" {
			writeError(w, logger, http.StatusBadRequest, errors.New("missing session id"))
			return
		}

		ctx := r.Context()

		session, err := store.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				writeError(w, logger, http.StatusNotFound, fmt.Errorf("session %s not found", id))
				return
			}
			writeError(w, logger, http.StatusInternalServerError, fmt.Errorf("failed to load session: %w", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(session); err != nil {
			logger.Errorw("failed to encode response", "error", err)
		}
	}
}

func normalizeAndValidateSession(input translationSessionInput) (TranslationSession, error) {
	if !sessionIDPattern.MatchString(input.ID) {
		return TranslationSession{}, fmt.Errorf("id must match %s", sessionIDPattern.String())
	}

	if input.Source == nil {
		return TranslationSession{}, errors.New("source is required")
	}

	if _, ok := allowedSourceTypes[input.Source.Type]; !ok {
		return TranslationSession{}, fmt.Errorf("unsupported source.type: %s", input.Source.Type)
	}

	if _, err := url.ParseRequestURI(input.Source.URI); err != nil {
		return TranslationSession{}, fmt.Errorf("invalid source.uri: %w", err)
	}

	if !targetLanguagePattern.MatchString(input.TargetLanguage) {
		return TranslationSession{}, errors.New("targetLanguage must be a two-letter lowercase code")
	}

	options := TranslationOptions{
		EnableDubbing:      false,
		LatencyToleranceMs: 5000,
		ModelProfile:       "cpu-basic",
	}

	if input.Options != nil {
		if input.Options.EnableDubbing != nil {
			options.EnableDubbing = *input.Options.EnableDubbing
		}
		if input.Options.LatencyToleranceMs != nil {
			if *input.Options.LatencyToleranceMs < 0 || *input.Options.LatencyToleranceMs > 60000 {
				return TranslationSession{}, errors.New("options.latencyToleranceMs must be between 0 and 60000")
			}
			options.LatencyToleranceMs = *input.Options.LatencyToleranceMs
		}
		if input.Options.ModelProfile != nil {
			if _, ok := allowedModelProfiles[*input.Options.ModelProfile]; !ok {
				return TranslationSession{}, fmt.Errorf("unsupported options.modelProfile: %s", *input.Options.ModelProfile)
			}
			options.ModelProfile = *input.Options.ModelProfile
		}
	}

	session := TranslationSession{
		ID:             input.ID,
		Source:         *input.Source,
		TargetLanguage: input.TargetLanguage,
		Options:        options,
	}

	return session, nil
}

func writeError(w http.ResponseWriter, logger *zap.SugaredLogger, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := map[string]string{"error": err.Error()}
	if encodeErr := json.NewEncoder(w).Encode(payload); encodeErr != nil {
		logger.Errorw("failed to encode error response", "error", encodeErr)
	}
}
