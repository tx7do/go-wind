package log

import "context"

// nopLogger discards every log record. It is the zero-cost default used until
// the caller injects a real Logger via [SetLogger].
type nopLogger struct{}

func (nopLogger) Debug(context.Context, string, ...any) {}
func (nopLogger) Info(context.Context, string, ...any)  {}
func (nopLogger) Warn(context.Context, string, ...any)  {}
func (nopLogger) Error(context.Context, string, ...any) {}

// With returns a new nopLogger. Since nopLogger discards all output, the
// attached key-value pairs have no effect but the method exists to satisfy
// the Logger interface.
func (nopLogger) With(...any) Logger { return nopLogger{} }

// Compile-time assertion: nopLogger implements Logger.
var _ Logger = nopLogger{}
