package errors

import "testing"

// ---------------------------------------------------------------------------
// CodeToHTTP: full 17-entry forward mapping (gRPC code → HTTP status).
// Source: grpc-httpjson-transcoding / Envoy / Google API transcoding.
// ---------------------------------------------------------------------------

func TestCodeToHTTP_FullMapping(t *testing.T) {
	tests := []struct {
		name string
		code uint32
		want int
	}{
		{"OK", CodeOK, 200},
		{"Canceled", CodeCanceled, 499},
		{"Unknown", CodeUnknown, 500},
		{"InvalidArgument", CodeInvalidArgument, 400},
		{"DeadlineExceeded", CodeDeadlineExceeded, 504},
		{"NotFound", CodeNotFound, 404},
		{"AlreadyExists", CodeAlreadyExists, 409},
		{"PermissionDenied", CodePermissionDenied, 403},
		{"ResourceExhausted", CodeResourceExhausted, 429},
		{"FailedPrecondition", CodeFailedPrecondition, 400},
		{"Aborted", CodeAborted, 409},
		{"OutOfRange", CodeOutOfRange, 400},
		{"Unimplemented", CodeUnimplemented, 501},
		{"Internal", CodeInternal, 500},
		{"Unavailable", CodeUnavailable, 503},
		{"DataLoss", CodeDataLoss, 500},
		{"Unauthenticated", CodeUnauthenticated, 401},
	}
	for _, tt := range tests {
		if got := CodeToHTTP(tt.code); got != tt.want {
			t.Errorf("CodeToHTTP(%s=%d) = %d, want %d", tt.name, tt.code, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// CodeToHTTP: unknown code must fall back to 500, never a misleading 2xx.
// ---------------------------------------------------------------------------

func TestCodeToHTTP_UnknownCodeFallsBackTo500(t *testing.T) {
	for _, unknown := range []uint32{17, 99, 1000, 0xFFFFFFFF} {
		if got := CodeToHTTP(unknown); got != 500 {
			t.Errorf("CodeToHTTP(%d) = %d, want 500 (fallback)", unknown, got)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTPToCode: key reverse mappings. The reverse is many-to-one, so each status
// maps to the single most representative Code.
// ---------------------------------------------------------------------------

func TestHTTPToCode_ReverseMapping(t *testing.T) {
	tests := []struct {
		httpStatus int
		want       uint32
	}{
		{200, CodeOK},
		{400, CodeInvalidArgument}, // representative of {InvalidArgument, FailedPrecondition, OutOfRange}
		{401, CodeUnauthenticated},
		{403, CodePermissionDenied},
		{404, CodeNotFound},
		{409, CodeAlreadyExists}, // representative of {AlreadyExists, Aborted}
		{429, CodeResourceExhausted},
		{499, CodeCanceled},
		{500, CodeInternal}, // representative of {Unknown, Internal, DataLoss}
		{501, CodeUnimplemented},
		{503, CodeUnavailable},
		{504, CodeDeadlineExceeded},
	}
	for _, tt := range tests {
		if got := HTTPToCode(tt.httpStatus); got != tt.want {
			t.Errorf("HTTPToCode(%d) = %d, want %d", tt.httpStatus, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTPToCode: unmapped HTTP statuses fall back to CodeUnknown.
// ---------------------------------------------------------------------------

func TestHTTPToCode_UnmappedFallsBackToUnknown(t *testing.T) {
	for _, unmapped := range []int{100, 302, 418, 451, 502, 505, 999} {
		if got := HTTPToCode(unmapped); got != CodeUnknown {
			t.Errorf("HTTPToCode(%d) = %d, want CodeUnknown(%d)", unmapped, got, CodeUnknown)
		}
	}
}

// ---------------------------------------------------------------------------
// Round-trip idempotency for single-shot codes.
//
// HTTPToCode(CodeToHTTP(c)) == c holds only for codes whose HTTP status is
// unique (single-shot). Codes sharing an HTTP status with another code (e.g.
// FailedPrecondition/OutOfRange → 400, Aborted → 409, Unknown/DataLoss → 500)
// are excluded because the reverse picks a different representative.
// ---------------------------------------------------------------------------

func TestRoundTrip_SingleShotCodes(t *testing.T) {
	excluded := map[uint32]bool{
		// These codes share an HTTP status with another code, so the reverse
		// mapping returns the representative peer, not themselves.
		CodeFailedPrecondition: true, // 400 → InvalidArgument
		CodeOutOfRange:         true, // 400 → InvalidArgument
		CodeAborted:            true, // 409 → AlreadyExists
		CodeUnknown:            true, // 500 → Internal
		CodeDataLoss:           true, // 500 → Internal
	}
	allCodes := []uint32{
		CodeOK, CodeCanceled, CodeUnknown, CodeInvalidArgument,
		CodeDeadlineExceeded, CodeNotFound, CodeAlreadyExists,
		CodePermissionDenied, CodeResourceExhausted, CodeFailedPrecondition,
		CodeAborted, CodeOutOfRange, CodeUnimplemented, CodeInternal,
		CodeUnavailable, CodeDataLoss, CodeUnauthenticated,
	}
	for _, c := range allCodes {
		if excluded[c] {
			continue
		}
		roundTrip := HTTPToCode(CodeToHTTP(c))
		if roundTrip != c {
			t.Errorf("round-trip failed for Code %d: CodeToHTTP→HTTPToCode = %d", c, roundTrip)
		}
	}
}
