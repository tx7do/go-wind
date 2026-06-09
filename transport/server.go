// Package transport defines the core transport-layer abstractions for the
// go-wind framework.
//
// It deliberately contains no concrete transport implementations. Callers
// implement the [Server] interface to plug in gRPC, HTTP, or any other
// protocol, and pass those implementations to [wind.New].
package transport

import "context"

// Server represents a single network server whose lifecycle is managed by
// the application. A typical implementation wraps a *grpc.Server,
// *http.Server, or similar, and blocks in Start until ctx is cancelled.
type Server interface {
	// Start begins accepting connections. It MUST block until ctx is
	// cancelled or an error occurs.
	Start(ctx context.Context) error
	// Stop performs a graceful shutdown. The provided ctx carries a deadline
	// that implementations should respect.
	Stop(ctx context.Context) error
}
