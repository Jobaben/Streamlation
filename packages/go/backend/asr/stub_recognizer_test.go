package asr

import (
	"context"
	"testing"
	"time"

	"streamlation/packages/backend/media"
)

func TestStubRecognizer_Recognize(t *testing.T) {
	t.Parallel()

	recognizer := NewStubRecognizer(nil)

	ctx := context.Background()
	sessionID := "test-session"

	// Create a channel of audio chunks
	chunks := make(chan media.AudioChunk, 3)
	chunks <- media.AudioChunk{Timestamp: 0, Duration: 100 * time.Millisecond}
	chunks <- media.AudioChunk{Timestamp: 100 * time.Millisecond, Duration: 100 * time.Millisecond}
	chunks <- media.AudioChunk{Timestamp: 200 * time.Millisecond, Duration: 100 * time.Millisecond}
	close(chunks)

	transcripts, err := recognizer.Recognize(ctx, sessionID, chunks)
	if err != nil {
		t.Fatalf("Recognize failed: %v", err)
	}

	var received []Transcript
	for transcript := range transcripts {
		received = append(received, transcript)
	}

	if len(received) != 3 {
		t.Errorf("expected 3 transcripts, got %d", len(received))
	}

	// Verify transcript properties
	expectedTexts := []string{
		"Hello world.",
		"This is a test.",
		"Welcome to Streamlation.",
	}

	for i, transcript := range received {
		if transcript.SessionID != sessionID {
			t.Errorf("transcript %d: expected session ID %s, got %s", i, sessionID, transcript.SessionID)
		}
		if transcript.Text != expectedTexts[i] {
			t.Errorf("transcript %d: expected text %q, got %q", i, expectedTexts[i], transcript.Text)
		}
		if transcript.Language != "en" {
			t.Errorf("transcript %d: expected language 'en', got %q", i, transcript.Language)
		}
		if transcript.Confidence < 0.9 {
			t.Errorf("transcript %d: expected confidence >= 0.9, got %f", i, transcript.Confidence)
		}
	}
}

func TestStubRecognizer_LoadModel(t *testing.T) {
	t.Parallel()

	recognizer := NewStubRecognizer(nil)

	// Initially model not loaded
	health := recognizer.Health()
	if health.ModelLoaded {
		t.Error("expected model not loaded initially")
	}

	// Load model
	err := recognizer.LoadModel(ModelCPUBasic)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}

	// After loading
	health = recognizer.Health()
	if !health.ModelLoaded {
		t.Error("expected model loaded after LoadModel")
	}
}

func TestStubRecognizer_Health(t *testing.T) {
	t.Parallel()

	recognizer := NewStubRecognizer(nil)
	status := recognizer.Health()

	if !status.Healthy {
		t.Error("expected healthy status")
	}
}

func TestStubRecognizer_ContextCancellation(t *testing.T) {
	t.Parallel()

	config := &StubRecognizerConfig{
		ProcessingDelay: 500 * time.Millisecond, // Long delay
		DefaultLanguage: "en",
	}
	recognizer := NewStubRecognizer(config)

	ctx, cancel := context.WithCancel(context.Background())
	sessionID := "test-session"

	chunks := make(chan media.AudioChunk, 10)
	for i := 0; i < 10; i++ {
		chunks <- media.AudioChunk{Timestamp: time.Duration(i) * 100 * time.Millisecond}
	}
	close(chunks)

	transcripts, err := recognizer.Recognize(ctx, sessionID, chunks)
	if err != nil {
		t.Fatalf("Recognize failed: %v", err)
	}

	// Read one then cancel
	count := 0
	for range transcripts {
		count++
		if count >= 1 {
			cancel()
			break
		}
	}

	// Drain
	for range transcripts {
		count++
	}

	if count >= 10 {
		t.Errorf("expected fewer than 10 transcripts due to cancellation, got %d", count)
	}
}
