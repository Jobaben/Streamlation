package media

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestStubNormalizer_Normalize(t *testing.T) {
	t.Parallel()

	config := &StubNormalizerConfig{
		ChunkDuration: 50 * time.Millisecond,
		ChunkDelay:    10 * time.Millisecond,
		TotalChunks:   5,
		SampleRate:    16000,
	}
	normalizer := NewStubNormalizer(config)

	ctx := context.Background()
	source := bytes.NewReader([]byte{})

	chunks, err := normalizer.Normalize(ctx, source)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	var received []AudioChunk
	for chunk := range chunks {
		received = append(received, chunk)
	}

	if len(received) != config.TotalChunks {
		t.Errorf("expected %d chunks, got %d", config.TotalChunks, len(received))
	}

	// Verify chunk properties
	for i, chunk := range received {
		if chunk.SampleRate != config.SampleRate {
			t.Errorf("chunk %d: expected sample rate %d, got %d", i, config.SampleRate, chunk.SampleRate)
		}
		if chunk.Channels != 1 {
			t.Errorf("chunk %d: expected 1 channel, got %d", i, chunk.Channels)
		}
		expectedTimestamp := time.Duration(i) * config.ChunkDuration
		if chunk.Timestamp != expectedTimestamp {
			t.Errorf("chunk %d: expected timestamp %v, got %v", i, expectedTimestamp, chunk.Timestamp)
		}
	}
}

func TestStubNormalizer_ContextCancellation(t *testing.T) {
	t.Parallel()

	config := &StubNormalizerConfig{
		ChunkDuration: 50 * time.Millisecond,
		ChunkDelay:    100 * time.Millisecond, // Long delay to ensure cancellation
		TotalChunks:   100,
		SampleRate:    16000,
	}
	normalizer := NewStubNormalizer(config)

	ctx, cancel := context.WithCancel(context.Background())
	source := bytes.NewReader([]byte{})

	chunks, err := normalizer.Normalize(ctx, source)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	// Read a few chunks then cancel
	count := 0
	for range chunks {
		count++
		if count >= 2 {
			cancel()
			break
		}
	}

	// Drain remaining chunks (should be few due to cancellation)
	for range chunks {
		count++
	}

	if count >= config.TotalChunks {
		t.Errorf("expected fewer than %d chunks due to cancellation, got %d", config.TotalChunks, count)
	}
}

func TestStubNormalizer_Health(t *testing.T) {
	t.Parallel()

	normalizer := NewStubNormalizer(nil)
	status := normalizer.Health()

	if !status.Healthy {
		t.Error("expected healthy status")
	}
	if status.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestStubNormalizer_DefaultConfig(t *testing.T) {
	t.Parallel()

	normalizer := NewStubNormalizer(nil)
	if normalizer.config == nil {
		t.Error("expected default config")
	}
	if normalizer.config.SampleRate != 16000 {
		t.Errorf("expected default sample rate 16000, got %d", normalizer.config.SampleRate)
	}
}
