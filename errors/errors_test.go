package errors

import (
	"errors"
	stderrors "errors"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// New should initialize fields.
// ---------------------------------------------------------------------------

func TestNew_InitializesFields(t *testing.T) {
	e := New(CodeNotFound, "ORDER_NOT_FOUND")
	if e.Code != CodeNotFound {
		t.Errorf("Code = %d, want %d", e.Code, CodeNotFound)
	}
	if e.Reason != "ORDER_NOT_FOUND" {
		t.Errorf("Reason = %q, want %q", e.Reason, "ORDER_NOT_FOUND")
	}
	if e.Details != nil {
		t.Errorf("Details = %v, want nil", e.Details)
	}
	if e.cause != nil {
		t.Errorf("cause = %v, want nil", e.cause)
	}
}

// ---------------------------------------------------------------------------
// Chainable builders must be immutable: original is left untouched.
// ---------------------------------------------------------------------------

func TestWithDetails_IsImmutable(t *testing.T) {
	base := New(CodeInvalidArgument, "VALIDATION_FAILED")
	derived := base.WithDetails(map[string]string{"field": "email"})

	// Derived carries the new details.
	if got := derived.Details["field"]; got != "email" {
		t.Errorf("derived.Details[field] = %q, want %q", got, "email")
	}
	// Original must NOT be polluted.
	if base.Details != nil {
		t.Errorf("base.Details = %v, want nil (immutability broken)", base.Details)
	}
}

func TestWithCause_IsImmutable(t *testing.T) {
	base := New(CodeInternal, "DB_DOWN")
	root := stderrors.New("connection refused")
	derived := base.WithCause(root)

	if !errors.Is(root, derived.cause) {
		t.Errorf("derived.cause = %v, want %v", derived.cause, root)
	}
	if base.cause != nil {
		t.Errorf("base.cause = %v, want nil (immutability broken)", base.cause)
	}
}

func TestWithCode_IsImmutable(t *testing.T) {
	base := New(CodeInternal, "MIGRATION_FAILED")
	derived := base.WithCode(CodeFailedPrecondition)

	if derived.Code != CodeFailedPrecondition {
		t.Errorf("derived.Code = %d, want %d", derived.Code, CodeFailedPrecondition)
	}
	if base.Code != CodeInternal {
		t.Errorf("base.Code = %d, want %d (immutability broken)", base.Code, CodeInternal)
	}
}

func TestBuilders_AreChainable(t *testing.T) {
	e := New(CodeNotFound, "USER_NOT_FOUND").
		WithDetails(map[string]string{"id": "42"}).
		WithCause(stderrors.New("redis miss"))

	if e.Code != CodeNotFound || e.Details["id"] != "42" || e.cause == nil {
		t.Errorf("chained builder did not compose: %+v", e)
	}
}

// ---------------------------------------------------------------------------
// Is method matches by Reason only.
// ---------------------------------------------------------------------------

func TestWindError_Is_MatchesByReason(t *testing.T) {
	a := New(CodeNotFound, "ORDER_NOT_FOUND")
	b := New(CodeInternal, "ORDER_NOT_FOUND") // different code, same reason

	if !a.Is(b) {
		t.Error("a.Is(b) = false, want true (Reason match)")
	}
}

func TestWindError_Is_DifferentReason(t *testing.T) {
	a := New(CodeNotFound, "ORDER_NOT_FOUND")
	b := New(CodeNotFound, "USER_NOT_FOUND")

	if a.Is(b) {
		t.Error("a.Is(b) = true, want false (different Reason)")
	}
}

func TestWindError_Is_NonWindTarget(t *testing.T) {
	a := New(CodeNotFound, "ORDER_NOT_FOUND")
	if a.Is(stderrors.New("plain")) {
		t.Error("a.Is(plainError) = true, want false")
	}
}

// ---------------------------------------------------------------------------
// Package-level Is / As / Unwrap / Join must bridge to stdlib and unwrap
// the cause chain.
// ---------------------------------------------------------------------------

func TestPackage_Is_TraversesCauseChain(t *testing.T) {
	target := NotFound("ORDER_NOT_FOUND")
	wrapped := fmt.Errorf("wrap: %w", target.WithCause(stderrors.New("db")))

	// Package-level errors.Is must walk the chain and hit WindError.Is (Reason match).
	if !Is(wrapped, target) {
		t.Error("Is(wrapped, target) = false, want true")
	}
}

func TestPackage_As_UnwrapsWindError(t *testing.T) {
	// This is the call shape used by HTTP/gRPC transport layers to unwrap an
	// underlying error into a *WindError, and the reason the As bridge exists.
	target := Internal("DB_DOWN")
	wrapped := fmt.Errorf("outer: %w", target)

	var got *WindError
	if !As(wrapped, &got) {
		t.Fatal("As(wrapped, &got) = false, want true")
	}
	if got.Reason != "DB_DOWN" || got.Code != CodeInternal {
		t.Errorf("As result = %+v, want Reason=DB_DOWN Code=Internal", got)
	}
}

func TestPackage_As_NotAWindError(t *testing.T) {
	var got *WindError
	if As(stderrors.New("plain"), &got) {
		t.Error("As(plainError) = true, want false")
	}
}

// Regression: As must pass `target` straight through, NOT `&target`. The buggy
// `stderrors.As(err, &target)` form panics with "target must be a non-nil
// pointer" because target is already the caller's *T pointer wrapped in an any.
func TestPackage_As_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("As panicked: %v", r)
		}
	}()
	var got *WindError
	_ = As(Internal("X"), &got)
}

