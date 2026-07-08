// Package errors provides a structured, transport-aware error model for the
// go-wind framework. It is the single source of truth for business errors that
// must flow across service boundaries (gRPC, HTTP) while remaining compatible
// with the Go standard library errors package.
//
// # Error model
//
// A [WindError] carries three pieces of information that together describe any
// business failure:
//
//   - Code    — the transport category (a uint32 that maps 1:1 to gRPC
//     codes.Code). It decides the HTTP status and gRPC status code the error is
//     transported as. Declared as uint32 (not codes.Code) so this core package
//     stays free of any gRPC dependency; the HTTP conversion is provided
//     in-package by [CodeToHTTP] / [HTTPToCode] (returning plain int, no
//     net/http dependency), and the gRPC conversion is done by callers at the
//     transport boundary via codes.Code(wErr.Code).
//   - Reason  — a domain-unique, stable identifier such as "ORDER_NOT_FOUND".
//     This is the value business code matches on with [errors.Is]; it must be
//     treated as an immutable contract, never localized.
//   - Details — a map of dynamic variables (e.g. {"id": "42"}) consumed by the
//     frontend for i18n substitution.
//
// In addition, a WindError optionally wraps a cause (the underlying native
// error, e.g. a database driver error) and a captured call stack for log
// collectors.
//
// # Construction
//
// Prefer the category-specific factories over the generic [New]:
//
//	if order == nil {
//	    return errors.NotFound("ORDER_NOT_FOUND")
//	}
//
// The full set of factories: [BadRequest], [NotFound], [AlreadyExists],
// [PermissionDenied], [ResourceExhausted], [FailedPrecondition],
// [Unauthenticated], [Internal], [Unimplemented], [Unavailable],
// [DeadlineExceeded]. Each maps to a [Code] constant aligned with gRPC
// codes.Code; see types.go for the complete table.
//
// Use [New] only when you need a Code outside the common factories.
//
// # Chainable builders and immutability
//
// [WindError.WithDetails], [WindError.WithCause], and [WindError.WithCode]
// return a new instance; the receiver is never mutated. This makes it safe to
// keep sentinel errors as package-level vars and derive specific instances at
// call sites:
//
//	var ErrOrderNotFound = errors.NotFound("ORDER_NOT_FOUND")
//
//	// Safe: returns a NEW instance, ErrOrderNotFound stays pristine.
//	return ErrOrderNotFound.WithDetails(map[string]string{"id": id})
//
// # HTTP mapping
//
// The package ships a built-in, zero-dependency mapping between Code and HTTP
// status codes, so HTTP servers do not need an external translation layer:
//
//	w.WriteHeader(errors.CodeToHTTP(wErr.Code))
//
// [CodeToHTTP] returns the HTTP status for a Code (unknown codes fall back to
// 500, never a misleading 2xx). The mapping follows the canonical
// grpc-httpjson-transcoding / Envoy table and covers all 17 codes.
//
// [HTTPToCode] is the reverse direction. Because Code→HTTP is many-to-one
// (e.g. InvalidArgument, FailedPrecondition, and OutOfRange all map to 400),
// the reverse returns the single most representative Code; unmapped HTTP
// statuses fall back to [CodeUnknown].
//
// Both functions return plain int / uint32 — the errors package deliberately
// does not import net/http, mirroring the same layering rule that keeps Code
// decoupled from gRPC's codes.Code.
//
// # Standard library compatibility
//
// This package is named errors, which shadows the standard library. The most
// common helpers are re-exported as package-level functions so callers can
// import a single package: [Is], [As], [Unwrap], [Join]. These bridge straight
// to the standard library and honor the WindError methods below.
//
// WindError implements:
//
//   - [WindError.Is]  — two WindErrors match when their Reason is equal.
//     [errors.Is] walks the cause chain, so a wrapped WindError still matches
//     its sentinel by Reason.
//   - [WindError.Unwrap] — returns the cause, enabling [errors.As] and
//     [errors.Is] traversal.
//
// For the typed convenience counterpart of [As], use [FromError] to obtain a
// *WindError and a bool in one call, avoiding the two-line
// "var wErr *WindError; errors.As(...)" boilerplate.
//
// # Stack traces
//
// The chainable builders ([WindError.WithDetails] and [WindError.WithCause])
// capture the call stack at the point they are invoked; [WindError.StackTrace]
// formats it for logging. A bare [New] does not capture a stack — capture
// happens lazily the first time a builder runs, and is idempotent across
// subsequent builders chained on the same instance.
//
// Note: New is NOT re-exported as a bridge because the project signature
// New(code, reason) clashes with the standard library's New(string). Callers
// needing the standard library New should alias the import.
package errors
