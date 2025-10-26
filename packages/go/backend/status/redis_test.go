package status

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"
)

func TestChannelName(t *testing.T) {
	got := channelName("session123")
	if got != "streamlation:session:session123:status" {
		t.Fatalf("unexpected channel name: %s", got)
	}
}

func TestReadPubSubMessage_Message(t *testing.T) {
	channel := "streamlation:session:abc:status"
	body := "{\"sessionId\":\"abc\"}"
	payload := fmt.Sprintf("*3\r\n$7\r\nmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n", len(channel), channel, len(body), body)
	reader := bufio.NewReader(bytes.NewBufferString(payload))
	msg, err := readPubSubMessage(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.kind != "message" {
		t.Fatalf("expected kind message, got %s", msg.kind)
	}
	if msg.channel != "streamlation:session:abc:status" {
		t.Fatalf("unexpected channel: %s", msg.channel)
	}
	if msg.payload != body {
		t.Fatalf("unexpected payload: %s", msg.payload)
	}
}

func TestReadPubSubMessage_Subscribe(t *testing.T) {
	channel := "streamlation:session:def:status"
	payload := fmt.Sprintf("*3\r\n$9\r\nsubscribe\r\n$%d\r\n%s\r\n:%d\r\n", len(channel), channel, 1)
	reader := bufio.NewReader(bytes.NewBufferString(payload))
	msg, err := readPubSubMessage(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.kind != "subscribe" {
		t.Fatalf("expected subscribe message, got %s", msg.kind)
	}
	if msg.channel != "streamlation:session:def:status" {
		t.Fatalf("unexpected channel: %s", msg.channel)
	}
	if msg.payload != "1" {
		t.Fatalf("expected payload 1, got %s", msg.payload)
	}
}
