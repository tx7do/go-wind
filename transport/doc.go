// Package transport defines the core transport-layer abstractions for the
// go-wind framework.
//
// It deliberately contains no concrete transport implementations. Callers
// implement the [Server] interface to plug in gRPC, HTTP, or any other
// protocol, and pass those implementations to [wind.New]. This keeps the
// framework core decoupled from any specific RPC stack; projects bring their
// own *grpc.Server / *http.Server and wrap it in a type that satisfies Server.
//
// # The Server contract
//
// [Server] is intentionally minimal — three methods covering the full lifecycle
// of a single network listener:
//
//   - [Server.Start] begins accepting connections and MUST block until its
//     context is cancelled or an error occurs. The blocking contract is what
//     lets [wind.App] run many servers concurrently under a single errgroup.
//   - [Server.Stop] performs a graceful shutdown. The context it receives
//     carries a deadline that implementations should respect.
//   - [Server.Endpoint] returns the actual network address the server is
//     listening on. For servers bound to ":0" (OS-assigned port), this MUST
//     return the resolved address after Start begins accepting connections,
//     so callers can register it with a service registry.
//
// # A minimal implementation
//
//	type MyServer struct{}
//
//	func (s *MyServer) Start(ctx context.Context) error {
//	    <-ctx.Done()
//	    return ctx.Err()
//	}
//
//	func (s *MyServer) Stop(ctx context.Context) error {
//	    // perform graceful cleanup, honoring ctx.Deadline()
//	    return nil
//	}
//
//	func (s *MyServer) Endpoint() string {
//	    return "grpc://0.0.0.0:9000"
//	}
//
// Multiple servers are composed by passing them all to [wind.WithServer]:
//
//	app := wind.New(
//	    wind.WithName("gateway"),
//	    wind.WithServer(grpcServer, httpServer, wsServer),
//	)
//	// All three start concurrently and stop concurrently on signal.
//
// # Lifecycle ownership
//
// transport.Server does not manage its own goroutine or signal handling — that
// is exclusively the responsibility of [wind.App]. A Server's job is only to
// expose Start/Stop/Endpoint; the App orchestrates concurrent startup,
// crash-cascading, and graceful shutdown. See the package documentation of the
// root wind package for the full lifecycle model.
package transport
