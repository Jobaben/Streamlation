package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStreamSourceStreamsChunks(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.bin")
	if err := os.WriteFile(filePath, []byte("abcdefghijklmnopqrstuvwxyz"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	source, err := NewFileStreamSource(FileConfig{Path: filePath, ChunkSize: 8, ChunkDuration: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewFileStreamSource returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	chunks, errs := source.Stream(ctx)

	var received [][]byte
	for chunk := range chunks {
		received = append(received, chunk.Payload)
	}

	select {
	case err, ok := <-errs:
		if ok && err != nil {
			t.Fatalf("unexpected error from stream: %v", err)
		}
	default:
	}

	if len(received) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(received))
	}

	expected := []string{"abcdefgh", "ijklmnop", "qrstuvwx", "yz"}
	for i, payload := range received {
		if string(payload) != expected[i] {
			t.Fatalf("chunk %d mismatch: expected %q got %q", i, expected[i], string(payload))
		}
	}

	metrics := source.Metrics()
	if metrics.ReceivedChunks != int64(len(received)) {
		t.Fatalf("expected metrics.ReceivedChunks=%d got %d", len(received), metrics.ReceivedChunks)
	}
	if metrics.LastSequence != int64(len(received)-1) {
		t.Fatalf("expected metrics.LastSequence=%d got %d", len(received)-1, metrics.LastSequence)
	}
}

func TestFileStreamSourceHonoursCancellation(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.bin")
	data := make([]byte, 1024)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	source, err := NewFileStreamSource(FileConfig{Path: filePath, ChunkSize: 64})
	if err != nil {
		t.Fatalf("NewFileStreamSource returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	chunks, errs := source.Stream(ctx)

	// Consume the first chunk then cancel.
	select {
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first chunk")
	case <-errs:
		t.Fatal("unexpected error from stream")
	case <-ctx.Done():
		t.Fatal("context cancelled prematurely")
	case <-chunks:
		cancel()
	}

	// Additional reads should drain without errors.
	for range chunks {
	}

	select {
	case err, ok := <-errs:
		if ok && err != nil {
			t.Fatalf("unexpected error after cancellation: %v", err)
		}
	default:
	}
}

func TestNewFileStreamSourceValidatesConfig(t *testing.T) {
	if _, err := NewFileStreamSource(FileConfig{}); err == nil {
		t.Fatal("expected error for missing path")
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	if _, err := NewFileStreamSource(FileConfig{Path: filePath, ChunkDuration: -time.Second}); err == nil {
		t.Fatal("expected error for negative chunk duration")
	}
}
