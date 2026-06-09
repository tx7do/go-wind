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

// NewMetadataContext returns a copy of ctx with the given [Metadata] attached.
func NewMetadataContext(ctx context.Context, md Metadata) context.Context {
	return context.WithValue(ctx, metadataKey{}, md)
}

// MetadataFromContext extracts the [Metadata] from ctx, if present.
func MetadataFromContext(ctx context.Context) (Metadata, bool) {
	md, ok := ctx.Value(metadataKey{}).(Metadata)
	return md, ok
}

// GetMetadata returns the value for key from the context's [Metadata].
// It returns an empty string when the key is absent or no metadata exists.
func GetMetadata(ctx context.Context, key string) string {
	if md, ok := MetadataFromContext(ctx); ok {
		return md[key]
	}
	return ""
}

// WithMetadata returns a new context with the given key/value pair set in
// its [Metadata].
//
// The existing metadata map is deep-copied before modification to prevent
// concurrent write races when the parent context is shared across goroutines
// (BUG-2 regression guard).
func WithMetadata(ctx context.Context, key, value string) context.Context {
	md, ok := MetadataFromContext(ctx)
	if !ok {
		md = make(Metadata)
	} else {
		// Must copy to avoid mutating the parent context's map, which may be
		// shared by other goroutines (concurrent write data race — BUG-2).
		md = cloneMetadata(md)
	}
	md[key] = value
	return NewMetadataContext(ctx, md)
}

// WithTraceID returns a new context with the given trace ID set in its
// [Metadata]. It is a convenience wrapper around [WithMetadata].
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return WithMetadata(ctx, HeaderTraceID, traceID)
}

// GetTraceID returns the trace ID from the context's [Metadata], or an empty
// string if none is set.
func GetTraceID(ctx context.Context) string {
	return GetMetadata(ctx, HeaderTraceID)
}

// WithUserID returns a new context with the given user ID set in its
// [Metadata]. It is a convenience wrapper around [WithMetadata].
func WithUserID(ctx context.Context, userID string) context.Context {
	return WithMetadata(ctx, HeaderUserID, userID)
}

// GetUserID returns the user ID from the context's [Metadata], or an empty
// string if none is set.
func GetUserID(ctx context.Context) string {
	return GetMetadata(ctx, HeaderUserID)
}

// WithColorTag returns a new context with the given color tag set in its
// [Metadata]. It is a convenience wrapper around [WithMetadata].
func WithColorTag(ctx context.Context, tag string) context.Context {
	return WithMetadata(ctx, HeaderColorTag, tag)
}

// GetColorTag returns the color tag from the context's [Metadata], or an
// empty string if none is set.
func GetColorTag(ctx context.Context) string {
	return GetMetadata(ctx, HeaderColorTag)
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
