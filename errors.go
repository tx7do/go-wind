package wind

import "errors"

// Common errors returned by the framework. Callers can use errors.Is to
// perform type-safe error handling.

// ErrAppAlreadyRunning is returned by [App.Run] when Run has already been
// called on the same [*App] instance. An [*App] is designed to be used once;
// create a new instance for each run.
var ErrAppAlreadyRunning = errors.New("wind: App.Run already called")
