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

	// Enabled must return false for all levels.
	if l.Enabled(LevelDebug) {
		t.Error("nopLogger.Enabled should return false")
	}
	if l.Enabled(LevelError) {
		t.Error("nopLogger.Enabled should return false")
	}
}

// ---------------------------------------------------------------------------
// SlogLogger.Enabled should correctly report level availability.
// ---------------------------------------------------------------------------

func TestSlogLogger_Enabled(t *testing.T) {
	l := NewSlogLogger() // default: INFO level

	// INFO and above should be enabled.
	if !l.Enabled(LevelInfo) {
		t.Error("INFO should be enabled at default level")
	}
	if !l.Enabled(LevelError) {
		t.Error("ERROR should always be enabled")
	}

	// DEBUG should NOT be enabled (default handler is INFO).
	if l.Enabled(LevelDebug) {
		t.Error("DEBUG should NOT be enabled at default INFO level")
	}
}

// ---------------------------------------------------------------------------
// LevelFilter.Enabled should respect both the filter threshold and the
// underlying logger's own level.
// ---------------------------------------------------------------------------

func TestLevelFilter_Enabled(t *testing.T) {
	lf := LevelFilter{
		Logger: NewSlogLogger(), // INFO level
		Level:  LevelWarn,
	}

	// DEBUG: below filter threshold → false
	if lf.Enabled(LevelDebug) {
		t.Error("DEBUG should be filtered out (threshold=WARN)")
	}
	// INFO: below filter threshold → false
	if lf.Enabled(LevelInfo) {
		t.Error("INFO should be filtered out (threshold=WARN)")
	}
	// WARN: at threshold, and underlying (INFO) enables it → true
	if !lf.Enabled(LevelWarn) {
		t.Error("WARN should be enabled")
	}
	// ERROR: above threshold, underlying enables it → true
	if !lf.Enabled(LevelError) {
		t.Error("ERROR should be enabled")
	}
}

// ---------------------------------------------------------------------------
// MultiLogger should fan out to all underlying loggers and Enabled should
// return true if any logger is enabled.
// ---------------------------------------------------------------------------

func TestMultiLogger_FanOut(t *testing.T) {
	ml := MultiLogger{
		Loggers: []Logger{
			nopLogger{},
			NewSlogLogger(),
		},
	}

	// Must not panic.
	ml.Info(context.Background(), "fan out test")
	ml.Error(context.Background(), "fan out test")

	// nopLogger.Enabled=false but SlogLogger.Enabled(INFO)=true → true
	if !ml.Enabled(LevelInfo) {
		t.Error("MultiLogger.Enabled should return true when any logger is enabled")
	}

	// With must return a non-nil MultiLogger.
	child := ml.With("module", "test")
	if child == nil {
		t.Fatal("MultiLogger.With returned nil")
	}
	child.Info(context.Background(), "child fan out")
}

func TestMultiLogger_AllNop_EnabledFalse(t *testing.T) {
	ml := MultiLogger{
		Loggers: []Logger{nopLogger{}, nopLogger{}},
	}
	if ml.Enabled(LevelError) {
		t.Error("MultiLogger with all nopLoggers should never be enabled")
	}
}
