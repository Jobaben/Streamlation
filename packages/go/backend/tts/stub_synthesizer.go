package tts

import (
	"context"
	"time"

	"streamlation/packages/backend/translation"
)

// StubSynthesizerConfig configures the stub synthesizer behavior.
type StubSynthesizerConfig struct {
	// ProcessingDelay simulates TTS processing time.
	ProcessingDelay time.Duration
	// SampleRate for generated audio.
	SampleRate int
	// AvailableVoices are the voices to report as available.
	AvailableVoices map[string][]VoiceProfile // [lang][]VoiceProfile
}

// DefaultStubSynthesizerConfig returns sensible defaults for testing.
func DefaultStubSynthesizerConfig() *StubSynthesizerConfig {
	return &StubSynthesizerConfig{
		ProcessingDelay: 100 * time.Millisecond,
		SampleRate:      22050,
		AvailableVoices: map[string][]VoiceProfile{
			"en": {
				{ID: "en-us-1", Name: "English US Female", Language: "en", Gender: "female"},
				{ID: "en-us-2", Name: "English US Male", Language: "en", Gender: "male"},
			},
			"es": {
				{ID: "es-es-1", Name: "Spanish Female", Language: "es", Gender: "female"},
				{ID: "es-es-2", Name: "Spanish Male", Language: "es", Gender: "male"},
			},
			"fr": {
				{ID: "fr-fr-1", Name: "French Female", Language: "fr", Gender: "female"},
				{ID: "fr-fr-2", Name: "French Male", Language: "fr", Gender: "male"},
			},
		},
	}
}

// StubSynthesizer is a test implementation that returns deterministic audio.
type StubSynthesizer struct {
	config *StubSynthesizerConfig
}

// NewStubSynthesizer creates a new stub synthesizer with the given config.
func NewStubSynthesizer(config *StubSynthesizerConfig) *StubSynthesizer {
	if config == nil {
		config = DefaultStubSynthesizerConfig()
	}
	return &StubSynthesizer{config: config}
}

// Synthesize generates audio from text.
func (s *StubSynthesizer) Synthesize(ctx context.Context, text string, voice VoiceProfile) (AudioSegment, error) {
	// Simulate processing delay
	if s.config.ProcessingDelay > 0 {
		select {
		case <-time.After(s.config.ProcessingDelay):
		case <-ctx.Done():
			return AudioSegment{}, ctx.Err()
		}
	}

	// Estimate duration based on text length (rough: 150 words/min)
	wordCount := len(text) / 5 // approximate words
	if wordCount < 1 {
		wordCount = 1
	}
	duration := time.Duration(wordCount) * 400 * time.Millisecond

	// Generate deterministic PCM data (silence with pattern)
	sampleCount := int(duration.Seconds() * float64(s.config.SampleRate))
	pcmData := make([]byte, sampleCount*2) // 16-bit samples

	return AudioSegment{
		PCMData:    pcmData,
		SampleRate: s.config.SampleRate,
		Duration:   duration,
	}, nil
}

// SynthesizeStream processes streaming translations.
func (s *StubSynthesizer) SynthesizeStream(ctx context.Context, sessionID string, translations <-chan translation.Translation, voice VoiceProfile) (<-chan AudioSegment, error) {
	out := make(chan AudioSegment)

	go func() {
		defer close(out)

		for trans := range translations {
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

			// Estimate duration
			wordCount := len(trans.TranslatedText) / 5
			if wordCount < 1 {
				wordCount = 1
			}
			duration := time.Duration(wordCount) * 400 * time.Millisecond

			// Generate audio segment
			sampleCount := int(duration.Seconds() * float64(s.config.SampleRate))
			segment := AudioSegment{
				PCMData:    make([]byte, sampleCount*2),
				SampleRate: s.config.SampleRate,
				Duration:   duration,
				Timestamp:  trans.StartTime,
				SessionID:  sessionID,
			}

			select {
			case out <- segment:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// AvailableVoices returns supported voice profiles for a language.
func (s *StubSynthesizer) AvailableVoices(lang string) []VoiceProfile {
	if voices, ok := s.config.AvailableVoices[lang]; ok {
		return voices
	}
	return nil
}

// Health returns the health status of the stub synthesizer.
func (s *StubSynthesizer) Health() HealthStatus {
	return HealthStatus{
		Healthy: true,
		Message: "stub synthesizer ready",
	}
}
