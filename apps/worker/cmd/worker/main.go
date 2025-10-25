// Package main contains the worker service entry point.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	logger.Infow("worker starting")

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logger.Debugw("worker heartbeat")
			case <-ctx.Done():
				return
			}
		}
	}()

	<-signals
	logger.Infow("worker shutdown signal received")
	cancel()
	time.Sleep(500 * time.Millisecond)
	logger.Infow("worker stopped")
}

func newLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger.Sugar()
}
