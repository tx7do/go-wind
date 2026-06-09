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

// instanceKey is an unexported context key type used to store and retrieve
// an [*Instance] without collisions with other context values.
type instanceKey struct{}

// NewInstanceContext returns a copy of ctx with the given [*Instance] attached.
func NewInstanceContext(ctx context.Context, inst *Instance) context.Context {
	return context.WithValue(ctx, instanceKey{}, inst)
}

// FromInstanceContext extracts the [*Instance] from ctx, if present.
func FromInstanceContext(ctx context.Context) (*Instance, bool) {
	inst, ok := ctx.Value(instanceKey{}).(*Instance)
	return inst, ok
}
