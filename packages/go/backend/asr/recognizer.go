package asr

import (
	"context"
	"time"

	"streamlation/packages/backend/media"
)

// Word represents a single word with timing information for alignment.
type Word struct {
	Text      string        `json:"text"`
	StartTime time.Duration `json:"startTime"`
	EndTime   time.Duration `json:"endTime"`
}

// Transcript represents a transcribed segment of audio.
type Transcript struct {
	// SessionID identifies the translation session.
	SessionID string `json:"sessionId"`
	// Text is the full transcribed text.
	Text string `json:"text"`
	// StartTime is when this segment begins in the source.
	StartTime time.Duration `json:"startTime"`
	// EndTime is when this segment ends in the source.
	EndTime time.Duration `json:"endTime"`
	// Confidence is the ASR confidence score (0.0 - 1.0).
	Confidence float64 `json:"confidence"`
	// Language is the detected source language (ISO 639-1 code).
	Language string `json:"language"`
	// Words contains word-level timing for subtitle alignment.
	Words []Word `json:"words,omitempty"`
}

// ModelProfile specifies the ASR model configuration.
type ModelProfile string

const (
	ModelCPUBasic    ModelProfile = "cpu-basic"
	ModelCPUAdvanced ModelProfile = "cpu-advanced"
	ModelGPU         ModelProfile = "gpu-accelerated"
)

// HealthStatus represents the health of a component.
type HealthStatus struct {
	Healthy     bool   `json:"healthy"`
	Message     string `json:"message,omitempty"`
	ModelLoaded bool   `json:"modelLoaded"`
}

// Recognizer transcribes audio chunks to text using speech recognition.
type Recognizer interface {
	// Recognize processes audio chunks and returns transcriptions.
	// The returned channel will be closed when processing completes.
	Recognize(ctx context.Context, sessionID string, chunks <-chan media.AudioChunk) (<-chan Transcript, error)

	// LoadModel loads a specific model profile. Must be called before Recognize.
	LoadModel(profile ModelProfile) error

	// Health returns the current health status of the recognizer.
	Health() HealthStatus
}
