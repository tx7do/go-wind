package log

import (
	"context"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// SetLogger / GetLogger must be safe to call concurrently with log calls.
// ---------------------------------------------------------------------------

func TestSetLogger_GetLogger_ConcurrentSafe(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: continuously swap loggers.
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			SetLogger(nopLogger{})
		}
	}()

	// Reader: continuously read the current logger.
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			l := GetLogger()
			l.Info(context.Background(), "test")
		}
	}()

	wg.Wait()
}

func TestSetLogger_NilRevertsToNop(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	SetLogger(nil)
	l := GetLogger()

	// Must not panic.
	l.Info(context.Background(), "should be nop")
	l.Error(context.Background(), "should be nop")
}

// ---------------------------------------------------------------------------
// SlogLogger must handle a nil context gracefully (replace with
// context.Background()) instead of panicking.
// ---------------------------------------------------------------------------

func TestSlogLogger_NilContextDoesNotPanic(t *testing.T) {
	l := NewSlogLogger()

	// nil context must not panic.
	l.Debug(nil, "debug message")
	l.Info(nil, "info message")
	l.Warn(nil, "warn message")
	l.Error(nil, "error message")
}

// ---------------------------------------------------------------------------
// SlogLogger.With must return a new logger whose records include the
// attached attributes.
// ---------------------------------------------------------------------------

func TestSlogLogger_WithAttachesAttributes(t *testing.T) {
	l := NewSlogLogger()

	// The With method must return a non-nil Logger with attached attrs.
	child := l.With("module", "test")
	if child == nil {
		t.Fatal("With returned nil")
	}

	// The child must still satisfy the Logger interface.
	child.Info(context.Background(), "hello")

	// Grandchild chaining also works.
	grandchild := child.With("request_id", "abc-123")
	grandchild.Info(context.Background(), "nested")
}

// ---------------------------------------------------------------------------
// nopLogger must implement Logger and discard everything silently.
// ---------------------------------------------------------------------------

func TestNopLogger_SilentlyDiscards(t *testing.T) {
	var l nopLogger

	// Must not panic on any method.
	l.Debug(context.Background(), "discarded")
	l.Info(context.Background(), "discarded")
	l.Warn(context.Background(), "discarded")
	l.Error(context.Background(), "discarded")

	// With must return a non-nil Logger.
	child := l.With("key", "value")
	if child == nil {
		t.Fatal("nopLogger.With returned nil")
	}
}
