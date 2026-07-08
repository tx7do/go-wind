package errors

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"runtime"
	"strconv"
)

// Standard-library compatibility bridge: this package is named errors, which
// shadows the standard library errors, so the most commonly used functions
// (errors.Is / errors.As / errors.Unwrap / errors.Join) are re-exported as
// package-level functions. Callers only need to import
// "github.com/tx7do/go-wind/errors" without aliasing the standard library.
//
// Notes:
//   - New is NOT re-exported here. The project's New has signature New(code,
//     reason), which clashes with the standard library's New(string); the
//     project semantics are preserved. Callers needing stdlib New should alias it.
//   - The Is / Unwrap methods on WindError and the package-level functions below
//     live in different namespaces and coexist without ambiguity. The standard
//     library's errors.Is / errors.As internally call back into the WindError
//     methods, so chain-based matching and unwrapping are correct.

// Is bridges the standard library errors.Is: reports whether err matches target
// along the error chain (Unwrap).
func Is(err, target error) bool { return stderrors.Is(err, target) }

// As bridges the standard library errors.As: finds the first error in err's
// chain assignable to *target. Call sites that need to unwrap an underlying
// error into a *WindError rely on this function.
func As(err error, target any) bool { return stderrors.As(err, target) }

// Unwrap bridges the standard library errors.Unwrap: returns the immediate
// cause of err.
func Unwrap(err error) error { return stderrors.Unwrap(err) }

// FromError extracts a *WindError from err's chain. Returns (nil, false) when
// err is nil or contains no WindError. It is the typed convenience counterpart
// of As: callers avoid the "var wErr *WindError; errors.As(...)" boilerplate.
func FromError(err error) (*WindError, bool) {
	if err == nil {
		return nil, false
	}
	var wErr *WindError
	// Uses the stdlib directly inside the package to avoid re-entering our own
	// exported bridge (which exists for external callers).
	if stderrors.As(err, &wErr) {
		return wErr, true
	}
	return nil, false
}

// Join bridges the standard library errors.Join: merges multiple errors into one.
func Join(errs ...error) error { return stderrors.Join(errs...) }

// WindError implements the native Go error interface.
type WindError struct {
	Code    uint32            `json:"-"`       // transport category (maps to gRPC codes.Code)
	Reason  string            `json:"reason"`  // domain-unique identifier (e.g. "ORDER_NOT_FOUND")
	Details map[string]string `json:"details"` // dynamic variables for i18n translation
	cause   error             `json:"-"`       // wrapped underlying error in the error chain
	stack   []uintptr         `json:"-"`       // call stack captured at the failing site
}

func New(code uint32, reason string) *WindError {
	return &WindError{
		Code:   code,
		Reason: reason,
	}
}

// Error implements the native error interface; tuned for log readability.
func (e *WindError) Error() string {
	var buf bytes.Buffer
	buf.WriteString("code: ")
	buf.WriteString(strconv.FormatUint(uint64(e.Code), 10))
	buf.WriteString(", reason: ")
	buf.WriteString(e.Reason)
	if e.cause != nil {
		buf.WriteString("; cause: ")
		buf.WriteString(e.cause.Error())
	}
	return buf.String()
}

// Is supports Go standard library errors.Is comparisons. Two WindErrors match
// when their Reason is equal; Code/Details/cause are intentionally ignored so
// business code can match on the stable domain identifier.
func (e *WindError) Is(target error) bool {
	t, ok := target.(*WindError)
	if !ok {
		return false
	}
	return e.Reason == t.Reason
}

// Unwrap supports Go standard library errors.Unwrap.
func (e *WindError) Unwrap() error {
	return e.cause
}

// WithDetails is a chainable builder: injects the i18n variables needed by the
// frontend and captures the current call stack.
func (e *WindError) WithDetails(details map[string]string) *WindError {
	err := e.clone()
	err.Details = details
	err.captureStack()
	return err
}

// WithCause is a chainable builder: wraps a native underlying error (e.g. a
// database error) and captures the current call stack.
func (e *WindError) WithCause(cause error) *WindError {
	err := e.clone()
	err.cause = cause
	err.captureStack()
	return err
}

// WithCode is an escape hatch: allows overriding the transport category in
// special business scenarios.
func (e *WindError) WithCode(code uint32) *WindError {
	err := e.clone()
	err.Code = code
	return err
}

func (e *WindError) clone() *WindError {
	// Details is shallow-copied (shares the underlying map): WithDetails replaces
	// the whole map rather than mutating individual keys, so the original is never
	// polluted by the clone, making shallow copy safe and allocation-free.
	// If a method that mutates Details in place is added in the future, switch to
	// a deep copy to avoid data races on the shared map.
	return &WindError{
		Code:    e.Code,
		Reason:  e.Reason,
		Details: e.Details,
		cause:   e.cause,
		stack:   e.stack,
	}
}

func (e *WindError) captureStack() {
	if e.stack != nil {
		return
	}
	var pcs [32]uintptr
	// Skip captureStack and the WithDetails/WithCause caller frame.
	n := runtime.Callers(3, pcs[:])
	e.stack = pcs[:n]
}

// StackTrace formats the captured call stack as a string, for log collectors.
func (e *WindError) StackTrace() string {
	if e.stack == nil {
		return ""
	}
	var buf bytes.Buffer
	frames := runtime.CallersFrames(e.stack)
	for {
		frame, more := frames.Next()
		buf.WriteString(fmt.Sprintf("\n\t%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}
	return buf.String()
}
