package media

import (
	"context"
	"io"
	"time"
)

// AudioChunk represents a normalized audio segment ready for ASR processing.
type AudioChunk struct {
	// Timestamp marks the presentation time of this chunk in the source stream.
	Timestamp time.Duration `json:"timestamp"`
	// SampleRate is the audio sample rate in Hz (typically 16000 for ASR).
	SampleRate int `json:"sampleRate"`
	// Channels is the number of audio channels (typically 1 for mono).
	Channels int `json:"channels"`
	// PCMData contains the raw PCM audio samples.
	PCMData []byte `json:"pcmData"`
	// RMS is the root mean square amplitude for testing/debugging.
	RMS float64 `json:"rms"`
	// Duration of this audio chunk.
	Duration time.Duration `json:"duration"`
}

// HealthStatus represents the health of a component.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// Normalizer extracts and normalizes audio from media streams into
// a consistent format suitable for ASR processing.
type Normalizer interface {
	// Normalize processes a media source and emits PCM audio chunks.
	// The returned channel will be closed when processing completes or
	// the context is cancelled.
	Normalize(ctx context.Context, source io.Reader) (<-chan AudioChunk, error)

	// Health returns the current health status of the normalizer.
	Health() HealthStatus
}
