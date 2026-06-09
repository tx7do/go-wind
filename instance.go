package wind

import "context"

// Instance describes a single service instance registered with (or discovered
// from) a service registry. It carries the information a client needs to
// connect: identity, version, network endpoints and arbitrary metadata.
type Instance struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Endpoints []string          `json:"endpoints"`
	Metadata  map[string]string `json:"metadata"`
}

// FirstEndpoint returns the first endpoint URL or an empty string if the
// instance has no endpoints. This is a convenience for the common
// single-endpoint case.
func (i *Instance) FirstEndpoint() string {
	if len(i.Endpoints) > 0 {
		return i.Endpoints[0]
	}
	return ""
}

// instanceKey is an unexported context key type used to store and retrieve
// an [*Instance] without collisions with other context values.
type instanceKey struct{}

// WithInstance returns a copy of ctx with the given [*Instance] attached.
func WithInstance(ctx context.Context, inst *Instance) context.Context {
	return context.WithValue(ctx, instanceKey{}, inst)
}

// InstanceFromContext extracts the [*Instance] from ctx, if present.
func InstanceFromContext(ctx context.Context) (*Instance, bool) {
	inst, ok := ctx.Value(instanceKey{}).(*Instance)
	return inst, ok
}
