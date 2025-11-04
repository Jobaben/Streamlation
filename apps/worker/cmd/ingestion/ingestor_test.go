package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sessionpkg "streamlation/packages/backend/session"
)

func TestStreamIngestorIngestsHLS(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/stream/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:1.5,\nseg-0.ts\n#EXTINF:1.5,\nseg-1.ts\n"))
	})
	handler.HandleFunc("/stream/seg-0.ts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("segment-0"))
	})
	handler.HandleFunc("/stream/seg-1.ts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("segment-1"))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	ingestor := newStreamIngestor(newTestLogger(t))
	ingestor.httpClient = server.Client()
	ingestor.sampleWindow = 150 * time.Millisecond

	session := sessionpkg.TranslationSession{
		ID: "session-hls",
		Source: sessionpkg.TranslationSource{
			Type: "hls",
			URI:  server.URL + "/stream/index.m3u8",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := ingestor.Ingest(ctx, session); err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
}

func TestStreamIngestorIngestsRTMP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		_ = ln.Close()
	}()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		const handshake = "STRM1"
		buf := make([]byte, len(handshake))
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		if _, err := conn.Write([]byte(handshake)); err != nil {
			return
		}

		frames := [][]byte{[]byte("hello"), []byte("world")}
		for _, frame := range frames {
			header := make([]byte, 4)
			binary.BigEndian.PutUint32(header, uint32(len(frame)))
			if _, err := conn.Write(header); err != nil {
				return
			}
			if _, err := conn.Write(frame); err != nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	ingestor := newStreamIngestor(newTestLogger(t))
	ingestor.dialer.Timeout = 100 * time.Millisecond
	ingestor.sampleWindow = 150 * time.Millisecond

	session := sessionpkg.TranslationSession{
		ID: "session-rtmp",
		Source: sessionpkg.TranslationSource{
			Type: "rtmp",
			URI:  fmt.Sprintf("rtmp://%s/live/stream", ln.Addr().String()),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := ingestor.Ingest(ctx, session); err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
}

func TestStreamIngestorUnsupportedSource(t *testing.T) {
	ingestor := newStreamIngestor(newTestLogger(t))
	session := sessionpkg.TranslationSession{
		ID:     "session-unsupported",
		Source: sessionpkg.TranslationSource{Type: "dash", URI: "http://example.com"},
	}
	err := ingestor.Ingest(context.Background(), session)
	if err == nil {
		t.Fatal("expected error for unsupported source")
	}
}
