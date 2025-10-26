package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	statuspkg "streamlation/packages/backend/status"
)

func TestSessionStatusHandler_WebSocketUpgradeAndEvent(t *testing.T) {
	subscriber := &stubStatusSubscriber{}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	handler := sessionStatusHandler(subscriber, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sessions/{id}/events", handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}
	defer conn.Close()

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	request := fmt.Sprintf("GET /sessions/session123/events HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", server.Listener.Addr().String(), key)
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("failed to write handshake request: %v", err)
	}

	reader := bufio.NewReader(conn)
	response, err := readUntilBlankLine(reader)
	if err != nil {
		t.Fatalf("failed to read handshake response: %v", err)
	}
	if !strings.Contains(response, "101 Switching Protocols") {
		t.Fatalf("expected switching protocols response, got %s", response)
	}
	if subscriber.lastSessionID != "session123" {
		t.Fatalf("expected subscriber to receive session ID, got %s", subscriber.lastSessionID)
	}

	event := statuspkg.SessionStatusEvent{SessionID: "session123", Stage: "ingestion", State: "queued", Timestamp: time.Now().UTC()}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}
	subscriber.stream.events <- event

	framePayload, opcode, err := readWebSocketFrame(reader)
	if err != nil {
		t.Fatalf("failed to read websocket frame: %v", err)
	}
	if opcode != 0x1 {
		t.Fatalf("expected text frame, got opcode %d", opcode)
	}
	if string(framePayload) != string(payload) {
		t.Fatalf("unexpected payload: %s", string(framePayload))
	}
}

func TestSessionStatusHandler_InvalidUpgrade(t *testing.T) {
	subscriber := &stubStatusSubscriber{}
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodGet, "/sessions/session123/events", nil)
	rr := httptest.NewRecorder()

	req.SetPathValue("id", "session123")
	handler := sessionStatusHandler(subscriber, logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

type stubStatusSubscriber struct {
	stream        *stubStatusStream
	lastSessionID string
}

func (s *stubStatusSubscriber) Subscribe(ctx context.Context, sessionID string) (statuspkg.StatusStream, error) {
	s.lastSessionID = sessionID
	s.stream = newStubStatusStream()
	return s.stream, nil
}

type stubStatusStream struct {
	events chan statuspkg.SessionStatusEvent
	errors chan error
}

func newStubStatusStream() *stubStatusStream {
	return &stubStatusStream{
		events: make(chan statuspkg.SessionStatusEvent, 4),
		errors: make(chan error, 1),
	}
}

func (s *stubStatusStream) Events() <-chan statuspkg.SessionStatusEvent { return s.events }
func (s *stubStatusStream) Errors() <-chan error                        { return s.errors }
func (s *stubStatusStream) Close() error {
	close(s.events)
	close(s.errors)
	return nil
}

func readUntilBlankLine(r *bufio.Reader) (string, error) {
	var builder strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		builder.WriteString(line)
		if line == "\r\n" {
			break
		}
	}
	return builder.String(), nil
}

func readWebSocketFrame(r *bufio.Reader) ([]byte, byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, 0, err
	}
	opcode := header[0] & 0x0F
	length := int(header[1] & 0x7F)
	switch length {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(r, extended); err != nil {
			return nil, 0, err
		}
		length = int(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(r, extended); err != nil {
			return nil, 0, err
		}
		length = int(binary.BigEndian.Uint64(extended))
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}
	return payload, opcode, nil
}
