package log

import "sync"

// Global logger state. The default is nopLogger so that importing the package
// is free of side effects; callers opt in to logging via [SetLogger].
var (
	globalLoggerMu sync.RWMutex
	globalLogger   Logger = nopLogger{}
)

// SetLogger sets the package-level [Logger] used by the framework. Pass nil
// to revert to the silent nopLogger.
//
// SetLogger is safe to call concurrently with log calls; callers that want to
// mutate the logger without races should hold their own coordination.
func SetLogger(l Logger) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	if l == nil {
		globalLogger = nopLogger{}
		return
	}
	globalLogger = l
}

// GetLogger returns the currently active package-level [Logger].
// It is the entry point submodules use to obtain the shared logger without
// taking a direct dependency on any logging framework.
func GetLogger() Logger {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()
	return globalLogger
}
