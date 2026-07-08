package errors

// This file provides bidirectional mapping between the transport-neutral Code
// (uint32, aligned with gRPC codes.Code) and HTTP status codes (as plain int).
//
// Design decision — why int and not http.StatusCode:
// The errors core package stays free of any net/http dependency, mirroring the
// same layering philosophy that declares Code as uint32 instead of codes.Code.
// Callers in an HTTP server apply the result at the transport boundary:
//
//	w.WriteHeader(errors.CodeToHTTP(wErr.Code))
//
// The Code→HTTP table follows the canonical mapping used by
// grpc-httpjson-transcoding, Envoy, and the Google API HTTP/JSON transcoding
// implementation. It is the authoritative full 17-entry table, in one-to-one
// correspondence with the Code constants in types.go.
//
// The reverse HTTP→Code direction is many-to-one (e.g. HTTP 400 may correspond
// to InvalidArgument, FailedPrecondition, or OutOfRange), so HTTPToCode returns
// the single most semantically representative Code for each status.

// codeToHTTP is the authoritative Code → HTTP status mapping.
// Source: grpc-httpjson-transcoding / Envoy / Google API transcoding.
var codeToHTTP = map[uint32]int{
	CodeOK:                 200, // OK
	CodeCanceled:           499, // Client Closed Request
	CodeUnknown:            500, // Internal Server Error
	CodeInvalidArgument:    400, // Bad Request
	CodeDeadlineExceeded:   504, // Gateway Timeout
	CodeNotFound:           404, // Not Found
	CodeAlreadyExists:      409, // Conflict
	CodePermissionDenied:   403, // Forbidden
	CodeResourceExhausted:  429, // Too Many Requests
	CodeFailedPrecondition: 400, // Bad Request
	CodeAborted:            409, // Conflict
	CodeOutOfRange:         400, // Bad Request
	CodeUnimplemented:      501, // Not Implemented
	CodeInternal:           500, // Internal Server Error
	CodeUnavailable:        503, // Service Unavailable
	CodeDataLoss:           500, // Internal Server Error
	CodeUnauthenticated:    401, // Unauthorized
}

// httpToCode is the reverse HTTP status → Code mapping. Because the forward
// direction is many-to-one, each HTTP status maps to the single most
// representative Code (e.g. 400 → InvalidArgument, even though
// FailedPrecondition and OutOfRange also map to 400).
var httpToCode = map[int]uint32{
	200: CodeOK,
	400: CodeInvalidArgument,
	401: CodeUnauthenticated,
	403: CodePermissionDenied,
	404: CodeNotFound,
	409: CodeAlreadyExists,
	429: CodeResourceExhausted,
	499: CodeCanceled,
	500: CodeInternal,
	501: CodeUnimplemented,
	503: CodeUnavailable,
	504: CodeDeadlineExceeded,
}

// CodeToHTTP returns the HTTP status code for the given Code. Unknown codes
// fall back to 500 (Internal Server Error) so that a malformed or
// future-defined code never accidentally produces a misleading 2xx.
func CodeToHTTP(code uint32) int {
	if httpStatus, ok := codeToHTTP[code]; ok {
		return httpStatus
	}
	return 500
}

// HTTPToCode returns the Code for the given HTTP status code. Because the
// Code→HTTP direction is many-to-one, the returned Code is the single most
// representative one for that status. Unmapped HTTP statuses (e.g. 418, 451)
// fall back to CodeUnknown so callers can detect that no precise mapping exists.
func HTTPToCode(httpStatus int) uint32 {
	if code, ok := httpToCode[httpStatus]; ok {
		return code
	}
	return CodeUnknown
}
