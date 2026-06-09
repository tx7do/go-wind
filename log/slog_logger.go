package log

import (
	"context"
	"log/slog"
	"os"
)

// SlogLogger adapts the stdlib [*slog.Logger] to the [Logger] interface.
//
// This is the reference implementation returned by [NewSlogLogger]. Callers
// that already own a configured *slog.Logger can wrap it directly:
//
//	log.SetLogger(log.SlogLogger{L: mySlogLogger})
type SlogLogger struct {
	// L is the underlying slog logger. It MUST be non-nil; [NewSlogLogger]
	// always returns a ready-to-use instance.
	L *slog.Logger
}

// Debug forwards to slog.Logger.DebugContext.
func (s SlogLogger) Debug(ctx context.Context, msg string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.L.DebugContext(ctx, msg, args...)
}

// Info forwards to slog.Logger.InfoContext.
func (s SlogLogger) Info(ctx context.Context, msg string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.L.InfoContext(ctx, msg, args...)
}

// Warn forwards to slog.Logger.WarnContext.
func (s SlogLogger) Warn(ctx context.Context, msg string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.L.WarnContext(ctx, msg, args...)
}

// Error forwards to slog.Logger.ErrorContext.
func (s SlogLogger) Error(ctx context.Context, msg string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.L.ErrorContext(ctx, msg, args...)
}

// With returns a new SlogLogger whose underlying *slog.Logger has the given
// key-value pairs attached. This is typically used to distinguish modules,
// e.g., logger.With("module", "registry"). The returned logger will include
// these attributes in every log record it produces.
func (s SlogLogger) With(args ...any) Logger {
	return SlogLogger{L: s.L.With(args...)}
}

// Compile-time assertion: SlogLogger implements Logger.
var _ Logger = SlogLogger{}

// NewSlogLogger builds a [SlogLogger] backed by the stdlib slog with sensible
// defaults: a text handler writing to stderr at INFO level.
//
// Callers needing a different format / level / destination should either:
//   - build their own *slog.Logger and wrap it: SlogLogger{L: myLogger}
//   - or implement the [Logger] interface themselves and pass it to
//     [SetLogger].
func NewSlogLogger() Logger {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	return SlogLogger{L: slog.New(h)}
}
