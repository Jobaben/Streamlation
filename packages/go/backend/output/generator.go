package output

import (
	"context"
	"io"
	"time"

	"streamlation/packages/backend/translation"
)

// SubtitleEvent represents a real-time subtitle update.
type SubtitleEvent struct {
	// Type is the event type: "add", "update", or "remove".
	Type string `json:"type"`
	// Index is the subtitle index.
	Index int `json:"index"`
	// StartTime is when the subtitle should appear.
	StartTime time.Duration `json:"startTime"`
	// EndTime is when the subtitle should disappear.
	EndTime time.Duration `json:"endTime"`
	// Text is the subtitle content.
	Text string `json:"text"`
	// SessionID identifies the translation session.
	SessionID string `json:"sessionId"`
}

// SubtitleFormat specifies the output format.
type SubtitleFormat string

const (
	FormatSRT SubtitleFormat = "srt"
	FormatVTT SubtitleFormat = "vtt"
)

// HealthStatus represents the health of a component.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// SubtitleGenerator creates subtitle files and streams from translations.
type SubtitleGenerator interface {
	// GenerateSRT creates SRT format subtitles from translations.
	GenerateSRT(ctx context.Context, sessionID string, translations <-chan translation.Translation) (io.Reader, error)

	// GenerateVTT creates WebVTT format subtitles from translations.
	GenerateVTT(ctx context.Context, sessionID string, translations <-chan translation.Translation) (io.Reader, error)

	// StreamSubtitles provides real-time subtitle updates.
	StreamSubtitles(ctx context.Context, sessionID string, translations <-chan translation.Translation) (<-chan SubtitleEvent, error)

	// Health returns the current health status of the generator.
	Health() HealthStatus
}
