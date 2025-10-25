package zap

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents logging severity.
type Level int8

const (
	DebugLevel Level = -1
	InfoLevel  Level = 0
	WarnLevel  Level = 1
	ErrorLevel Level = 2
	FatalLevel Level = 3
)

// AtomicLevel stores a log level that can be shared across loggers.
type AtomicLevel struct {
	level Level
	mu    sync.RWMutex
}

// NewAtomicLevelAt creates an AtomicLevel seeded with the provided level.
func NewAtomicLevelAt(l Level) AtomicLevel {
	return AtomicLevel{level: l}
}

// Level returns the current severity threshold.
func (a *AtomicLevel) Level() Level {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.level
}

// SetLevel updates the severity threshold.
func (a *AtomicLevel) SetLevel(l Level) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.level = l
}

// Config mirrors the subset of zap.Config used within this project.
type Config struct {
	Level AtomicLevel
}

// NewProductionConfig returns a Config pre-configured for production use.
func NewProductionConfig() Config {
	return Config{Level: NewAtomicLevelAt(InfoLevel)}
}

// Core represents the logger core exposed by zap.
type Core interface {
	Enabled(Level) bool
}

type simpleCore struct {
	level Level
}

func (c *simpleCore) Enabled(l Level) bool {
	return l >= c.level
}

// Logger is a minimal stand-in for zap.Logger.
type Logger struct {
	core   *simpleCore
	logger *log.Logger
}

// SugaredLogger mimics zap.SugaredLogger.
type SugaredLogger struct {
	base *Logger
}

// Build constructs a Logger from the config.
func (c Config) Build() (*Logger, error) {
	std := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.LUTC)
	return &Logger{
		core:   &simpleCore{level: c.Level.Level()},
		logger: std,
	}, nil
}

// Sugar returns a SugaredLogger wrapper.
func (l *Logger) Sugar() *SugaredLogger {
	return &SugaredLogger{base: l}
}

// Desugar returns the underlying Logger.
func (s *SugaredLogger) Desugar() *Logger {
	return s.base
}

// Core exposes the logger core.
func (l *Logger) Core() Core {
	return l.core
}

// Core exposes the logger core for the sugared variant.
func (s *SugaredLogger) Core() Core {
	return s.base.core
}

// Sync is a no-op retained for API parity.
func (l *Logger) Sync() error { return nil }

// Sync is a no-op for SugaredLogger.
func (s *SugaredLogger) Sync() error { return s.base.Sync() }

// Infow logs at info level with structured context.
func (s *SugaredLogger) Infow(msg string, keysAndValues ...interface{}) {
	s.log(InfoLevel, msg, keysAndValues...)
}

// Errorw logs at error level with structured context.
func (s *SugaredLogger) Errorw(msg string, keysAndValues ...interface{}) {
	s.log(ErrorLevel, msg, keysAndValues...)
}

// Fatalw logs at fatal level and exits the process.
func (s *SugaredLogger) Fatalw(msg string, keysAndValues ...interface{}) {
	s.log(FatalLevel, msg, keysAndValues...)
	os.Exit(1)
}

// Debugw logs at debug level.
func (s *SugaredLogger) Debugw(msg string, keysAndValues ...interface{}) {
	s.log(DebugLevel, msg, keysAndValues...)
}

func (s *SugaredLogger) log(level Level, msg string, keysAndValues ...interface{}) {
	if !s.base.core.Enabled(level) {
		return
	}
	s.base.logger.Printf("%s\t%s", levelString(level), formatMessage(msg, keysAndValues...))
}

func levelString(l Level) string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "INFO"
	}
}

func formatMessage(msg string, keysAndValues ...interface{}) string {
	if len(keysAndValues) == 0 {
		return msg
	}
	builder := strings.Builder{}
	builder.WriteString(msg)
	builder.WriteRune('\t')
	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprint(keysAndValues[i])
		var value interface{}
		if i+1 < len(keysAndValues) {
			value = keysAndValues[i+1]
		}
		builder.WriteString(key)
		builder.WriteRune('=')
		builder.WriteString(fmt.Sprint(value))
		if i+2 < len(keysAndValues) {
			builder.WriteRune(' ')
		}
	}
	return builder.String()
}

// With returns the same logger for compatibility.
func (s *SugaredLogger) With(keysAndValues ...interface{}) *SugaredLogger {
	_ = keysAndValues
	return s
}

// WithOptions returns the same logger to preserve compatibility.
func (l *Logger) WithOptions(options ...interface{}) *Logger {
	_ = options
	return l
}

// Named returns the same logger (namespace not supported in stub).
func (l *Logger) Named(name string) *Logger {
	_ = name
	return l
}

// With adds context and returns the same logger for compatibility.
func (l *Logger) With(fields ...interface{}) *Logger {
	_ = fields
	return l
}

// Sugar returns the SugaredLogger wrapper (already implemented above but provided for completeness).
func (l *Logger) WithSugared(fields ...interface{}) *SugaredLogger {
	_ = fields
	return l.Sugar()
}

// Check ensures compatibility with zap's API surface used in tests.
func (l *Logger) Check(level Level, msg string) bool {
	return l.core.Enabled(level)
}

// Warnw logs at warn level.
func (s *SugaredLogger) Warnw(msg string, keysAndValues ...interface{}) {
	s.log(WarnLevel, msg, keysAndValues...)
}

// Panicw logs and panics for compatibility.
func (s *SugaredLogger) Panicw(msg string, keysAndValues ...interface{}) {
	s.log(FatalLevel, msg, keysAndValues...)
	panic(msg)
}

// TimeFormat is exposed for compatibility with zap but unused here.
const TimeFormat = time.RFC3339Nano
