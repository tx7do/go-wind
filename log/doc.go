// Package log provides a minimal, backend-agnostic logging interface for the
// go-wind framework.
//
// The package intentionally defines only an interface contract and a global
// registry — it ships no concrete logging backend. This keeps the framework
// core free of any logging dependency; projects adapt their own backend
// (the stdlib *slog.Logger, zap, zerolog, kratos log, …) with a few lines of
// glue code. A zero-cost [nopLogger] is included as the silent default so that
// importing the package has no side effects; concrete adapters (slog adapter,
// LevelFilter, MultiLogger) live in go-wind-plugins.
//
// # The Logger interface
//
// [Logger] is deliberately small — 4 log methods ([Logger.Debug],
// [Logger.Info], [Logger.Warn], [Logger.Error]) plus [Logger.Enabled] and
// [Logger.With] — so adapting any backend is trivial:
//
//	type zapAdapter struct{ l *zap.Logger }
//	func (z zapAdapter) Info(ctx context.Context, msg string, args ...any) { … }
//	// … implement Debug/Warn/Error/Enabled/With likewise
//
// The first argument is always a [context.Context] so backends that support it
// can extract trace ids or request-scoped attributes. Callers with no context
// at hand may pass nil.
//
// # Injecting a backend
//
// Use [SetLogger] to install the package-level logger used by the framework,
// and [GetLogger] to read it. Both are safe to call concurrently with log
// calls. Pass nil to [SetLogger] to revert to the silent nopLogger:
//
//	windlog.SetLogger(myZapAdapter{})
//	defer windlog.SetLogger(nil)
//
// Submodules obtain the shared logger via [GetLogger] without taking a direct
// dependency on any logging framework.
//
// # Guarding expensive arguments
//
// [Logger.Enabled] lets callers avoid constructing expensive log arguments when
// the level would be discarded:
//
//	if logger.Enabled(log.LevelDebug) {
//	    logger.Debug(ctx, "detail", computeExpensiveData())
//	}
//
// [Level] enumerates the standard severities (LevelDebug … LevelError) and is
// the argument type for [Logger.Enabled].
//
// # Attaching context to a logger
//
// [Logger.With] returns a new Logger with key-value pairs permanently attached,
// typically used to tag a module or request:
//
//	moduleLogger := logger.With("module", "registry")
//	moduleLogger.Info(ctx, "started")
package log
