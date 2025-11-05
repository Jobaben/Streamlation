package ingestion

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestRTMPStreamSourceStreamsFrames(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	frames := [][]byte{[]byte("alpha"), []byte("beta"), []byte("gamma")}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		handshake := make([]byte, len(handshakeMagic))
		if _, err := io.ReadFull(conn, handshake); err != nil {
			t.Logf("failed to read handshake: %v", err)
			return
		}
		if _, err := conn.Write([]byte(handshakeMagic)); err != nil {
			t.Logf("failed to write handshake: %v", err)
			return
		}

		for _, frame := range frames {
			header := make([]byte, 4)
			binary.BigEndian.PutUint32(header, uint32(len(frame)))
			if _, err := conn.Write(header); err != nil {
				t.Logf("write header: %v", err)
				return
			}
			if _, err := conn.Write(frame); err != nil {
				t.Logf("write payload: %v", err)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond)
	}()

	source, err := NewRTMPStreamSource(RTMPConfig{
		URL:            "rtmp://" + ln.Addr().String() + "/live/stream",
		BufferSize:     4,
		ReconnectDelay: 10 * time.Millisecond,
		ReadTimeout:    200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewRTMPStreamSource: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	chunks, errs := source.Stream(ctx)

	var received [][]byte
collect:
	for {
		select {
		case <-ctx.Done():
			break collect
		case err := <-errs:
			if err != nil {
				t.Fatalf("rtmp stream error: %v", err)
			}
		case chunk, ok := <-chunks:
			if !ok {
				break collect
			}
			received = append(received, append([]byte(nil), chunk.Payload...))
			if len(received) == len(frames) {
				break collect
			}
		}
	}

	if len(received) != len(frames) {
		t.Fatalf("expected %d frames, got %d", len(frames), len(received))
	}
	for i := range frames {
		if string(received[i]) != string(frames[i]) {
			t.Fatalf("frame %d mismatch: got %q want %q", i, string(received[i]), string(frames[i]))
		}
	}

	metrics := source.Metrics()
	if metrics.ReceivedChunks != int64(len(frames)) {
		t.Fatalf("metrics.ReceivedChunks = %d, want %d", metrics.ReceivedChunks, len(frames))
	}
}
