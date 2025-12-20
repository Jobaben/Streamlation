package output

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"streamlation/packages/backend/translation"
)

func TestStubGenerator_GenerateSRT(t *testing.T) {
	t.Parallel()

	generator := NewStubGenerator()
	ctx := context.Background()
	sessionID := "test-session"

	// Create translation stream
	translations := make(chan translation.Translation, 2)
	translations <- translation.Translation{
		TranslatedText: "Hola mundo.",
		StartTime:      0,
		EndTime:        2 * time.Second,
	}
	translations <- translation.Translation{
		TranslatedText: "Esto es una prueba.",
		StartTime:      2 * time.Second,
		EndTime:        4 * time.Second,
	}
	close(translations)

	reader, err := generator.GenerateSRT(ctx, sessionID, translations)
	if err != nil {
		t.Fatalf("GenerateSRT failed: %v", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	srt := string(content)

	// Verify SRT format
	if !strings.Contains(srt, "1\n") {
		t.Error("expected subtitle index 1")
	}
	if !strings.Contains(srt, "00:00:00,000 --> 00:00:02,000") {
		t.Error("expected correct timestamp for first subtitle")
	}
	if !strings.Contains(srt, "Hola mundo.") {
		t.Error("expected first subtitle text")
	}
	if !strings.Contains(srt, "2\n") {
		t.Error("expected subtitle index 2")
	}
	if !strings.Contains(srt, "Esto es una prueba.") {
		t.Error("expected second subtitle text")
	}
}

func TestStubGenerator_GenerateVTT(t *testing.T) {
	t.Parallel()

	generator := NewStubGenerator()
	ctx := context.Background()
	sessionID := "test-session"

	translations := make(chan translation.Translation, 1)
	translations <- translation.Translation{
		TranslatedText: "Test subtitle.",
		StartTime:      0,
		EndTime:        1 * time.Second,
	}
	close(translations)

	reader, err := generator.GenerateVTT(ctx, sessionID, translations)
	if err != nil {
		t.Fatalf("GenerateVTT failed: %v", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	vtt := string(content)

	// Verify VTT format
	if !strings.HasPrefix(vtt, "WEBVTT\n") {
		t.Error("expected WEBVTT header")
	}
	if !strings.Contains(vtt, "00:00:00.000 --> 00:00:01.000") {
		t.Error("expected correct VTT timestamp")
	}
	if !strings.Contains(vtt, "Test subtitle.") {
		t.Error("expected subtitle text")
	}
}

func TestStubGenerator_StreamSubtitles(t *testing.T) {
	t.Parallel()

	generator := NewStubGenerator()
	ctx := context.Background()
	sessionID := "test-session"

	translations := make(chan translation.Translation, 3)
	translations <- translation.Translation{
		TranslatedText: "First.",
		StartTime:      0,
		EndTime:        1 * time.Second,
	}
	translations <- translation.Translation{
		TranslatedText: "Second.",
		StartTime:      1 * time.Second,
		EndTime:        2 * time.Second,
	}
	translations <- translation.Translation{
		TranslatedText: "Third.",
		StartTime:      2 * time.Second,
		EndTime:        3 * time.Second,
	}
	close(translations)

	events, err := generator.StreamSubtitles(ctx, sessionID, translations)
	if err != nil {
		t.Fatalf("StreamSubtitles failed: %v", err)
	}

	var received []SubtitleEvent
	for event := range events {
		received = append(received, event)
	}

	if len(received) != 3 {
		t.Errorf("expected 3 events, got %d", len(received))
	}

	for i, event := range received {
		if event.Type != "add" {
			t.Errorf("event %d: expected type 'add', got %q", i, event.Type)
		}
		if event.Index != i {
			t.Errorf("event %d: expected index %d, got %d", i, i, event.Index)
		}
		if event.SessionID != sessionID {
			t.Errorf("event %d: expected session ID %s, got %s", i, sessionID, event.SessionID)
		}
	}
}

func TestStubGenerator_Health(t *testing.T) {
	t.Parallel()

	generator := NewStubGenerator()
	status := generator.Health()

	if !status.Healthy {
		t.Error("expected healthy status")
	}
}

func TestFormatSRTTime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "00:00:00,000"},
		{1 * time.Second, "00:00:01,000"},
		{1*time.Minute + 30*time.Second + 500*time.Millisecond, "00:01:30,500"},
		{1*time.Hour + 2*time.Minute + 3*time.Second + 456*time.Millisecond, "01:02:03,456"},
	}

	for _, tt := range tests {
		result := formatSRTTime(tt.duration)
		if result != tt.expected {
			t.Errorf("formatSRTTime(%v): expected %q, got %q", tt.duration, tt.expected, result)
		}
	}
}

func TestFormatVTTTime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "00:00:00.000"},
		{1 * time.Second, "00:00:01.000"},
		{1*time.Minute + 30*time.Second + 500*time.Millisecond, "00:01:30.500"},
		{1*time.Hour + 2*time.Minute + 3*time.Second + 456*time.Millisecond, "01:02:03.456"},
	}

	for _, tt := range tests {
		result := formatVTTTime(tt.duration)
		if result != tt.expected {
			t.Errorf("formatVTTTime(%v): expected %q, got %q", tt.duration, tt.expected, result)
		}
	}
}
