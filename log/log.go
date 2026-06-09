// Package log provides a minimal, backend-agnostic logging interface for the
// go-wind framework.
//
// It defines a small [Logger] interface that any project can adapt to its own
// backend (stdlib *slog.Logger, zap, zerolog, kratos log, …) with a few lines
// of glue code. A zero-cost [nopLogger] is included as the default; concrete
// adapters (slog, LevelFilter, MultiLogger) live in go-wind-plugins.
package log

import "context"

// Logger is the minimal logging interface used throughout the framework.
//
// It is deliberately small (4 log methods + With) so that any project can
// adapt its own backend (the stdlib *slog.Logger, zap, zerolog, kratos log,
// …) with a few lines of glue code and inject it via [SetLogger].
//
// The first argument is always a context, so backends that support it can
// extract trace ids / request-scoped attributes. Callers that have no context
// at hand may pass nil.
type Logger interface {
	// Debug logs a message at DEBUG level with optional structured key/value
	// pairs passed as args (alternating keys and values).
	Debug(ctx context.Context, msg string, args ...any)
	// Info logs a message at INFO level.
	Info(ctx context.Context, msg string, args ...any)
	// Warn logs a message at WARN level.
	Warn(ctx context.Context, msg string, args ...any)
	// Error logs a message at ERROR level.
	Error(ctx context.Context, msg string, args ...any)

	// Enabled reports whether the logger emits records at the given [Level].
	// Expensive argument construction can be guarded by this check:
	//
	//		if logger.Enabled(log.LevelDebug) {
	//		    logger.Debug(ctx, "detail", computeExpensiveData())
	//		}
	Enabled(level Level) bool

	// With returns a new Logger instance with the given key-value pairs
	// attached. This is typically used to distinguish modules, e.g.,
	// logger.With("module", "registry").
	With(args ...any) Logger
}
