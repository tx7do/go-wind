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

// ---------------------------------------------------------------------------
// NewMetadataContext must deep-copy the input map so that subsequent mutations
// by the caller do not affect the value stored in the context.
// ---------------------------------------------------------------------------

func TestNewMetadataContext_DeepCopiesInput(t *testing.T) {
	md := Metadata{HeaderTraceID: "trace-1"}
	ctx := NewMetadataContext(context.Background(), md)

	// Mutate the original map after creating the context.
	md[HeaderUserID] = "user-999"

	// The context must NOT see the mutation.
	if got := GetMetadata(ctx, HeaderUserID); got != "" {
		t.Fatalf("context saw post-creation mutation of input map: got %q", got)
	}
	if got := GetMetadata(ctx, HeaderTraceID); got != "trace-1" {
		t.Fatalf("original value corrupted: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// WithMetadatas: batch merge with single deep-copy.
// ---------------------------------------------------------------------------

func TestWithMetadatas_BatchSet(t *testing.T) {
	ctx := WithMetadatas(context.Background(), Metadata{
		HeaderTraceID:  "trace-1",
		HeaderUserID:   "user-42",
		HeaderColorTag: "blue",
	})

	if got := GetTraceID(ctx); got != "trace-1" {
		t.Fatalf("trace ID: expected trace-1, got %q", got)
	}
	if got := GetUserID(ctx); got != "user-42" {
		t.Fatalf("user ID: expected user-42, got %q", got)
	}
	if got := GetColorTag(ctx); got != "blue" {
		t.Fatalf("color tag: expected blue, got %q", got)
	}
}

func TestWithMetadatas_MergesWithExisting(t *testing.T) {
	parent := WithTraceID(context.Background(), "trace-1")

	child := WithMetadatas(parent, Metadata{
		HeaderUserID:   "user-42",
		HeaderColorTag: "blue",
	})

	// Child must have inherited parent's key.
	if got := GetTraceID(child); got != "trace-1" {
		t.Fatalf("child lost parent's trace ID: got %q", got)
	}
	// Child must have the new keys.
	if got := GetUserID(child); got != "user-42" {
		t.Fatalf("child's user ID missing: got %q", got)
	}
	// Parent must NOT have the child's keys (proves deep-copy).
	if got := GetUserID(parent); got != "" {
		t.Fatalf("parent was mutated by child's merge: got %q", got)
	}
}

func TestWithMetadatas_DoesNotMutateInputMap(t *testing.T) {
	extra := Metadata{HeaderTraceID: "trace-1"}
	_ = WithMetadatas(context.Background(), extra)

	// The input map must be untouched.
	if len(extra) != 1 || extra[HeaderTraceID] != "trace-1" {
		t.Fatalf("input map was mutated: %v", extra)
	}
}

func TestWithMetadatas_ConcurrentSafe(t *testing.T) {
	parent := WithMetadata(context.Background(), HeaderTraceID, "trace-0")

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = WithMetadatas(parent, Metadata{HeaderUserID: "user-" + itoa(i)})
		}(i)
	}
	wg.Wait()

	if got := GetMetadata(parent, HeaderUserID); got != "" {
		t.Fatalf("parent was mutated by concurrent children: got %q", got)
	}
}

func TestWithMetadatas_EmptyInputReturnsCtxUnchanged(t *testing.T) {
	bg := context.Background()
	result := WithMetadatas(bg, nil)

	// No metadata present, so the context should be unchanged.
	if result != bg {
		t.Fatal("expected ctx unchanged when no existing metadata and empty extra")
	}
	if _, ok := MetadataFromContext(result); ok {
		t.Fatal("expected no metadata in context")
	}
}

// ---------------------------------------------------------------------------
// WithoutMetadata: remove a key from the context's Metadata.
// ---------------------------------------------------------------------------

func TestWithoutMetadata_RemovesKey(t *testing.T) {
	ctx := WithMetadatas(context.Background(), Metadata{
		HeaderTraceID: "trace-1",
		HeaderUserID:  "user-42",
	})

	result := WithoutMetadata(ctx, HeaderTraceID)

	// The key must be gone.
	if got := GetTraceID(result); got != "" {
		t.Fatalf("expected empty trace ID after removal, got %q", got)
	}
	// Other keys must survive.
	if got := GetUserID(result); got != "user-42" {
		t.Fatalf("user ID should survive, got %q", got)
	}
}

func TestWithoutMetadata_DoesNotMutateParent(t *testing.T) {
	parent := WithMetadatas(context.Background(), Metadata{
		HeaderTraceID: "trace-1",
		HeaderUserID:  "user-42",
	})

	_ = WithoutMetadata(parent, HeaderTraceID)

	// The parent context must remain intact.
	if got := GetTraceID(parent); got != "trace-1" {
		t.Fatalf("parent was mutated by child's delete: got %q", got)
	}
	if got := GetUserID(parent); got != "user-42" {
		t.Fatalf("parent's user ID was corrupted: got %q", got)
	}
}

func TestWithoutMetadata_KeyAbsentReturnsCtxUnchanged(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-1")

	// Removing a key that does not exist should return ctx unchanged.
	result := WithoutMetadata(ctx, HeaderUserID)
	if result != ctx {
		t.Fatal("expected ctx unchanged when key is absent")
	}
}

func TestWithoutMetadata_NoMetadataReturnsCtxUnchanged(t *testing.T) {
	bg := context.Background()

	// Removing from a context with no metadata should return ctx unchanged.
	result := WithoutMetadata(bg, HeaderTraceID)
	if result != bg {
		t.Fatal("expected ctx unchanged when no metadata exists")
	}
}

func TestWithoutMetadata_ConcurrentSafe(t *testing.T) {
	parent := WithMetadatas(context.Background(), Metadata{
		HeaderTraceID: "trace-0",
		HeaderUserID:  "user-0",
	})

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n * 2)
	// Half the goroutines delete TraceID, half delete UserID.
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = WithoutMetadata(parent, HeaderTraceID)
		}()
		go func() {
			defer wg.Done()
			_ = WithoutMetadata(parent, HeaderUserID)
		}()
	}
	wg.Wait()

	// Parent must remain intact.
	if got := GetTraceID(parent); got != "trace-0" {
		t.Fatalf("parent trace ID corrupted: got %q", got)
	}
	if got := GetUserID(parent); got != "user-0" {
		t.Fatalf("parent user ID corrupted: got %q", got)
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
