// Package main starts the API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// defaultListenAddr is the default address used when APP_SERVER_ADDR is not provided.
const defaultListenAddr = ":8080"

func main() {
	logger := newLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			// Some environments (e.g. tests) return "invalid argument" on Sync; ignore it.
			_ = err
		}
	}()

	addr := getListenAddr()

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthHandler(logger))

	sessionManager := newTranslationSessionManager()
	mux.HandleFunc("POST /sessions", createSessionHandler(sessionManager, logger))
	mux.HandleFunc("GET /sessions/{id}", getSessionHandler(sessionManager, logger))

	server := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(logger)(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Infow("server listening", "addr", addr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalw("server failed", "error", err)
		}
	}()

	<-shutdown
	logger.Infow("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Errorw("graceful shutdown failed", "error", err)
		if closeErr := server.Close(); closeErr != nil {
			logger.Errorw("forced close failed", "error", closeErr)
		}
	}
}

func getListenAddr() string {
	if addr := os.Getenv("APP_SERVER_ADDR"); addr != "" {
		return addr
	}
	return defaultListenAddr
}

func healthHandler(logger *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, `{"status":"ok"}`); err != nil {
			logger.Errorw("failed to write health response", "error", err)
		}
	})
}

func loggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(lrw, r)
			duration := time.Since(start)
			logger.Infow("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", lrw.statusCode,
				"duration", duration.String(),
			)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.statusCode = statusCode
	lrw.ResponseWriter.WriteHeader(statusCode)
}

func newLogger() *zap.SugaredLogger {
	level := strings.ToLower(os.Getenv("APP_LOG_LEVEL"))
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
