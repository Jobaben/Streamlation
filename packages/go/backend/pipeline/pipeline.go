package pipeline

import (
	"context"
	"time"

	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"
)

// Runner coordinates the translation pipeline for a session and emits
// structured progress events as work advances through each stage.
type Runner interface {
	Run(ctx context.Context, session sessionpkg.TranslationSession, emit func(statuspkg.SessionStatusEvent) error) error
}

// Step describes a synthetic pipeline stage used by the stub runner to
// simulate end-to-end progress without requiring the full media stack.
type Step struct {
	Stage  string
	State  string
	Detail string
	Delay  time.Duration
}

// SequentialStub emits a predefined sequence of steps, waiting for the
// optional delay between each update. It is useful for development and tests
// until the real ingestion, ASR, and translation processors arrive.
type SequentialStub struct {
	steps []Step
}

// NewSequentialStub constructs a stub runner that emits the provided steps in
// order.
func NewSequentialStub(steps []Step) *SequentialStub {
	return &SequentialStub{steps: append([]Step(nil), steps...)}
}

// Run emits each configured step while honouring context cancellation. If a
// step delay is specified the runner waits before emitting the status update.
func (s *SequentialStub) Run(ctx context.Context, session sessionpkg.TranslationSession, emit func(statuspkg.SessionStatusEvent) error) error {
	if emit == nil {
		return nil
	}

	for _, step := range s.steps {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if step.Delay > 0 {
			select {
			case <-time.After(step.Delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		event := statuspkg.SessionStatusEvent{
			SessionID: session.ID,
			Stage:     step.Stage,
			State:     step.State,
			Detail:    step.Detail,
			Timestamp: time.Now().UTC(),
		}

		if err := emit(event); err != nil {
			return err
		}
	}

	return nil
}
