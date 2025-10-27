package main

import (
	"context"
	"errors"
	"time"

	queuepkg "streamlation/packages/backend/queue"
	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"

	"go.uber.org/zap"
)

type queueConsumer interface {
	Pop(ctx context.Context, timeout time.Duration) (*queuepkg.IngestionJob, error)
}

type sessionGetter interface {
	Get(ctx context.Context, id string) (sessionpkg.TranslationSession, error)
}

type statusPublisher interface {
	Publish(ctx context.Context, event statuspkg.SessionStatusEvent) error
}

type sessionIngestor interface {
	Ingest(ctx context.Context, session sessionpkg.TranslationSession) error
}

// IngestionWorker coordinates ingestion jobs from Redis and prepares them for the media pipeline.
type IngestionWorker struct {
	queue        queueConsumer
	sessions     sessionGetter
	publisher    statusPublisher
	ingestor     sessionIngestor
	logger       *zap.SugaredLogger
	pollInterval time.Duration
	idleDelay    time.Duration
}

// NewIngestionWorker constructs a worker instance with sane defaults.
func NewIngestionWorker(queue queueConsumer, sessions sessionGetter, publisher statusPublisher, ingestor sessionIngestor, logger *zap.SugaredLogger, pollInterval time.Duration) *IngestionWorker {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	return &IngestionWorker{
		queue:        queue,
		sessions:     sessions,
		publisher:    publisher,
		ingestor:     ingestor,
		logger:       logger,
		pollInterval: pollInterval,
		idleDelay:    500 * time.Millisecond,
	}
}

// Run starts the worker loop until the context is cancelled.
func (w *IngestionWorker) Run(ctx context.Context) error {
	w.logger.Infow("ingestion worker started", "pollInterval", w.pollInterval.String())
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		job, err := w.queue.Pop(ctx, w.pollInterval)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			w.logger.Errorw("failed to pop ingestion job", "error", err)
			select {
			case <-time.After(w.idleDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		if job == nil {
			continue
		}
		w.handleJob(ctx, job)
	}
}

func (w *IngestionWorker) handleJob(ctx context.Context, job *queuepkg.IngestionJob) {
	start := time.Now().UTC()
	w.logger.Infow("processing ingestion job", "sessionID", job.SessionID)

	session, err := w.sessions.Get(ctx, job.SessionID)
	if err != nil {
		w.publishStatus(ctx, statuspkg.SessionStatusEvent{
			SessionID: job.SessionID,
			Stage:     "ingestion",
			State:     "error",
			Detail:    "failed to load session",
			Timestamp: time.Now().UTC(),
		})
		w.logger.Errorw("failed to load session", "error", err, "sessionID", job.SessionID)
		return
	}

	w.publishStatus(ctx, statuspkg.SessionStatusEvent{
		SessionID: session.ID,
		Stage:     "ingestion",
		State:     "started",
		Detail:    "ingestion pipeline starting",
		Timestamp: start,
	})

	if err := w.ingestor.Ingest(ctx, session); err != nil {
		if errors.Is(err, context.Canceled) {
			w.logger.Warnw("ingestion canceled", "sessionID", session.ID)
			return
		}
		w.publishStatus(ctx, statuspkg.SessionStatusEvent{
			SessionID: session.ID,
			Stage:     "ingestion",
			State:     "error",
			Detail:    "ingestion pipeline failed",
			Timestamp: time.Now().UTC(),
		})
		w.logger.Errorw("ingestion failed", "error", err, "sessionID", session.ID)
		return
	}

	w.publishStatus(ctx, statuspkg.SessionStatusEvent{
		SessionID: session.ID,
		Stage:     "ingestion",
		State:     "completed",
		Detail:    "ingestion pipeline completed",
		Timestamp: time.Now().UTC(),
	})
	w.logger.Infow("ingestion completed", "sessionID", session.ID, "duration", time.Since(start).String())
}

func (w *IngestionWorker) publishStatus(ctx context.Context, event statuspkg.SessionStatusEvent) {
	if w.publisher == nil {
		return
	}
	if err := w.publisher.Publish(ctx, event); err != nil {
		w.logger.Errorw("failed to publish status event", "error", err, "sessionID", event.SessionID, "stage", event.Stage, "state", event.State)
	}
}
