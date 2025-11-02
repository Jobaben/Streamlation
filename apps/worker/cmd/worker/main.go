// Package main contains the worker service entry point.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	pipelinepkg "streamlation/packages/backend/pipeline"
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

	pipeline := pipelinepkg.NewSequentialStub([]pipelinepkg.Step{
		{Stage: "ingestion", State: "buffering", Detail: "fetching stream metadata"},
		{Stage: "media", State: "normalizing", Detail: "standardizing audio"},
		{Stage: "asr", State: "processing", Detail: "transcribing audio chunks"},
		{Stage: "translation", State: "generating", Detail: "producing target language captions"},
		{Stage: "output", State: "rendering", Detail: "assembling subtitle artifacts"},
	})

	processor := &ingestionProcessor{
		store:         store,
		consumer:      consumer,
		publisher:     statusPublisher,
		pipeline:      pipeline,
		logger:        logger,
		maxConcurrent: getWorkerConcurrency(),
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

func getWorkerConcurrency() int {
	raw := os.Getenv("WORKER_MAX_CONCURRENCY")
	if raw == "" {
		return 4
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 4
	}
	return value
}

type sessionStore interface {
	Get(ctx context.Context, id string) (sessionpkg.TranslationSession, error)
}

type ingestionConsumer interface {
	Pop(ctx context.Context, timeout time.Duration) (*queuepkg.IngestionJob, error)
}

type ingestionProcessor struct {
	store         sessionStore
	consumer      ingestionConsumer
	publisher     statusPublisher
	pipeline      pipelinepkg.Runner
	logger        *zap.SugaredLogger
	maxConcurrent int
}

func (p *ingestionProcessor) Run(ctx context.Context) {
	concurrency := p.maxConcurrent
	if concurrency <= 0 {
		concurrency = 1
	}

	jobs := make(chan *queuepkg.IngestionJob, concurrency)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.processJobs(workerCtx, jobs)
		}()
	}

	defer func() {
		close(jobs)
		wg.Wait()
	}()

	for {
		if workerCtx.Err() != nil {
			return
		}

		job, err := p.consumer.Pop(workerCtx, 5*time.Second)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if workerCtx.Err() != nil {
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

		select {
		case jobs <- job:
		case <-workerCtx.Done():
			return
		}
	}
}

func (p *ingestionProcessor) processJobs(ctx context.Context, jobs <-chan *queuepkg.IngestionJob) {
	drainCtx := context.WithoutCancel(ctx)
	jobCtx := ctx

	for {
		if jobCtx == ctx && ctx.Err() != nil {
			jobCtx = drainCtx
		}

		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}
			p.handleJob(jobCtx, job)
		case <-ctx.Done():
			jobCtx = drainCtx

			select {
			case job, ok := <-jobs:
				if !ok {
					return
				}
				p.handleJob(jobCtx, job)
			default:
				return
			}
		}
	}
}

func (p *ingestionProcessor) handleJob(ctx context.Context, job *queuepkg.IngestionJob) {
	if job == nil {
		return
	}

	_ = p.publish(ctx, statuspkg.SessionStatusEvent{
		SessionID: job.SessionID,
		Stage:     "ingestion",
		State:     "dequeued",
		Detail:    "ingestion job received",
	})

	session, err := p.store.Get(ctx, job.SessionID)
	if err != nil {
		if errors.Is(err, postgres.ErrSessionNotFound) {
			p.logger.Warnw("session not found for ingestion job", "sessionID", job.SessionID)
			_ = p.publish(ctx, statuspkg.SessionStatusEvent{
				SessionID: job.SessionID,
				Stage:     "session",
				State:     "not_found",
				Detail:    "session missing for ingestion job",
			})
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		p.logger.Errorw("failed to load session for ingestion job", "error", err, "sessionID", job.SessionID)
		_ = p.publish(ctx, statuspkg.SessionStatusEvent{
			SessionID: job.SessionID,
			Stage:     "ingestion",
			State:     "error",
			Detail:    "failed to load session metadata",
		})
		return
	}

	_ = p.publish(ctx, statuspkg.SessionStatusEvent{
		SessionID: session.ID,
		Stage:     "ingestion",
		State:     "ready",
		Detail:    "ingestion job ready",
	})

	p.logger.Infow("ingestion job ready", "sessionID", session.ID, "sourceType", session.Source.Type, "sourceURI", session.Source.URI, "targetLanguage", session.TargetLanguage)

	if p.pipeline != nil {
		if err := p.pipeline.Run(ctx, session, func(event statuspkg.SessionStatusEvent) error {
			return p.publish(ctx, event)
		}); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			p.logger.Errorw("pipeline execution failed", "error", err, "sessionID", session.ID)
			_ = p.publish(ctx, statuspkg.SessionStatusEvent{
				SessionID: session.ID,
				Stage:     "pipeline",
				State:     "error",
				Detail:    err.Error(),
			})
		}
	}
}

type statusPublisher interface {
	Publish(ctx context.Context, event statuspkg.SessionStatusEvent) error
}

func (p *ingestionProcessor) publish(ctx context.Context, event statuspkg.SessionStatusEvent) error {
	if p.publisher == nil {
		return nil
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if err := p.publisher.Publish(ctx, event); err != nil {
		p.logger.Errorw("failed to publish status event", "error", err, "sessionID", event.SessionID, "stage", event.Stage, "state", event.State)
		return err
	}
	return nil
}

func newLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger.Sugar()
}
