package tts

import (
	"context"
	"time"

	"streamlation/packages/backend/translation"
)

// AudioSegment represents a synthesized audio segment.
type AudioSegment struct {
	// PCMData contains the raw PCM audio samples.
	PCMData []byte `json:"pcmData"`
	// SampleRate is the audio sample rate in Hz.
	SampleRate int `json:"sampleRate"`
	// Duration of this audio segment.
	Duration time.Duration `json:"duration"`
	// Timestamp marks alignment with the source stream.
	Timestamp time.Duration `json:"timestamp"`
	// SessionID identifies the translation session.
	SessionID string `json:"sessionId"`
}

// VoiceProfile specifies voice characteristics for synthesis.
type VoiceProfile struct {
	// ID is the voice identifier.
	ID string `json:"id"`
	// Name is the human-readable voice name.
	Name string `json:"name"`
	// Language is the voice's primary language.
	Language string `json:"language"`
	// Gender is the voice gender (male, female, neutral).
	Gender string `json:"gender"`
}

// HealthStatus represents the health of a component.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// Synthesizer converts text to speech audio.
type Synthesizer interface {
	// Synthesize generates audio from text using the specified voice.
	Synthesize(ctx context.Context, text string, voice VoiceProfile) (AudioSegment, error)

	// SynthesizeStream processes streaming translations and returns audio segments.
	SynthesizeStream(ctx context.Context, sessionID string, translations <-chan translation.Translation, voice VoiceProfile) (<-chan AudioSegment, error)

	// AvailableVoices returns supported voice profiles for a language.
	AvailableVoices(lang string) []VoiceProfile

	// Health returns the current health status of the synthesizer.
	Health() HealthStatus
}
