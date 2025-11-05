package status

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

func TestChannelName(t *testing.T) {
	got := channelName("session123")
	if got != "streamlation:session:session123:status" {
		t.Fatalf("unexpected channel name: %s", got)
	}
}

func TestRedisStatusPublisherAndSubscriber(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	channel := channelName("session123")
	ready := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		subConn, err := ln.Accept()
		if err != nil {
			t.Errorf("failed to accept subscriber: %v", err)
			return
		}
		defer subConn.Close()
		subReader := bufio.NewReader(subConn)
		subWriter := bufio.NewWriter(subConn)

		args, err := readCommand(subReader)
		if err != nil {
			t.Errorf("failed to read subscribe command: %v", err)
			return
		}
		if len(args) < 2 || strings.ToUpper(args[0]) != "SUBSCRIBE" {
			t.Errorf("unexpected subscribe command: %v", args)
			return
		}
		ack := fmt.Sprintf("*3\r\n$9\r\nsubscribe\r\n$%d\r\n%s\r\n:1\r\n", len(channel), channel)
		if _, err := subWriter.WriteString(ack); err != nil {
			t.Errorf("failed to write subscribe ack: %v", err)
			return
		}
		if err := subWriter.Flush(); err != nil {
			t.Errorf("failed to flush subscribe ack: %v", err)
			return
		}

		close(ready)

		pubConn, err := ln.Accept()
		if err != nil {
			t.Errorf("failed to accept publisher: %v", err)
			return
		}
		defer pubConn.Close()
		pubReader := bufio.NewReader(pubConn)
		pubWriter := bufio.NewWriter(pubConn)

		pubArgs, err := readCommand(pubReader)
		if err != nil {
			t.Errorf("failed to read publish command: %v", err)
			return
		}
		if len(pubArgs) < 3 || strings.ToUpper(pubArgs[0]) != "PUBLISH" {
			t.Errorf("unexpected publish command: %v", pubArgs)
			return
		}
		payload := pubArgs[2]
		if _, err := pubWriter.WriteString(":1\r\n"); err != nil {
			t.Errorf("failed to write publish response: %v", err)
			return
		}
		if err := pubWriter.Flush(); err != nil {
			t.Errorf("failed to flush publish response: %v", err)
			return
		}

		message := fmt.Sprintf("*3\r\n$7\r\nmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n", len(channel), channel, len(payload), payload)
		if _, err := subWriter.WriteString(message); err != nil {
			t.Errorf("failed to write pubsub message: %v", err)
			return
		}
		if err := subWriter.Flush(); err != nil {
			t.Errorf("failed to flush pubsub message: %v", err)
			return
		}
	}()

	subscriber, err := NewRedisStatusSubscriber(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to create subscriber: %v", err)
	}
	t.Cleanup(func() { _ = subscriber.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream, err := subscriber.Subscribe(ctx, "session123")
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	t.Cleanup(func() { _ = stream.Close() })

	<-ready

	publisher, err := NewRedisStatusPublisher(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to create publisher: %v", err)
	}
	t.Cleanup(func() { _ = publisher.Close() })

	event := SessionStatusEvent{SessionID: "session123", Stage: "ingestion", State: "buffering", Timestamp: time.Now().UTC()}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case got, ok := <-stream.Events():
		if !ok {
			t.Fatal("events channel closed unexpectedly")
		}
		if got.SessionID != event.SessionID || got.Stage != event.Stage || got.State != event.State {
			t.Fatalf("unexpected event payload: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status event")
	}

	select {
	case err, ok := <-stream.Errors():
		if ok && err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	default:
	}

	<-done
}

func TestRedisStatusPublisherRequiresSessionID(t *testing.T) {
	publisher, err := NewRedisStatusPublisher("127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected error constructing publisher: %v", err)
	}
	t.Cleanup(func() { _ = publisher.Close() })

	if err := publisher.Publish(context.Background(), SessionStatusEvent{}); err == nil {
		t.Fatal("expected error when publishing without session id")
	}
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
