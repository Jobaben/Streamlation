package queue

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRedisIngestionEnqueuer_ReusesConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	commands := make(chan []string, 2)
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("failed to accept connection: %v", err)
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)

		for i := 0; i < 2; i++ {
			args, err := readCommand(reader)
			if err != nil {
				t.Errorf("failed to read command: %v", err)
				return
			}
			commands <- args
			if _, err := writer.WriteString(":1\r\n"); err != nil {
				t.Errorf("failed to write response: %v", err)
				return
			}
			if err := writer.Flush(); err != nil {
				t.Errorf("failed to flush response: %v", err)
				return
			}
		}
	}()

	enqueuer, err := NewRedisIngestionEnqueuer(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to create enqueuer: %v", err)
	}
	t.Cleanup(func() { _ = enqueuer.Close() })

	if err := enqueuer.EnqueueIngestion(context.Background(), "session-1"); err != nil {
		t.Fatalf("enqueue returned error: %v", err)
	}
	if err := enqueuer.EnqueueIngestion(context.Background(), "session-2"); err != nil {
		t.Fatalf("second enqueue returned error: %v", err)
	}

	close(commands)
	<-done

	count := 0
	for args := range commands {
		count++
		if len(args) == 0 || args[0] != "LPUSH" {
			t.Fatalf("unexpected command: %v", args)
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 commands, got %d", count)
	}
}

func TestRedisIngestionConsumer_Pop(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	payload := `{"session_id":"abc"}`
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("failed to accept connection: %v", err)
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)

		// First BRPOP returns payload.
		args, err := readCommand(reader)
		if err != nil {
			t.Errorf("failed to read command: %v", err)
			return
		}
		if len(args) == 0 || args[0] != "BRPOP" {
			t.Errorf("unexpected first command: %v", args)
			return
		}
		response := fmt.Sprintf("*2\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n", len(IngestionQueueName), IngestionQueueName, len(payload), payload)
		if _, err := writer.WriteString(response); err != nil {
			t.Errorf("failed to write response: %v", err)
			return
		}
		if err := writer.Flush(); err != nil {
			t.Errorf("failed to flush response: %v", err)
			return
		}

		// Second BRPOP returns nil (timeout).
		args, err = readCommand(reader)
		if err != nil {
			t.Errorf("failed to read second command: %v", err)
			return
		}
		if len(args) == 0 || args[0] != "BRPOP" {
			t.Errorf("unexpected second command: %v", args)
			return
		}
		if _, err := writer.WriteString("*-1\r\n"); err != nil {
			t.Errorf("failed to write nil response: %v", err)
			return
		}
		if err := writer.Flush(); err != nil {
			t.Errorf("failed to flush nil response: %v", err)
			return
		}
	}()

	consumer, err := NewRedisIngestionConsumer(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to create consumer: %v", err)
	}
	t.Cleanup(func() { _ = consumer.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	job, err := consumer.Pop(ctx, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("expected job")
	}
	if job.SessionID != "abc" {
		t.Fatalf("unexpected session id: %s", job.SessionID)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel2()
	job, err = consumer.Pop(ctx2, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error when queue empty: %v", err)
	}
	if job != nil {
		t.Fatalf("expected nil job when queue empty, got %#v", job)
	}

	<-done
}

func readCommand(r *bufio.Reader) ([]string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if prefix != '*' {
		return nil, fmt.Errorf("unexpected prefix %q", prefix)
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != '$' {
			return nil, fmt.Errorf("unexpected bulk prefix %q", b)
		}
		bulkLenLine, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		bulkLen, err := strconv.Atoi(strings.TrimSpace(bulkLenLine))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, bulkLen+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:bulkLen]))
	}
	return args, nil
}
