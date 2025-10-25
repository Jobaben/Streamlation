package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"

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

// TranslationSession models the configuration for a translation session.
type TranslationSession struct {
	ID             string             `json:"id"`
	Source         TranslationSource  `json:"source"`
	TargetLanguage string             `json:"targetLanguage"`
	Options        TranslationOptions `json:"options"`
}

// TranslationSource describes the input stream configuration.
type TranslationSource struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

// TranslationOptions contains tuning values for a session.
type TranslationOptions struct {
	EnableDubbing      bool   `json:"enableDubbing"`
	LatencyToleranceMs int    `json:"latencyToleranceMs"`
	ModelProfile       string `json:"modelProfile"`
}

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

func newTranslationSessionManager() *translationSessionManager {
	return &translationSessionManager{
		sessions: make(map[string]TranslationSession),
	}
}

type translationSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]TranslationSession
}

func (m *translationSessionManager) create(session TranslationSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[session.ID]; exists {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	m.sessions[session.ID] = session
	return nil
}

func (m *translationSessionManager) get(id string) (TranslationSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	return session, ok
}

func createSessionHandler(mgr *translationSessionManager, logger *zap.SugaredLogger) http.HandlerFunc {
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

		if err := mgr.create(session); err != nil {
			writeError(w, logger, http.StatusConflict, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(session); err != nil {
			logger.Errorw("failed to encode response", "error", err)
		}
	}
}

func getSessionHandler(mgr *translationSessionManager, logger *zap.SugaredLogger) http.HandlerFunc {
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

		session, ok := mgr.get(id)
		if !ok {
			writeError(w, logger, http.StatusNotFound, fmt.Errorf("session %s not found", id))
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
