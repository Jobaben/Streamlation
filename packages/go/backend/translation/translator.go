package translation

import (
	"context"
	"time"

	"streamlation/packages/backend/asr"
)

// Translation represents a translated segment.
type Translation struct {
	// SourceText is the original text that was translated.
	SourceText string `json:"sourceText"`
	// TranslatedText is the translated result.
	TranslatedText string `json:"translatedText"`
	// SourceLang is the source language (ISO 639-1 code).
	SourceLang string `json:"sourceLang"`
	// TargetLang is the target language (ISO 639-1 code).
	TargetLang string `json:"targetLang"`
	// Confidence is the translation confidence score (0.0 - 1.0).
	Confidence float64 `json:"confidence"`
	// StartTime is when this segment begins in the source.
	StartTime time.Duration `json:"startTime"`
	// EndTime is when this segment ends in the source.
	EndTime time.Duration `json:"endTime"`
	// SessionID identifies the translation session.
	SessionID string `json:"sessionId"`
}

// LanguagePair represents a supported source-target language combination.
type LanguagePair struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// HealthStatus represents the health of a component.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// Translator converts text between languages.
type Translator interface {
	// Translate converts a single text segment to the target language.
	Translate(ctx context.Context, text string, sourceLang, targetLang string) (Translation, error)

	// TranslateStream processes streaming transcripts and returns translations.
	TranslateStream(ctx context.Context, sessionID string, transcripts <-chan asr.Transcript, targetLang string) (<-chan Translation, error)

	// SupportedLanguages returns available language pairs.
	SupportedLanguages() []LanguagePair

	// Health returns the current health status of the translator.
	Health() HealthStatus
}
