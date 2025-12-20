package media

import (
	"context"
	"io"
	"time"
)

// StubNormalizerConfig configures the stub normalizer behavior.
type StubNormalizerConfig struct {
	// ChunkDuration is the duration of each emitted chunk.
	ChunkDuration time.Duration
	// ChunkDelay is the delay between emitting chunks (simulates processing).
	ChunkDelay time.Duration
	// TotalChunks is the number of chunks to emit (0 = unlimited until context done).
	TotalChunks int
	// SampleRate for generated chunks.
	SampleRate int
	// ErrorAfter causes an error after N chunks (0 = no error).
	ErrorAfter int
	// ErrorMessage is the error message to return.
	ErrorMessage string
}

// DefaultStubNormalizerConfig returns sensible defaults for testing.
func DefaultStubNormalizerConfig() *StubNormalizerConfig {
	return &StubNormalizerConfig{
		ChunkDuration: 100 * time.Millisecond,
		ChunkDelay:    50 * time.Millisecond,
		TotalChunks:   10,
		SampleRate:    16000,
	}
}

// StubNormalizer is a test implementation that emits deterministic audio chunks.
type StubNormalizer struct {
	config *StubNormalizerConfig
}

// NewStubNormalizer creates a new stub normalizer with the given config.
// If config is nil, defaults are used.
func NewStubNormalizer(config *StubNormalizerConfig) *StubNormalizer {
	if config == nil {
		config = DefaultStubNormalizerConfig()
	}
	return &StubNormalizer{config: config}
}

// Normalize emits a deterministic sequence of audio chunks for testing.
func (s *StubNormalizer) Normalize(ctx context.Context, source io.Reader) (<-chan AudioChunk, error) {
	out := make(chan AudioChunk)

	go func() {
		defer close(out)

		var timestamp time.Duration
		for i := 0; s.config.TotalChunks == 0 || i < s.config.TotalChunks; i++ {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Simulate processing delay
			if s.config.ChunkDelay > 0 {
				select {
				case <-time.After(s.config.ChunkDelay):
				case <-ctx.Done():
					return
				}
			}

			// Generate deterministic PCM data (silence with small variation)
			pcmSize := int(s.config.ChunkDuration.Seconds() * float64(s.config.SampleRate) * 2) // 16-bit samples
			pcmData := make([]byte, pcmSize)
			// Add a small pattern for testing
			for j := 0; j < len(pcmData); j += 2 {
				pcmData[j] = byte(i % 256)
			}

			chunk := AudioChunk{
				Timestamp:  timestamp,
				SampleRate: s.config.SampleRate,
				Channels:   1,
				PCMData:    pcmData,
				RMS:        0.01 * float64(i+1),
				Duration:   s.config.ChunkDuration,
			}

			select {
			case out <- chunk:
				timestamp += s.config.ChunkDuration
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Health returns the health status of the stub normalizer.
func (s *StubNormalizer) Health() HealthStatus {
	return HealthStatus{
		Healthy: true,
		Message: "stub normalizer ready",
	}
}
