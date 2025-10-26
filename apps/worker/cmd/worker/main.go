// Package main contains the worker service entry point.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	postgres "streamlation/packages/backend/postgres"
	queuepkg "streamlation/packages/backend/queue"
	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"

	"go.uber.org/zap"
)

const (
	defaultDatabaseURL = "postgres://streamlation:streamlation@localhost:5432/streamlation?sslmode=disable"
	defaultRedisAddr   = "127.0.0.1:6379"
)

func main() {
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	dbURL := getDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dbURL)
	if err != nil {
		logger.Fatalw("failed to connect to database", "error", err)
	}
	defer func() {
		if err := pgClient.Close(); err != nil {
			logger.Errorw("failed to close database connection", "error", err)
		}
	}()

	if err := postgres.EnsureSessionSchema(ctx, pgClient); err != nil {
		logger.Fatalw("failed to ensure session schema", "error", err)
	}

	store := postgres.NewSessionStore(pgClient)
	redisAddr := getRedisAddr()
	consumer := queuepkg.NewRedisIngestionConsumer(redisAddr)
	statusPublisher := statuspkg.NewRedisStatusPublisher(redisAddr)

	processor := &ingestionProcessor{
		store:     store,
		consumer:  consumer,
		publisher: statusPublisher,
		logger:    logger,
	}

	logger.Infow("worker starting")

	go processor.Run(ctx)

	<-signals
	logger.Infow("worker shutdown signal received")
	cancel()
	time.Sleep(500 * time.Millisecond)
	logger.Infow("worker stopped")
}

func getDatabaseURL() string {
	if url := os.Getenv("WORKER_DATABASE_URL"); url != "" {
		return url
	}
	return defaultDatabaseURL
}

func getRedisAddr() string {
	if addr := os.Getenv("WORKER_REDIS_ADDR"); addr != "" {
		return addr
	}
	return defaultRedisAddr
}

type sessionStore interface {
	Get(ctx context.Context, id string) (sessionpkg.TranslationSession, error)
}

type ingestionConsumer interface {
	Pop(ctx context.Context, timeout time.Duration) (*queuepkg.IngestionJob, error)
}

type ingestionProcessor struct {
	store     sessionStore
	consumer  ingestionConsumer
	publisher statusPublisher
	logger    *zap.SugaredLogger
}

func (p *ingestionProcessor) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		job, err := p.consumer.Pop(ctx, 5*time.Second)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			p.logger.Errorw("failed to pop ingestion job", "error", err)
			continue
		}
		if job == nil {
			continue
		}

		p.publish(ctx, statuspkg.SessionStatusEvent{
			SessionID: job.SessionID,
			Stage:     "ingestion",
			State:     "dequeued",
			Detail:    "ingestion job received",
		})

		session, err := p.store.Get(ctx, job.SessionID)
		if err != nil {
			if errors.Is(err, postgres.ErrSessionNotFound) {
				p.logger.Warnw("session not found for ingestion job", "sessionID", job.SessionID)
				p.publish(ctx, statuspkg.SessionStatusEvent{
					SessionID: job.SessionID,
					Stage:     "session",
					State:     "not_found",
					Detail:    "session missing for ingestion job",
				})
				continue
			}
			if errors.Is(err, context.Canceled) {
				return
			}
			p.logger.Errorw("failed to load session for ingestion job", "error", err, "sessionID", job.SessionID)
			p.publish(ctx, statuspkg.SessionStatusEvent{
				SessionID: job.SessionID,
				Stage:     "ingestion",
				State:     "error",
				Detail:    "failed to load session metadata",
			})
			continue
		}

		p.publish(ctx, statuspkg.SessionStatusEvent{
			SessionID: session.ID,
			Stage:     "ingestion",
			State:     "ready",
			Detail:    "ingestion job ready",
		})

		p.logger.Infow("ingestion job ready", "sessionID", session.ID, "sourceType", session.Source.Type, "sourceURI", session.Source.URI, "targetLanguage", session.TargetLanguage)
	}
}

type statusPublisher interface {
	Publish(ctx context.Context, event statuspkg.SessionStatusEvent) error
}

func (p *ingestionProcessor) publish(ctx context.Context, event statuspkg.SessionStatusEvent) {
	if p.publisher == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if err := p.publisher.Publish(ctx, event); err != nil {
		p.logger.Errorw("failed to publish status event", "error", err, "sessionID", event.SessionID, "stage", event.Stage, "state", event.State)
	}
}

func newLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger.Sugar()
}
