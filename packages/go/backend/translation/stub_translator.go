package translation

import (
	"context"
	"time"

	"streamlation/packages/backend/asr"
)

// StubTranslatorConfig configures the stub translator behavior.
type StubTranslatorConfig struct {
	// ProcessingDelay simulates translation processing time.
	ProcessingDelay time.Duration
	// Dictionary maps source text to translated text.
	// If nil, returns "[LANG] " prefix + original text.
	Dictionary map[string]map[string]string // [targetLang][sourceText]translatedText
	// SupportedPairs defines available language pairs.
	SupportedPairs []LanguagePair
}

// DefaultStubTranslatorConfig returns sensible defaults for testing.
func DefaultStubTranslatorConfig() *StubTranslatorConfig {
	return &StubTranslatorConfig{
		ProcessingDelay: 50 * time.Millisecond,
		Dictionary: map[string]map[string]string{
			"es": {
				"Hello world.":              "Hola mundo.",
				"This is a test.":           "Esto es una prueba.",
				"Welcome to Streamlation.":  "Bienvenido a Streamlation.",
				"Real-time translation.":    "Traducción en tiempo real.",
				"Thank you for watching.":   "Gracias por ver.",
			},
			"fr": {
				"Hello world.":              "Bonjour le monde.",
				"This is a test.":           "Ceci est un test.",
				"Welcome to Streamlation.":  "Bienvenue sur Streamlation.",
				"Real-time translation.":    "Traduction en temps réel.",
				"Thank you for watching.":   "Merci d'avoir regardé.",
			},
		},
		SupportedPairs: []LanguagePair{
			{Source: "en", Target: "es"},
			{Source: "en", Target: "fr"},
			{Source: "es", Target: "en"},
			{Source: "fr", Target: "en"},
		},
	}
}

// StubTranslator is a test implementation that returns deterministic translations.
type StubTranslator struct {
	config *StubTranslatorConfig
}

// NewStubTranslator creates a new stub translator with the given config.
func NewStubTranslator(config *StubTranslatorConfig) *StubTranslator {
	if config == nil {
		config = DefaultStubTranslatorConfig()
	}
	return &StubTranslator{config: config}
}

// Translate converts a single text segment.
func (s *StubTranslator) Translate(ctx context.Context, text string, sourceLang, targetLang string) (Translation, error) {
	// Simulate processing delay
	if s.config.ProcessingDelay > 0 {
		select {
		case <-time.After(s.config.ProcessingDelay):
		case <-ctx.Done():
			return Translation{}, ctx.Err()
		}
	}

	translated := s.lookupTranslation(text, targetLang)

	return Translation{
		SourceText:     text,
		TranslatedText: translated,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		Confidence:     0.92,
	}, nil
}

// TranslateStream processes streaming transcripts.
func (s *StubTranslator) TranslateStream(ctx context.Context, sessionID string, transcripts <-chan asr.Transcript, targetLang string) (<-chan Translation, error) {
	out := make(chan Translation)

	go func() {
		defer close(out)

		for transcript := range transcripts {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Simulate processing delay
			if s.config.ProcessingDelay > 0 {
				select {
				case <-time.After(s.config.ProcessingDelay):
				case <-ctx.Done():
					return
				}
			}

			translated := s.lookupTranslation(transcript.Text, targetLang)

			translation := Translation{
				SourceText:     transcript.Text,
				TranslatedText: translated,
				SourceLang:     transcript.Language,
				TargetLang:     targetLang,
				Confidence:     0.92,
				StartTime:      transcript.StartTime,
				EndTime:        transcript.EndTime,
				SessionID:      sessionID,
			}

			select {
			case out <- translation:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// lookupTranslation finds a translation in the dictionary or generates a default.
func (s *StubTranslator) lookupTranslation(text, targetLang string) string {
	if langDict, ok := s.config.Dictionary[targetLang]; ok {
		if translated, ok := langDict[text]; ok {
			return translated
		}
	}
	// Default: prefix with language code
	return "[" + targetLang + "] " + text
}

// SupportedLanguages returns available language pairs.
func (s *StubTranslator) SupportedLanguages() []LanguagePair {
	return s.config.SupportedPairs
}

// Health returns the health status of the stub translator.
func (s *StubTranslator) Health() HealthStatus {
	return HealthStatus{
		Healthy: true,
		Message: "stub translator ready",
	}
}
