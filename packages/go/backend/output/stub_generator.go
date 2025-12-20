package output

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"streamlation/packages/backend/translation"
)

// StubGenerator is a test implementation that generates deterministic subtitles.
type StubGenerator struct{}

// NewStubGenerator creates a new stub subtitle generator.
func NewStubGenerator() *StubGenerator {
	return &StubGenerator{}
}

// GenerateSRT creates SRT format subtitles from translations.
func (s *StubGenerator) GenerateSRT(ctx context.Context, sessionID string, translations <-chan translation.Translation) (io.Reader, error) {
	var buf bytes.Buffer
	index := 1

	for trans := range translations {
		select {
		case <-ctx.Done():
			return &buf, ctx.Err()
		default:
		}

		// Format: index\nstart --> end\ntext\n\n
		startTime := formatSRTTime(trans.StartTime)
		endTime := formatSRTTime(trans.EndTime)

		fmt.Fprintf(&buf, "%d\n", index)
		fmt.Fprintf(&buf, "%s --> %s\n", startTime, endTime)
		fmt.Fprintf(&buf, "%s\n\n", trans.TranslatedText)

		index++
	}

	return &buf, nil
}

// GenerateVTT creates WebVTT format subtitles from translations.
func (s *StubGenerator) GenerateVTT(ctx context.Context, sessionID string, translations <-chan translation.Translation) (io.Reader, error) {
	var buf bytes.Buffer

	// VTT header
	buf.WriteString("WEBVTT\n\n")

	index := 1
	for trans := range translations {
		select {
		case <-ctx.Done():
			return &buf, ctx.Err()
		default:
		}

		// Format: cue-id\nstart --> end\ntext\n\n
		startTime := formatVTTTime(trans.StartTime)
		endTime := formatVTTTime(trans.EndTime)

		fmt.Fprintf(&buf, "%d\n", index)
		fmt.Fprintf(&buf, "%s --> %s\n", startTime, endTime)
		fmt.Fprintf(&buf, "%s\n\n", trans.TranslatedText)

		index++
	}

	return &buf, nil
}

// StreamSubtitles provides real-time subtitle updates.
func (s *StubGenerator) StreamSubtitles(ctx context.Context, sessionID string, translations <-chan translation.Translation) (<-chan SubtitleEvent, error) {
	out := make(chan SubtitleEvent)

	go func() {
		defer close(out)

		index := 0
		for trans := range translations {
			select {
			case <-ctx.Done():
				return
			default:
			}

			event := SubtitleEvent{
				Type:      "add",
				Index:     index,
				StartTime: trans.StartTime,
				EndTime:   trans.EndTime,
				Text:      trans.TranslatedText,
				SessionID: sessionID,
			}

			select {
			case out <- event:
				index++
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Health returns the health status of the stub generator.
func (s *StubGenerator) Health() HealthStatus {
	return HealthStatus{
		Healthy: true,
		Message: "stub generator ready",
	}
}

// formatSRTTime formats a duration as SRT timestamp (HH:MM:SS,mmm).
func formatSRTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

// formatVTTTime formats a duration as VTT timestamp (HH:MM:SS.mmm).
func formatVTTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}
