package transport

import "context"

// Transporter carries per-request transport metadata. It can be attached to
// a context via [WithTransporter] so that downstream handlers can inspect the
// transport kind, endpoint and operation name.
type Transporter interface {
	// Kind returns the transport type, e.g. "grpc" or "http".
	Kind() string
	// Endpoint returns the network endpoint, typically "host:port".
	Endpoint() string
	// Operation returns the fully-qualified operation name, e.g. the gRPC
	// method path "/package.Service/Method".
	Operation() string
}

// transporterKey is an unexported context key type used to store and retrieve
// a [Transporter] without collisions with other context values.
type transporterKey struct{}

// WithTransporter returns a copy of ctx with the given [Transporter] attached.
func WithTransporter(ctx context.Context, tr Transporter) context.Context {
	return context.WithValue(ctx, transporterKey{}, tr)
}

// TransporterFromContext extracts the [Transporter] from ctx, if present.
func TransporterFromContext(ctx context.Context) (Transporter, bool) {
	tr, ok := ctx.Value(transporterKey{}).(Transporter)
	return tr, ok
}
