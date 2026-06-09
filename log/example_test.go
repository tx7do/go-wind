package log_test

import (
	"context"
	"fmt"

	"github.com/tx7do/go-wind/log"
)

// ExampleSetLogger demonstrates setting the package-level global logger.
// After this call, all code using [log.GetLogger] will receive the new
// logger. Pass nil to revert to the silent nopLogger.
func ExampleSetLogger() {
	// Use a custom Logger implementation or one from go-wind-plugins.
	// Here we demonstrate the global logger lifecycle.
	log.SetLogger(nil) // revert to nopLogger
	defer log.SetLogger(nil)

	logger := log.GetLogger()
	logger.Info(context.Background(), "hello from global logger")
	// Output:
}

// ExampleLogger demonstrates the Logger interface contract.
// Callers implement this interface to bridge their own logging backend.
func ExampleLogger() {
	// A minimal custom logger implementation:
	var l log.Logger = myLogger{}

	// All methods take context as the first argument for trace propagation.
	l.Debug(context.Background(), "debug message", "key", "value")
	l.Info(context.Background(), "info message")
	l.Warn(context.Background(), "warn message")
	l.Error(context.Background(), "error message")

	// Enabled lets callers guard expensive argument construction.
	if l.Enabled(log.LevelDebug) {
		l.Debug(context.Background(), "expensive", computeData())
	}

	// With returns a new logger with attached key-value pairs.
	child := l.With("module", "example")
	child.Info(context.Background(), "child logger")
	// Output:
	// DEBUG
	// INFO
	// WARN
	// ERROR
	// DEBUG
	// INFO
}

func computeData() any { return nil }

// myLogger is a minimal Logger implementation for demonstration purposes.
type myLogger struct{}

func (myLogger) Debug(context.Context, string, ...any) { fmt.Println("DEBUG") }
func (myLogger) Info(context.Context, string, ...any)  { fmt.Println("INFO") }
func (myLogger) Warn(context.Context, string, ...any)  { fmt.Println("WARN") }
func (myLogger) Error(context.Context, string, ...any) { fmt.Println("ERROR") }
func (myLogger) Enabled(log.Level) bool                { return true }
func (l myLogger) With(args ...any) log.Logger         { return l }
