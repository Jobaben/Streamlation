package translation

import (
	"context"
	"testing"
	"time"

	"streamlation/packages/backend/asr"
)

func TestStubTranslator_Translate(t *testing.T) {
	t.Parallel()

	translator := NewStubTranslator(nil)
	ctx := context.Background()

	// Test known translation
	result, err := translator.Translate(ctx, "Hello world.", "en", "es")
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	if result.TranslatedText != "Hola mundo." {
		t.Errorf("expected 'Hola mundo.', got %q", result.TranslatedText)
	}
	if result.SourceLang != "en" {
		t.Errorf("expected source lang 'en', got %q", result.SourceLang)
	}
	if result.TargetLang != "es" {
		t.Errorf("expected target lang 'es', got %q", result.TargetLang)
	}
}

func TestStubTranslator_TranslateUnknown(t *testing.T) {
	t.Parallel()

	translator := NewStubTranslator(nil)
	ctx := context.Background()

	// Test unknown translation (should use default prefix)
	result, err := translator.Translate(ctx, "Unknown text.", "en", "de")
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	expected := "[de] Unknown text."
	if result.TranslatedText != expected {
		t.Errorf("expected %q, got %q", expected, result.TranslatedText)
	}
}

func TestStubTranslator_TranslateStream(t *testing.T) {
	t.Parallel()

	translator := NewStubTranslator(nil)
	ctx := context.Background()
	sessionID := "test-session"

	// Create transcript stream
	transcripts := make(chan asr.Transcript, 3)
	transcripts <- asr.Transcript{
		Text:      "Hello world.",
		Language:  "en",
		StartTime: 0,
		EndTime:   100 * time.Millisecond,
	}
	transcripts <- asr.Transcript{
		Text:      "This is a test.",
		Language:  "en",
		StartTime: 100 * time.Millisecond,
		EndTime:   200 * time.Millisecond,
	}
	transcripts <- asr.Transcript{
		Text:      "Welcome to Streamlation.",
		Language:  "en",
		StartTime: 200 * time.Millisecond,
		EndTime:   300 * time.Millisecond,
	}
	close(transcripts)

	translations, err := translator.TranslateStream(ctx, sessionID, transcripts, "es")
	if err != nil {
		t.Fatalf("TranslateStream failed: %v", err)
	}

	var received []Translation
	for trans := range translations {
		received = append(received, trans)
	}

	if len(received) != 3 {
		t.Errorf("expected 3 translations, got %d", len(received))
	}

	expectedTexts := []string{
		"Hola mundo.",
		"Esto es una prueba.",
		"Bienvenido a Streamlation.",
	}

	for i, trans := range received {
		if trans.TranslatedText != expectedTexts[i] {
			t.Errorf("translation %d: expected %q, got %q", i, expectedTexts[i], trans.TranslatedText)
		}
		if trans.SessionID != sessionID {
			t.Errorf("translation %d: expected session ID %s, got %s", i, sessionID, trans.SessionID)
		}
	}
}

func TestStubTranslator_SupportedLanguages(t *testing.T) {
	t.Parallel()

	translator := NewStubTranslator(nil)
	pairs := translator.SupportedLanguages()

	if len(pairs) == 0 {
		t.Error("expected non-empty language pairs")
	}

	// Check for en->es pair
	found := false
	for _, pair := range pairs {
		if pair.Source == "en" && pair.Target == "es" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected en->es language pair")
	}
}

func TestStubTranslator_Health(t *testing.T) {
	t.Parallel()

	translator := NewStubTranslator(nil)
	status := translator.Health()

	if !status.Healthy {
		t.Error("expected healthy status")
	}
}

func TestStubTranslator_ContextCancellation(t *testing.T) {
	t.Parallel()

	config := &StubTranslatorConfig{
		ProcessingDelay: 200 * time.Millisecond,
	}
	translator := NewStubTranslator(config)

	ctx, cancel := context.WithCancel(context.Background())

	// Test single translation cancellation
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := translator.Translate(ctx, "Hello", "en", "es")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}
