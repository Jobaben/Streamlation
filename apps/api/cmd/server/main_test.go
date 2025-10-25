package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestGetListenAddr(t *testing.T) {
	t.Setenv("APP_SERVER_ADDR", "127.0.0.1:9000")

	got := getListenAddr()
	if got != "127.0.0.1:9000" {
		t.Fatalf("expected 127.0.0.1:9000, got %s", got)
	}
}

func TestGetDatabaseURLDefaults(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "")
	if got := getDatabaseURL(); got != defaultDatabaseURL {
		t.Fatalf("expected default database URL, got %s", got)
	}
}

func TestGetRedisAddrDefaults(t *testing.T) {
	t.Setenv("APP_REDIS_ADDR", "")
	if got := getRedisAddr(); got != defaultRedisAddr {
		t.Fatalf("expected default redis addr, got %s", got)
	}
}

func TestHealthHandler(t *testing.T) {
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler := healthHandler(logger)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", rr.Code)
	}

	if body := rr.Body.String(); body != "{\"status\":\"ok\"}" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestNewLoggerHonorsEnv(t *testing.T) {
	t.Setenv("APP_LOG_LEVEL", "debug")
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	if !logger.Desugar().Core().Enabled(zap.DebugLevel) {
		t.Fatal("expected logger to enable debug level")
	}
}

// Ensure newLogger does not panic when env is unset.
func TestNewLoggerDefaultLevel(t *testing.T) {
	t.Setenv("APP_LOG_LEVEL", "")
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	if logger == nil {
		t.Fatal("expected logger instance")
	}
}
