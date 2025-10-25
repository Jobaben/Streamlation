package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
)

func TestBuildRESPCommand(t *testing.T) {
	cmd := buildRESPCommand([]string{"PING"})
	expected := "*1\r\n$4\r\nPING\r\n"
	if string(cmd) != expected {
		t.Fatalf("unexpected RESP command: %q", string(cmd))
	}
}

func TestReadSimpleIntegerReply(t *testing.T) {
	data := []byte(":1\r\n")
	reader := bufio.NewReader(bytes.NewReader(data))
	if err := readSimpleIntegerReply(reader); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedisIngestionEnqueuer_UsesQueue(t *testing.T) {
	// Start a lightweight TCP listener that mimics a Redis integer reply.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer func() {
		_ = ln.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("failed to accept connection: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		buf := make([]byte, 1024)
		n, readErr := conn.Read(buf)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			t.Errorf("failed to read command: %v", readErr)
			return
		}
		received := string(buf[:n])
		if !bytes.Contains([]byte(received), []byte(ingestionQueueName)) {
			t.Errorf("expected queue name in command, got %q", received)
		}
		if _, err := conn.Write([]byte(":1\r\n")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}()

	enqueuer := NewRedisIngestionEnqueuer(ln.Addr().String())
	if err := enqueuer.EnqueueIngestion(context.Background(), "session-1"); err != nil {
		t.Fatalf("enqueue returned error: %v", err)
	}

	<-done
}
