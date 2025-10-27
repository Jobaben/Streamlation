package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"
)

func TestSequentialStubEmitsSteps(t *testing.T) {
	session := sessionpkg.TranslationSession{ID: "session-123"}
	steps := []Step{
		{Stage: "ingestion", State: "buffering", Detail: "initialising"},
		{Stage: "asr", State: "processing", Detail: "transcribing"},
	}

	runner := NewSequentialStub(steps)

	var events []statuspkg.SessionStatusEvent
	emit := func(event statuspkg.SessionStatusEvent) error {
		events = append(events, event)
		return nil
	}

	ctx := context.Background()
	if err := runner.Run(ctx, session, emit); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != len(steps) {
		t.Fatalf("expected %d events, got %d", len(steps), len(events))
	}

	for i, event := range events {
		if event.SessionID != session.ID {
			t.Fatalf("event %d missing session id", i)
		}
		if event.Stage != steps[i].Stage || event.State != steps[i].State {
			t.Fatalf("unexpected event %d: %#v", i, event)
		}
		if event.Timestamp.IsZero() {
			t.Fatalf("event %d missing timestamp", i)
		}
	}
}

func TestSequentialStubHonoursContextCancellation(t *testing.T) {
	session := sessionpkg.TranslationSession{ID: "cancel-me"}
	steps := []Step{
		{Stage: "ingestion", State: "buffering", Delay: 50 * time.Millisecond},
		{Stage: "translation", State: "generating"},
	}

	runner := NewSequentialStub(steps)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emit := func(statuspkg.SessionStatusEvent) error {
		cancel()
		return nil
	}

	err := runner.Run(ctx, session, emit)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestSequentialStubPropagatesEmitErrors(t *testing.T) {
	session := sessionpkg.TranslationSession{ID: "emit-error"}
	steps := []Step{{Stage: "output", State: "writing"}}

	runner := NewSequentialStub(steps)

	emitErr := errors.New("fail")

	err := runner.Run(context.Background(), session, func(statuspkg.SessionStatusEvent) error {
		return emitErr
	})

	if !errors.Is(err, emitErr) {
		t.Fatalf("expected emit error, got %v", err)
	}
}
