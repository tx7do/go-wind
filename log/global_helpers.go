package log

import "context"

// Debug logs via the package-level global logger.
func Debug(ctx context.Context, msg string, args ...any) {
	GetLogger().Debug(ctx, msg, args...)
}

// Info logs via the package-level global logger.
func Info(ctx context.Context, msg string, args ...any) {
	GetLogger().Info(ctx, msg, args...)
}

// Warn logs via the package-level global logger.
func Warn(ctx context.Context, msg string, args ...any) {
	GetLogger().Warn(ctx, msg, args...)
}

// Error logs via the package-level global logger.
func Error(ctx context.Context, msg string, args ...any) {
	GetLogger().Error(ctx, msg, args...)
}

// Enabled reports whether the global logger emits records at the given level.
func Enabled(level Level) bool {
	return GetLogger().Enabled(level)
}

// With returns a child logger from the global logger with attached key-values.
func With(args ...any) Logger {
	return GetLogger().With(args...)
}

