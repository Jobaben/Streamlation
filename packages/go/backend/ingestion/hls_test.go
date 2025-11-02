package ingestion

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHLSStreamSourceStreamsSegments(t *testing.T) {
	t.Helper()

	var (
		mu       sync.Mutex
		sequence = 0
		segments = [][]byte{
			[]byte("segment-0"),
			[]byte("segment-1"),
			[]byte("segment-2"),
		}
	)

	handler := http.NewServeMux()
	handler.HandleFunc("/stream/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		upto := sequence + 2
		if upto > len(segments) {
			upto = len(segments)
		}
		_, _ = w.Write([]byte("#EXTM3U\n"))
		for i := 0; i < upto; i++ {
			_, _ = w.Write([]byte("#EXTINF:4.0,\n"))
			_, _ = w.Write([]byte(fmt.Sprintf("seg-%d.ts\n", i)))
		}
		if sequence < len(segments) {
			sequence++
		}
	})
	handler.HandleFunc("/stream/seg-0.ts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(segments[0])
	})
	handler.HandleFunc("/stream/seg-1.ts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(segments[1])
	})
	handler.HandleFunc("/stream/seg-2.ts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(segments[2])
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	source, err := NewHLSStreamSource(HLSConfig{
		PlaylistURL:  server.URL + "/stream/index.m3u8",
		Client:       server.Client(),
		PollInterval: 20 * time.Millisecond,
		BufferSize:   4,
	})
	if err != nil {
		t.Fatalf("NewHLSStreamSource error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	chunks, errs := source.Stream(ctx)

	var received []MediaChunk
	deadline := time.After(400 * time.Millisecond)
loop:
	for {
		select {
		case <-deadline:
			break loop
		case err := <-errs:
			if err != nil {
				t.Fatalf("stream returned error: %v", err)
			}
		case chunk, ok := <-chunks:
			if !ok {
				break loop
			}
			received = append(received, chunk)
			if len(received) == len(segments) {
				break loop
			}
		}
	}

	if len(received) != len(segments) {
		t.Fatalf("expected %d segments, got %d", len(segments), len(received))
	}

	for i, chunk := range received {
		expected := segments[i]
		if string(chunk.Payload) != string(expected) {
			t.Fatalf("segment %d payload mismatch: got %q want %q", i, string(chunk.Payload), string(expected))
		}
		if chunk.Duration != 4*time.Second {
			t.Fatalf("segment %d duration = %v, want 4s", i, chunk.Duration)
		}
		if chunk.Metadata["uri"] != fmt.Sprintf("seg-%d.ts", i) {
			t.Fatalf("segment %d uri mismatch: %s", i, chunk.Metadata["uri"])
		}
	}

	metrics := source.Metrics()
	if metrics.ReceivedChunks != int64(len(segments)) {
		t.Fatalf("metrics.ReceivedChunks = %d, want %d", metrics.ReceivedChunks, len(segments))
	}
}
