package wind

import "context"

// Standard header keys used to propagate request-scoped metadata across
// service boundaries. Callers may use these keys or define their own.
const (
	HeaderTraceID  = "x-wind-trace-id"
	HeaderUserID   = "x-wind-user-id"
	HeaderColorTag = "x-wind-color-tag"
)

// metadataKey is an unexported context key type used to store and retrieve
// [Metadata] without collisions with other context values.
type metadataKey struct{}

// Metadata is a simple string-keyed map carried through the context chain.
// It is the vehicle for trace IDs, user IDs, color tags and other
// request-scoped attributes.
type Metadata map[string]string

// NewContext returns a copy of ctx with the given [Metadata] attached.
func NewContext(ctx context.Context, md Metadata) context.Context {
	return context.WithValue(ctx, metadataKey{}, md)
}

// FromContext extracts the [Metadata] from ctx, if present.
func FromContext(ctx context.Context) (Metadata, bool) {
	md, ok := ctx.Value(metadataKey{}).(Metadata)
	return md, ok
}

// GetMetadata returns the value for key from the context's [Metadata].
// It returns an empty string when the key is absent or no metadata exists.
func GetMetadata(ctx context.Context, key string) string {
	if md, ok := FromContext(ctx); ok {
		return md[key]
	}
	return ""
}

// WithTraceID returns a new context with the given trace ID set in its
// [Metadata].
//
// The existing metadata map is deep-copied before modification to prevent
// concurrent write races when the parent context is shared across goroutines
// (BUG-2 regression guard).
func WithTraceID(ctx context.Context, traceID string) context.Context {
	md, ok := FromContext(ctx)
	if !ok {
		md = make(Metadata)
	} else {
		// Must copy to avoid mutating the parent context's map, which may be
		// shared by other goroutines (concurrent write data race — BUG-2).
		md = cloneMetadata(md)
	}
	md[HeaderTraceID] = traceID
	return NewContext(ctx, md)
}

// GetTraceID returns the trace ID from the context's [Metadata], or an empty
// string if none is set.
func GetTraceID(ctx context.Context) string {
	return GetMetadata(ctx, HeaderTraceID)
}

// cloneMetadata returns a deep copy of md. The returned map is always
// non-nil and safe to mutate independently of the original.
func cloneMetadata(md Metadata) Metadata {
	cp := make(Metadata, len(md))
	for k, v := range md {
		cp[k] = v
	}
	return cp
}