func TestPackage_Unwrap_BridgesToStdlib(t *testing.T) {
	root := stderrors.New("root")
	wrapped := Internal("DB_DOWN").WithCause(root)

	if got := Unwrap(wrapped); !errors.Is(got, root) {
		t.Errorf("Unwrap = %v, want %v", got, root)
	}
}

func TestPackage_Join_BridgesToStdlib(t *testing.T) {
	a := stderrors.New("a")
	b := stderrors.New("b")
	joined := Join(a, b)
	if joined == nil {
		t.Fatal("Join returned nil")
	}
	// Joined error must match its members by identity (stderrors.Is on Join's
	// multi-error walks the members).
	if !Is(joined, a) {
		t.Error("Is(joined, a) = false, want true")
	}
	if !Is(joined, b) {
		t.Error("Is(joined, b) = false, want true")
	}
}

// ---------------------------------------------------------------------------
// FromError: typed convenience counterpart of As.
// ---------------------------------------------------------------------------

func TestFromError_ExtractsWindError(t *testing.T) {
	base := NotFound("ORDER_NOT_FOUND").
		WithDetails(map[string]string{"id": "42"}).
		WithCause(stderrors.New("db"))
	// Wrapped the way real call sites (middleware) see it.
	wrapped := fmt.Errorf("outer: %w", base)

	got, ok := FromError(wrapped)
	if !ok {
		t.Fatal("FromError returned ok=false, want true")
	}
	if got.Reason != "ORDER_NOT_FOUND" {
		t.Errorf("Reason = %q, want ORDER_NOT_FOUND", got.Reason)
	}
	if got.Code != CodeNotFound {
		t.Errorf("Code = %d, want %d", got.Code, CodeNotFound)
	}
	if got.Details["id"] != "42" {
		t.Errorf("Details[id] = %q, want 42", got.Details["id"])
	}
	// Must return the exact instance so chain inspection (cause) stays intact.
	if got.Unwrap() == nil {
		t.Error("extracted error lost its cause chain")
	}
}

