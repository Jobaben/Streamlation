// Package main provides the ingestion worker entrypoint.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	postgres "streamlation/packages/backend/postgres"
	queuepkg "streamlation/packages/backend/queue"
	statuspkg "streamlation/packages/backend/status"

	"go.uber.org/zap"
)

const (
	defaultDatabaseURL = "postgres://streamlation:streamlation@localhost:5432/streamlation?sslmode=disable"
	defaultRedisAddr   = "127.0.0.1:6379"
)

func main() {
	logger := newLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			_ = err
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-shutdown
		logger.Infow("shutdown signal received", "signal", sig.String())
		cancel()
	}()

	dbURL := getEnv("WORKER_DATABASE_URL", defaultDatabaseURL)
	redisAddr := getEnv("WORKER_REDIS_ADDR", defaultRedisAddr)
	pollInterval := getDurationEnv("WORKER_POLL_INTERVAL", 5*time.Second)

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

	sessionStore := postgres.NewSessionStore(pgClient)
	queue, err := queuepkg.NewRedisIngestionConsumer(redisAddr)
	if err != nil {
		logger.Fatalw("failed to create redis ingestion consumer", "error", err)
	}
	defer func() { _ = queue.Close() }()

	publisher, err := statuspkg.NewRedisStatusPublisher(redisAddr)
	if err != nil {
		logger.Fatalw("failed to create redis status publisher", "error", err)
	}
	defer func() { _ = publisher.Close() }()
	ingestor := newStreamIngestor(logger)

	worker := NewIngestionWorker(queue, sessionStore, publisher, ingestor, logger, pollInterval)
	if err := worker.Run(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Fatalw("ingestion worker terminated", "error", err)
		}
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid duration for %s: %v\n", key, err)
		return fallback
	}
	if d <= 0 {
		return fallback
	}
	return d
}

func newLogger() *zap.SugaredLogger {
	level := strings.ToLower(os.Getenv("WORKER_LOG_LEVEL"))
	cfg := zap.NewProductionConfig()

	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "warn", "warning":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}

	return logger.Sugar()
}
