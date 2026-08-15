package log

import (
	"context"
	"testing"
)

type globalCaptureLogger struct {
	lastLevel string
	lastMsg   string
	withArgs  []any
}

func (l *globalCaptureLogger) Debug(_ context.Context, msg string, _ ...any) { l.lastLevel, l.lastMsg = "DEBUG", msg }
func (l *globalCaptureLogger) Info(_ context.Context, msg string, _ ...any)  { l.lastLevel, l.lastMsg = "INFO", msg }
func (l *globalCaptureLogger) Warn(_ context.Context, msg string, _ ...any)  { l.lastLevel, l.lastMsg = "WARN", msg }
func (l *globalCaptureLogger) Error(_ context.Context, msg string, _ ...any) { l.lastLevel, l.lastMsg = "ERROR", msg }
func (l *globalCaptureLogger) Enabled(Level) bool                            { return true }
func (l *globalCaptureLogger) With(args ...any) Logger {
	return &globalCaptureLogger{withArgs: args}
}

func TestGlobalHelpers_LogAndEnabled(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	l := &globalCaptureLogger{}
	SetLogger(l)

	Info(context.Background(), "hello")
	if l.lastLevel != "INFO" || l.lastMsg != "hello" {
		t.Fatalf("unexpected log state: level=%q msg=%q", l.lastLevel, l.lastMsg)
	}

	if !Enabled(LevelDebug) {
		t.Fatal("Enabled(LevelDebug) = false, want true")
	}
}

func TestGlobalHelpersWith(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	SetLogger(&globalCaptureLogger{})
	child := With("module", "test")
	if child == nil {
		t.Fatal("With returned nil")
	}
}

