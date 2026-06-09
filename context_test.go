package wind

import (
	"context"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// BUG-2 regression: WithTraceID (and WithMetadata) must deep-copy the
// existing metadata map before writing, so that concurrent goroutines
// sharing the same parent context do not race on the underlying map.
// ---------------------------------------------------------------------------

func TestWithMetadata_DeepCopyDoesNotMutateParent(t *testing.T) {
	parent := WithMetadata(context.Background(), HeaderTraceID, "trace-1")

	child := WithMetadata(parent, HeaderUserID, "user-1")

	// The child must have both keys.
	if got := GetMetadata(child, HeaderTraceID); got != "trace-1" {
		t.Fatalf("child lost parent's trace ID: got %q", got)
	}
	if got := GetMetadata(child, HeaderUserID); got != "user-1" {
		t.Fatalf("child's user ID missing: got %q", got)
	}

	// The parent must NOT have the child's key (proves deep-copy).
	if got := GetMetadata(parent, HeaderUserID); got != "" {
		t.Fatalf("parent was mutated by child's write: got %q (BUG-2 regression)", got)
	}
	// The parent's original key must be untouched.
	if got := GetMetadata(parent, HeaderTraceID); got != "trace-1" {
		t.Fatalf("parent's trace ID was corrupted: got %q", got)
	}
}

func TestWithTraceID_DeepCopyDoesNotMutateParent(t *testing.T) {
	parent := WithTraceID(context.Background(), "trace-1")
	_ = WithTraceID(parent, "trace-2")

	if got := GetTraceID(parent); got != "trace-1" {
		t.Fatalf("parent trace ID was mutated: got %q (BUG-2 regression)", got)
	}
}

func TestWithUserID_AndGetUserID(t *testing.T) {
	ctx := WithUserID(context.Background(), "user-42")
	if got := GetUserID(ctx); got != "user-42" {
		t.Fatalf("expected user-42, got %q", got)
	}
	if got := GetUserID(context.Background()); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestWithColorTag_AndGetColorTag(t *testing.T) {
	ctx := WithColorTag(context.Background(), "blue")
	if got := GetColorTag(ctx); got != "blue" {
		t.Fatalf("expected blue, got %q", got)
	}
	if got := GetColorTag(context.Background()); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestWithMetadata_ConcurrentSafe(t *testing.T) {
	// All goroutines derive from the same parent; each writes a different key.
	// If WithMetadata did NOT deep-copy, this would panic or produce a data race.
	parent := WithMetadata(context.Background(), HeaderTraceID, "trace-0")

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = WithMetadata(parent, HeaderUserID, "user-"+itoa(i))
		}(i)
	}
	wg.Wait()

	// Parent must remain unchanged.
	if got := GetMetadata(parent, HeaderUserID); got != "" {
		t.Fatalf("parent was mutated by concurrent children: got %q (BUG-2 regression)", got)
	}
}

// itoa is a minimal int-to-string helper to avoid importing strconv in tests.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
