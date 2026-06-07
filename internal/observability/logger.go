package observability

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap's sugared API with a Winston/Pino-friendly surface.
// Use Info/Error for message-first logging (Winston), or InfoFields for
// fields-first logging (Pino). With/WithFields attach persistent metadata.
type Logger struct {
	sugar      *zap.SugaredLogger
	underlying *zap.Logger
}

// NewLogger returns a JSON logger that always includes service on every line.
func NewLogger(service string) (*Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "json"
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.CallerKey = ""

	base, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build zap logger: %w", err)
	}

	return wrap(base.With(zap.String("service", service))), nil
}

func wrap(l *zap.Logger) *Logger {
	return &Logger{
		sugar:      l.Sugar(),
		underlying: l,
	}
}

// With returns a child logger with extra default fields (Winston child logger).
func (l *Logger) With(keysAndValues ...any) *Logger {
	child := l.sugar.With(keysAndValues...)
	return &Logger{sugar: child, underlying: child.Desugar()}
}

// WithFields returns a child logger with extra default fields (Pino bindings).
// fields may be a map[string]any or a JSON object string.
func (l *Logger) WithFields(fields any) *Logger {
	return l.With(toKV(fields)...)
}

// Debug logs at debug level. Winston style: Debug("msg", "key", value, ...).
func (l *Logger) Debug(msg string, keysAndValues ...any) {
	l.sugar.Debugw(msg, keysAndValues...)
}

// Info logs at info level. Winston style: Info("msg", "key", value, ...).
func (l *Logger) Info(msg string, keysAndValues ...any) {
	l.sugar.Infow(msg, keysAndValues...)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, keysAndValues ...any) {
	l.sugar.Warnw(msg, keysAndValues...)
}

// Error logs at error level.
func (l *Logger) Error(msg string, keysAndValues ...any) {
	l.sugar.Errorw(msg, keysAndValues...)
}

// DebugFields logs at debug level. Pino style: DebugFields({"key": value}, "msg").
// fields may be a map[string]any or a JSON object string.
func (l *Logger) DebugFields(fields any, msg string) {
	l.sugar.Debugw(msg, toKV(fields)...)
}

// InfoFields logs at info level. Pino style: InfoFields({"key": value}, "msg").
// fields may be a map[string]any or a JSON object string.
func (l *Logger) InfoFields(fields any, msg string) {
	l.sugar.Infow(msg, toKV(fields)...)
}

// WarnFields logs at warn level.
// fields may be a map[string]any or a JSON object string.
func (l *Logger) WarnFields(fields any, msg string) {
	l.sugar.Warnw(msg, toKV(fields)...)
}

// ErrorFields logs at error level.
// fields may be a map[string]any or a JSON object string.
func (l *Logger) ErrorFields(fields any, msg string) {
	l.sugar.Errorw(msg, toKV(fields)...)
}

// Sync flushes any buffered log entries. Call once on shutdown via defer.
// Syncing stdout/stderr can return harmless errors on Unix; those are ignored.
func (l *Logger) Sync() error {
	if err := l.underlying.Sync(); err != nil && !isIgnorableSyncError(err) {
		return err
	}
	return nil
}

func isIgnorableSyncError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "sync /dev/stdout") ||
		strings.Contains(msg, "sync /dev/stderr")
}

// toKV converts fields to a flat key-value slice for zap's Infow/Errorw/etc.
// Accepted types:
//   - map[string]any  — used directly
//   - string          — interpreted as a JSON object and unmarshaled
func toKV(fields any) []any {
	var m map[string]any
	switch f := fields.(type) {
	case map[string]any:
		m = f
	case string:
		if err := json.Unmarshal([]byte(f), &m); err != nil {
			return []any{"raw_fields", f, "fields_parse_error", err.Error()}
		}
	default:
		return []any{"raw_fields", fields}
	}
	kv := make([]any, 0, len(m)*2)
	for k, v := range m {
		kv = append(kv, k, v)
	}
	return kv
}
