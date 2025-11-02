package ingestion

import (
	"context"
	"time"
)

// MediaChunk represents a normalized chunk of media data emitted by a stream source.
type MediaChunk struct {
	Sequence  int64
	Timestamp time.Time
	Duration  time.Duration
	Payload   []byte
	Metadata  map[string]string
}

// StreamMetrics captures aggregated statistics about a stream source.
type StreamMetrics struct {
	ReceivedChunks int64
	DroppedChunks  int64
	ErrorCount     int64
	ReconnectCount int64
	LastSequence   int64
}

// StreamSource exposes a streaming interface for ingestion adapters.
type StreamSource interface {
	Stream(ctx context.Context) (<-chan MediaChunk, <-chan error)
	Metrics() StreamMetrics
}
