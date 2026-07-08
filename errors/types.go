package errors

// Code is the standard transport-level error category; its values map exactly
// to gRPC's codes.Code. By declaring Code as uint32 instead of depending on
// codes.Code directly, the errors core package stays free of any gRPC
// dependency. HTTP status conversion is provided in-package by [CodeToHTTP] /
// [HTTPToCode] (returning int, no net/http dependency); the gRPC conversion is
// done by callers at the transport boundary via codes.Code(wErr.Code).
//
// Do NOT change these numeric values: they are kept in one-to-one correspondence
// with the gRPC protocol, the HTTP mapping table in http.go (see [CodeToHTTP]),
// and the errdetails.ErrorInfo semantics.
const (
	CodeOK                 uint32 = 0  // OK, success
	CodeCanceled           uint32 = 1  // CANCELED, caller cancelled the call
	CodeUnknown            uint32 = 2  // UNKNOWN, unknown error
	CodeInvalidArgument    uint32 = 3  // INVALID_ARGUMENT, invalid argument
	CodeDeadlineExceeded   uint32 = 4  // DEADLINE_EXCEEDED, deadline exceeded
	CodeNotFound           uint32 = 5  // NOT_FOUND, resource not found
	CodeAlreadyExists      uint32 = 6  // ALREADY_EXISTS, resource already exists
	CodePermissionDenied   uint32 = 7  // PERMISSION_DENIED, permission denied
	CodeResourceExhausted  uint32 = 8  // RESOURCE_EXHAUSTED, resource exhausted (rate limited)
	CodeFailedPrecondition uint32 = 9  // FAILED_PRECONDITION, precondition not met
	CodeAborted            uint32 = 10 // ABORTED, transaction aborted (usually retryable)
	CodeOutOfRange         uint32 = 11 // OUT_OF_RANGE, value out of range
	CodeUnimplemented      uint32 = 12 // UNIMPLEMENTED, not implemented
	CodeInternal           uint32 = 13 // INTERNAL, internal error
	CodeUnavailable        uint32 = 14 // UNAVAILABLE, service unavailable (retryable)
	CodeDataLoss           uint32 = 15 // DATA_LOSS, data loss
	CodeUnauthenticated    uint32 = 16 // UNAUTHENTICATED, not authenticated
)

// The convenience factory functions below cover the most common transport
// categories. Business code only needs to provide a domain-unique Reason to
// create a standard WindError in one line. To bind dynamic variables or wrap an
// underlying native error, chain WithDetails / WithCause.

// BadRequest creates an invalid-argument error (INVALID_ARGUMENT).
func BadRequest(reason string) *WindError { return New(CodeInvalidArgument, reason) }

// NotFound creates a resource-not-found error (NOT_FOUND).
func NotFound(reason string) *WindError { return New(CodeNotFound, reason) }

// AlreadyExists creates a resource-already-exists error (ALREADY_EXISTS).
func AlreadyExists(reason string) *WindError { return New(CodeAlreadyExists, reason) }

// PermissionDenied creates a permission-denied error (PERMISSION_DENIED).
func PermissionDenied(reason string) *WindError { return New(CodePermissionDenied, reason) }

// ResourceExhausted creates a resource-exhausted / rate-limited error (RESOURCE_EXHAUSTED).
func ResourceExhausted(reason string) *WindError { return New(CodeResourceExhausted, reason) }

// FailedPrecondition creates a failed-precondition error (FAILED_PRECONDITION).
func FailedPrecondition(reason string) *WindError { return New(CodeFailedPrecondition, reason) }

// Unauthenticated creates an unauthenticated error (UNAUTHENTICATED).
func Unauthenticated(reason string) *WindError { return New(CodeUnauthenticated, reason) }

// Internal creates an internal error (INTERNAL).
func Internal(reason string) *WindError { return New(CodeInternal, reason) }

// Unimplemented creates a not-implemented error (UNIMPLEMENTED).
func Unimplemented(reason string) *WindError { return New(CodeUnimplemented, reason) }

// Unavailable creates a service-unavailable error (UNAVAILABLE).
func Unavailable(reason string) *WindError { return New(CodeUnavailable, reason) }

// DeadlineExceeded creates a deadline-exceeded error (DEADLINE_EXCEEDED).
func DeadlineExceeded(reason string) *WindError { return New(CodeDeadlineExceeded, reason) }