func TestFromError_NilInput(t *testing.T) {
	got, ok := FromError(nil)
	if ok || got != nil {
		t.Errorf("FromError(nil) = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestFromError_PlainError(t *testing.T) {
	got, ok := FromError(stderrors.New("plain"))
	if ok || got != nil {
		t.Errorf("FromError(plain) = (%v, %v), want (nil, false)", got, ok)
	}
}

// ---------------------------------------------------------------------------
// WindError.Unwrap method returns the cause.
// ---------------------------------------------------------------------------

func TestWindError_Unwrap_ReturnsCause(t *testing.T) {
	root := stderrors.New("root")
	e := Internal("DB_DOWN").WithCause(root)

	if !errors.Is(root, e.Unwrap()) {
		t.Error("Unwrap() did not return the wrapped cause")
	}
}

func TestWindError_Unwrap_NilCause(t *testing.T) {
	e := New(CodeInternal, "X")
	if e.Unwrap() != nil {
		t.Error("Unwrap() on nil cause did not return nil")
	}
}

// ---------------------------------------------------------------------------
// Error() string formatting.
// ---------------------------------------------------------------------------

func TestError_FormatWithoutCause(t *testing.T) {
	e := New(CodeNotFound, "ORDER_NOT_FOUND")
	want := "code: 5, reason: ORDER_NOT_FOUND"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestError_FormatWithCause(t *testing.T) {
	e := New(CodeInternal, "DB_DOWN").WithCause(stderrors.New("connection refused"))
	got := e.Error()
	if !contains(got, "cause: connection refused") {
		t.Errorf("Error() = %q, missing cause segment", got)
	}
	if !contains(got, "reason: DB_DOWN") {
		t.Errorf("Error() = %q, missing reason segment", got)
	}
}

// ---------------------------------------------------------------------------
// StackTrace: non-empty after a builder, and idempotent across builders.
// ---------------------------------------------------------------------------

func TestStackTrace_NonEmptyAndContainsCaller(t *testing.T) {
	e := Internal("X").WithCause(stderrors.New("boom"))
	st := e.StackTrace()
	if st == "" {
		t.Fatal("StackTrace() is empty after WithCause")
	}
	// Stack frames include this test function's file.
	if !contains(st, "errors_test.go") {
		t.Errorf("StackTrace() = %q, does not contain test file", st)
	}
}

func TestStackTrace_NotCapturedOnBaseNew(t *testing.T) {
	// A bare New() does not capture a stack; only builders do.
	e := New(CodeInternal, "X")
	if e.StackTrace() != "" {
		t.Errorf("StackTrace() = %q, want empty for bare New", e.StackTrace())
	}
}

func TestCaptureStack_IdempotentAcrossBuilders(t *testing.T) {
	// First builder captures stack; chaining another must not re-capture.
	e := Internal("X").WithCause(stderrors.New("first"))
	first := e.StackTrace()
	second := e.WithCause(stderrors.New("second")).StackTrace()

	if first != second {
		t.Error("stack was re-captured on second builder; expected idempotent")
	}
}

// ---------------------------------------------------------------------------
// Factory functions return the correct Code for each transport category.
// ---------------------------------------------------------------------------

func TestFactoryFunctions(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) *WindError
		code uint32
	}{
		{"BadRequest", BadRequest, CodeInvalidArgument},
		{"NotFound", NotFound, CodeNotFound},
		{"AlreadyExists", AlreadyExists, CodeAlreadyExists},
		{"PermissionDenied", PermissionDenied, CodePermissionDenied},
		{"ResourceExhausted", ResourceExhausted, CodeResourceExhausted},
		{"FailedPrecondition", FailedPrecondition, CodeFailedPrecondition},
		{"Unauthenticated", Unauthenticated, CodeUnauthenticated},
		{"Internal", Internal, CodeInternal},
		{"Unimplemented", Unimplemented, CodeUnimplemented},
		{"Unavailable", Unavailable, CodeUnavailable},
		{"DeadlineExceeded", DeadlineExceeded, CodeDeadlineExceeded},
	}
	for _, tt := range tests {
		e := tt.fn("REASON_" + tt.name)
		if e.Code != tt.code {
			t.Errorf("%s().Code = %d, want %d", tt.name, e.Code, tt.code)
		}
		if e.Reason != "REASON_"+tt.name {
			t.Errorf("%s().Reason = %q, want %q", tt.name, e.Reason, "REASON_"+tt.name)
		}
	}
}

// ---------------------------------------------------------------------------
// Code constants must align with gRPC codes.Code values (no accidental drift).
// ---------------------------------------------------------------------------

func TestCodeConstants_AlignWithGRPC(t *testing.T) {
	// gRPC codes: https://grpc.io/docs/guides/status-codes/
	tests := []struct {
		code uint32
		want uint32
	}{
		{CodeOK, 0},
		{CodeCanceled, 1},
		{CodeUnknown, 2},
		{CodeInvalidArgument, 3},
		{CodeDeadlineExceeded, 4},
		{CodeNotFound, 5},
		{CodeAlreadyExists, 6},
		{CodePermissionDenied, 7},
		{CodeResourceExhausted, 8},
		{CodeFailedPrecondition, 9},
		{CodeAborted, 10},
		{CodeOutOfRange, 11},
		{CodeUnimplemented, 12},
		{CodeInternal, 13},
		{CodeUnavailable, 14},
		{CodeDataLoss, 15},
		{CodeUnauthenticated, 16},
	}
	for _, tt := range tests {
		if tt.code != tt.want {
			t.Errorf("code drift: got %d, want %d", tt.code, tt.want)
		}
	}
}

// contains is a tiny local helper to avoid pulling in strings just for tests.
func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
