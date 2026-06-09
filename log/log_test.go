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

	// Enabled must return false for all levels.
	if l.Enabled(LevelDebug) {
		t.Error("nopLogger.Enabled should return false")
	}
	if l.Enabled(LevelError) {
		t.Error("nopLogger.Enabled should return false")
	}
}

// ---------------------------------------------------------------------------
// Level.String should return human-readable names.
// ---------------------------------------------------------------------------

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}
