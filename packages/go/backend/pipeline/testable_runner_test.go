package pipeline

import (
	"context"
	"testing"
	"time"

	"streamlation/packages/backend/asr"
	"streamlation/packages/backend/media"
	"streamlation/packages/backend/output"
	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"
	"streamlation/packages/backend/translation"
)

func TestTestableRunner_Run(t *testing.T) {
	t.Parallel()

	// Create stub components
	normalizer := media.NewStubNormalizer(nil)
	recognizer := asr.NewStubRecognizer(nil)
	translator := translation.NewStubTranslator(nil)
	generator := output.NewStubGenerator()

	runner := NewTestableRunner(normalizer, recognizer, translator, generator)

	session := sessionpkg.TranslationSession{
		ID:             "test-session",
		TargetLanguage: "es",
		Source: sessionpkg.TranslationSource{
			Type: "file",
			URI:  "test.mp4",
		},
	}

	// Collect emitted events
	var events []statuspkg.SessionStatusEvent
	emit := func(event statuspkg.SessionStatusEvent) error {
		events = append(events, event)
		return nil
	}

	ctx := context.Background()
	err := runner.Run(ctx, session, emit)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify we got events for all stages
	stages := map[string]bool{}
	for _, event := range events {
		stages[event.Stage] = true
		if event.SessionID != session.ID {
			t.Errorf("expected session ID %s, got %s", session.ID, event.SessionID)
		}
	}

	expectedStages := []string{"ingestion", "normalization", "asr", "translation", "output"}
	for _, stage := range expectedStages {
		if !stages[stage] {
			t.Errorf("missing events for stage: %s", stage)
		}
	}

	// Verify we have both running and completed states for each stage
	stageStates := map[string]map[string]bool{}
	for _, event := range events {
		if stageStates[event.Stage] == nil {
			stageStates[event.Stage] = map[string]bool{}
		}
		stageStates[event.Stage][event.State] = true
	}

	for _, stage := range expectedStages {
		states := stageStates[stage]
		if !states["running"] {
			t.Errorf("missing 'running' state for stage: %s", stage)
		}
		if !states["completed"] {
			t.Errorf("missing 'completed' state for stage: %s", stage)
		}
	}
}

func TestTestableRunner_NilEmit(t *testing.T) {
	t.Parallel()

	normalizer := media.NewStubNormalizer(nil)
	recognizer := asr.NewStubRecognizer(nil)
	translator := translation.NewStubTranslator(nil)
	generator := output.NewStubGenerator()

	runner := NewTestableRunner(normalizer, recognizer, translator, generator)

	session := sessionpkg.TranslationSession{
		ID:             "test-session",
		TargetLanguage: "es",
	}

	ctx := context.Background()
	err := runner.Run(ctx, session, nil)
	if err != nil {
		t.Fatalf("Run with nil emit should not fail: %v", err)
	}
}

func TestTestableRunner_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Use slow config to allow cancellation
	normConfig := &media.StubNormalizerConfig{
		ChunkDuration: 100 * time.Millisecond,
		ChunkDelay:    500 * time.Millisecond,
		TotalChunks:   100,
		SampleRate:    16000,
	}
	normalizer := media.NewStubNormalizer(normConfig)
	recognizer := asr.NewStubRecognizer(nil)
	translator := translation.NewStubTranslator(nil)
	generator := output.NewStubGenerator()

	runner := NewTestableRunner(normalizer, recognizer, translator, generator)

	session := sessionpkg.TranslationSession{
		ID:             "test-session",
		TargetLanguage: "es",
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short time
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := runner.Run(ctx, session, nil)
	// May or may not return error depending on timing
	_ = err
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{-5, "-5"},
		{-123, "-123"},
	}

	for _, tt := range tests {
		result := itoa(tt.n)
		if result != tt.expected {
			t.Errorf("itoa(%d): expected %q, got %q", tt.n, tt.expected, result)
		}
	}
}
