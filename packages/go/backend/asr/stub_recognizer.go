package asr

import (
	"context"
	"time"

	"streamlation/packages/backend/media"
)

// StubRecognizerConfig configures the stub recognizer behavior.
type StubRecognizerConfig struct {
	// ProcessingDelay simulates ASR processing time per chunk.
	ProcessingDelay time.Duration
	// DefaultLanguage is the language to report in transcripts.
	DefaultLanguage string
	// Transcripts maps chunk indices to predetermined text.
	// If nil, generates "Chunk N" text.
	Transcripts map[int]string
	// ErrorAfter causes an error after N transcripts (0 = no error).
	ErrorAfter int
}

// DefaultStubRecognizerConfig returns sensible defaults for testing.
func DefaultStubRecognizerConfig() *StubRecognizerConfig {
	return &StubRecognizerConfig{
		ProcessingDelay: 100 * time.Millisecond,
		DefaultLanguage: "en",
		Transcripts: map[int]string{
			0: "Hello world.",
			1: "This is a test.",
			2: "Welcome to Streamlation.",
			3: "Real-time translation.",
			4: "Thank you for watching.",
		},
	}
}

// StubRecognizer is a test implementation that returns deterministic transcripts.
type StubRecognizer struct {
	config      *StubRecognizerConfig
	modelLoaded bool
}

// NewStubRecognizer creates a new stub recognizer with the given config.
func NewStubRecognizer(config *StubRecognizerConfig) *StubRecognizer {
	if config == nil {
		config = DefaultStubRecognizerConfig()
	}
	return &StubRecognizer{config: config}
}

// LoadModel simulates loading an ASR model.
func (s *StubRecognizer) LoadModel(profile ModelProfile) error {
	s.modelLoaded = true
	return nil
}

// Recognize converts audio chunks to transcripts.
func (s *StubRecognizer) Recognize(ctx context.Context, sessionID string, chunks <-chan media.AudioChunk) (<-chan Transcript, error) {
	out := make(chan Transcript)

	go func() {
		defer close(out)

		chunkIndex := 0
		for chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Simulate processing delay
			if s.config.ProcessingDelay > 0 {
				select {
				case <-time.After(s.config.ProcessingDelay):
				case <-ctx.Done():
					return
				}
			}

			// Get text from config or generate default
			text := s.config.Transcripts[chunkIndex]
			if text == "" {
				text = "Chunk " + string(rune('0'+chunkIndex%10)) + " transcribed."
			}

			transcript := Transcript{
				SessionID:  sessionID,
				Text:       text,
				StartTime:  chunk.Timestamp,
				EndTime:    chunk.Timestamp + chunk.Duration,
				Confidence: 0.95,
				Language:   s.config.DefaultLanguage,
				Words: []Word{
					{Text: text, StartTime: chunk.Timestamp, EndTime: chunk.Timestamp + chunk.Duration},
				},
			}

			select {
			case out <- transcript:
				chunkIndex++
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Health returns the health status of the stub recognizer.
func (s *StubRecognizer) Health() HealthStatus {
	return HealthStatus{
		Healthy:     true,
		Message:     "stub recognizer ready",
		ModelLoaded: s.modelLoaded,
	}
}
