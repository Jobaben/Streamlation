package ingestion

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileConfig configures the file-backed stream source.
type FileConfig struct {
	// Path is the local filesystem path to the media file.
	Path string
	// ChunkSize controls the number of bytes per emitted chunk. Defaults to 64 KiB when zero.
	ChunkSize int
	// ChunkDuration approximates the playback duration per chunk.
	ChunkDuration time.Duration
	// BufferSize controls the channel buffer size for emitted chunks. Defaults to 4 when zero.
	BufferSize int
	// EmitInterval throttles chunk emission to simulate realtime playback. Disabled when zero.
	EmitInterval time.Duration
	// Metadata carries additional key/value metadata to attach to each chunk.
	Metadata map[string]string
}

type fileStreamSource struct {
	cfg     FileConfig
	metrics StreamMetrics
	mu      sync.Mutex
}

// NewFileStreamSource constructs a StreamSource that replays a local file as discrete media chunks.
func NewFileStreamSource(cfg FileConfig) (StreamSource, error) {
	if cfg.Path == "" {
		return nil, errors.New("file path is required")
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 64 * 1024
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 4
	}
	if cfg.ChunkDuration < 0 {
		return nil, errors.New("chunk duration cannot be negative")
	}
	// Normalize the path for the current platform to improve logging parity.
	cfg.Path = filepath.Clean(filepath.FromSlash(cfg.Path))

	return &fileStreamSource{cfg: cfg}, nil
}

func (f *fileStreamSource) Stream(ctx context.Context) (<-chan MediaChunk, <-chan error) {
	chunks := make(chan MediaChunk, f.cfg.BufferSize)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		file, err := os.Open(f.cfg.Path)
		if err != nil {
			errs <- err
			f.recordError()
			return
		}
		defer func() { _ = file.Close() }()

		reader := bufio.NewReader(file)
		sequence := int64(0)
		start := time.Now().UTC()
		buf := make([]byte, f.cfg.ChunkSize)

		for {
			if ctx.Err() != nil {
				return
			}

			n, readErr := io.ReadFull(reader, buf)
			if errors.Is(readErr, io.ErrUnexpectedEOF) || errors.Is(readErr, io.EOF) {
				if n == 0 {
					return
				}
			} else if readErr != nil {
				errs <- readErr
				f.recordError()
				return
			}

			payload := make([]byte, n)
			copy(payload, buf[:n])

			chunk := MediaChunk{
				Sequence:  sequence,
				Timestamp: start.Add(time.Duration(sequence) * f.cfg.ChunkDuration),
				Duration:  f.cfg.ChunkDuration,
				Payload:   payload,
			}
			if len(f.cfg.Metadata) > 0 {
				chunk.Metadata = make(map[string]string, len(f.cfg.Metadata))
				for k, v := range f.cfg.Metadata {
					chunk.Metadata[k] = v
				}
			}

			select {
			case <-ctx.Done():
				return
			case chunks <- chunk:
				f.recordChunk(sequence)
			}

			sequence++

			if readErr != nil {
				// We hit EOF after emitting the final chunk above.
				return
			}

			if f.cfg.EmitInterval > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(f.cfg.EmitInterval):
				}
			}
		}
	}()

	return chunks, errs
}

func (f *fileStreamSource) Metrics() StreamMetrics {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.metrics
}

func (f *fileStreamSource) recordChunk(sequence int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metrics.ReceivedChunks++
	f.metrics.LastSequence = sequence
}

func (f *fileStreamSource) recordError() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metrics.ErrorCount++
}

var _ StreamSource = (*fileStreamSource)(nil)
