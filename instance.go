package wind

// Instance describes a single service instance registered with (or discovered
// from) a service registry. It carries the information a client needs to
// connect: identity, version, network endpoints and arbitrary metadata.
//
// App.Instance returns a populated [*Instance] for the caller to use with
// their chosen registry implementation in go-wind-plugins.
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
