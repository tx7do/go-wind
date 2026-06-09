package log_test

import (
	"context"

	"github.com/tx7do/go-wind/log"
)

// ExampleNewSlogLogger demonstrates creating the default slog-backed logger.
// The output goes to stderr at INFO level.
func ExampleNewSlogLogger() {
	logger := log.NewSlogLogger()
	logger.Info(context.Background(), "server started", "port", 8080)
}

// ExampleSetLogger demonstrates setting the package-level global logger.
// After this call, all code using [log.GetLogger] will receive the new
// logger. Pass nil to revert to the silent nopLogger.
func ExampleSetLogger() {
	log.SetLogger(log.NewSlogLogger())
	defer log.SetLogger(nil)

	logger := log.GetLogger()
	logger.Info(context.Background(), "hello from global logger")
}

// ExampleLevelFilter demonstrates filtering log messages by severity.
// Messages below the configured level are silently discarded without
// reaching the underlying logger.
func ExampleLevelFilter() {
	// Wrap an slog logger with a WARN threshold.
	filtered := log.LevelFilter{
		Logger: log.NewSlogLogger(),
		Level:  log.LevelWarn,
	}

	filtered.Info(context.Background(), "this is discarded") // below WARN
	filtered.Warn(context.Background(), "this is visible")   // at WARN
	filtered.Error(context.Background(), "this is visible")  // above WARN
}
