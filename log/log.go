package log

import "context"

// Logger is the minimal logging interface used throughout the framework.
// See the package documentation (doc.go) for how to adapt a backend and inject it.
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
