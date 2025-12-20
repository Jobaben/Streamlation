package tts

import (
	"context"
	"testing"
	"time"

	"streamlation/packages/backend/translation"
)

func TestStubSynthesizer_Synthesize(t *testing.T) {
	t.Parallel()

	synthesizer := NewStubSynthesizer(nil)
	ctx := context.Background()

	voice := VoiceProfile{ID: "en-us-1", Language: "en", Gender: "female"}
	segment, err := synthesizer.Synthesize(ctx, "Hello world", voice)
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}

	if segment.SampleRate != 22050 {
		t.Errorf("expected sample rate 22050, got %d", segment.SampleRate)
	}
	if len(segment.PCMData) == 0 {
		t.Error("expected non-empty PCM data")
	}
	if segment.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestStubSynthesizer_SynthesizeStream(t *testing.T) {
	t.Parallel()

	synthesizer := NewStubSynthesizer(nil)
	ctx := context.Background()
	sessionID := "test-session"

	translations := make(chan translation.Translation, 2)
	translations <- translation.Translation{
		TranslatedText: "Hello world.",
		StartTime:      0,
		EndTime:        1 * time.Second,
	}
	translations <- translation.Translation{
		TranslatedText: "This is a test.",
		StartTime:      1 * time.Second,
		EndTime:        2 * time.Second,
	}
	close(translations)

	voice := VoiceProfile{ID: "en-us-1", Language: "en", Gender: "female"}
	segments, err := synthesizer.SynthesizeStream(ctx, sessionID, translations, voice)
	if err != nil {
		t.Fatalf("SynthesizeStream failed: %v", err)
	}

	var received []AudioSegment
	for segment := range segments {
		received = append(received, segment)
	}

	if len(received) != 2 {
		t.Errorf("expected 2 segments, got %d", len(received))
	}

	for i, segment := range received {
		if segment.SessionID != sessionID {
			t.Errorf("segment %d: expected session ID %s, got %s", i, sessionID, segment.SessionID)
		}
		if len(segment.PCMData) == 0 {
			t.Errorf("segment %d: expected non-empty PCM data", i)
		}
	}
}

func TestStubSynthesizer_AvailableVoices(t *testing.T) {
	t.Parallel()

	synthesizer := NewStubSynthesizer(nil)

	// Test English voices
	voices := synthesizer.AvailableVoices("en")
	if len(voices) == 0 {
		t.Error("expected non-empty voices for English")
	}

	// Verify voice properties
	foundFemale := false
	foundMale := false
	for _, voice := range voices {
		if voice.Language != "en" {
			t.Errorf("expected language 'en', got %q", voice.Language)
		}
		if voice.Gender == "female" {
			foundFemale = true
		}
		if voice.Gender == "male" {
			foundMale = true
		}
	}

	if !foundFemale {
		t.Error("expected female voice for English")
	}
	if !foundMale {
		t.Error("expected male voice for English")
	}

	// Test Spanish voices
	voices = synthesizer.AvailableVoices("es")
	if len(voices) == 0 {
		t.Error("expected non-empty voices for Spanish")
	}

	// Test unsupported language
	voices = synthesizer.AvailableVoices("xx")
	if len(voices) != 0 {
		t.Error("expected empty voices for unsupported language")
	}
}

func TestStubSynthesizer_Health(t *testing.T) {
	t.Parallel()

	synthesizer := NewStubSynthesizer(nil)
	status := synthesizer.Health()

	if !status.Healthy {
		t.Error("expected healthy status")
	}
}

func TestStubSynthesizer_ContextCancellation(t *testing.T) {
	t.Parallel()

	config := &StubSynthesizerConfig{
		ProcessingDelay: 500 * time.Millisecond,
		SampleRate:      22050,
	}
	synthesizer := NewStubSynthesizer(config)

	ctx, cancel := context.WithCancel(context.Background())
	voice := VoiceProfile{ID: "en-us-1", Language: "en"}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := synthesizer.Synthesize(ctx, "Hello", voice)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}
