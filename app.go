// Package wind provides a minimalist microservice framework following a
// composable (Lego-like) design philosophy. The core [App] manages server
// lifecycles, while registration, configuration, logging and instance
// assembly are left entirely to the caller.
//
// This is NOT a battery-included framework. Each subsystem (transport,
// registry, config, log) exposes only interfaces and helper types so that
// callers mix and match implementations as needed.
package wind

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/tx7do/go-wind/transport"
)

// Option configures an [*App] via functional options.
type Option func(*App)

// ErrAppAlreadyRunning is returned by [App.Run] when Run has already been
// called on the same [*App] instance. An [*App] is designed to be used once;
// create a new instance for each run.
var ErrAppAlreadyRunning = errors.New("wind: App.Run already called")

// App is the central runtime that owns and manages the lifecycle of one or
// more [transport.Server] instances. It is intentionally free of any
// hard-coded integration — callers wire up servers, registries, loggers, etc.
// through the composable Option pattern.
type App struct {
	opts options

	mu     sync.Mutex
	cancel context.CancelFunc

	// running guards against [Run] being called more than once per
	// [*App] instance (FINDING-1). Once set to true it is never reset.
	running   atomic.Bool
	done      chan struct{}
	closeOnce sync.Once
}

// options holds the fully-resolved configuration for an [App]. All fields are
// unexported; callers set them through [Option] functions and read them back
// through the [App] accessor methods.
type options struct {
	id      string
	name    string
	version string

	sigs        []os.Signal
	stopTimeout time.Duration

	servers []transport.Server
}

// WithID sets the unique identifier of the application. It is typically
// used to construct an [Instance] for service registration.
func WithID(id string) Option {
	return func(o *App) { o.opts.id = id }
}

// WithName sets the human-readable name of the application.
func WithName(name string) Option {
	return func(o *App) { o.opts.name = name }
}

// WithVersion sets the semantic version of the application.
func WithVersion(version string) Option {
	return func(o *App) { o.opts.version = version }
}

// WithServer attaches one or more [transport.Server] instances to the [App].
// All servers are started concurrently when [App.Run] is called and stopped
// concurrently during graceful shutdown.
func WithServer(srv ...transport.Server) Option {
	return func(o *App) { o.opts.servers = append(o.opts.servers, srv...) }
}

// WithStopTimeout sets the maximum duration allowed for graceful shutdown.
// Each server's Stop call receives a context with this deadline. The default
// is 10 seconds.
func WithStopTimeout(d time.Duration) Option {
	return func(o *App) { o.opts.stopTimeout = d }
}

// WithSignal overrides the default set of OS signals that trigger graceful
// shutdown. By default the app listens for SIGTERM, SIGINT and SIGQUIT.
func WithSignal(sigs ...os.Signal) Option {
	return func(o *App) { o.opts.sigs = sigs }
}

// New creates an [*App] with the given options. Sensible defaults are applied:
//   - Listens for SIGTERM, SIGINT and SIGQUIT for graceful shutdown.
//   - A 10-second stop timeout is enforced during shutdown.
func New(opts ...Option) *App {
	o := options{
		sigs:        []os.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT},
		stopTimeout: 10 * time.Second,
	}
	app := &App{opts: o, done: make(chan struct{})}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// --- accessors ---------------------------------------------------------------
//
// These read-only getters let callers retrieve values set via Option during
// composable assembly, e.g. to build an [Instance] for registration or as
// log tags.

// ID returns the unique identifier set via [WithID].
func (a *App) ID() string { return a.opts.id }

// Name returns the application name set via [WithName].
func (a *App) Name() string { return a.opts.name }

// Version returns the application version set via [WithVersion].
func (a *App) Version() string { return a.opts.version }

// Instance builds an [*Instance] from the app's configured ID, Name and
// Version, plus the provided endpoint URLs. This is a convenience helper for
// callers who wish to register with a service registry — it does NOT perform
// any registration on its own (composable design: the caller chooses whether
// and how to register).
func (a *App) Instance(endpoints ...string) *Instance {
	return &Instance{
		ID:        a.opts.id,
		Name:      a.opts.name,
		Version:   a.opts.version,
		Endpoints: endpoints,
	}
}

// Done returns a channel that is closed when [Run] finishes — either after a
// normal graceful shutdown or after a server crash. It allows external
// supervisors to wait for the app to terminate without calling [Stop] or
// wrapping [Run] in their own error channel. Done is provided for
// read-only observation; it must not be closed by the caller.
//
// Before [Run] is called the channel is open (not closed).
func (a *App) Done() <-chan struct{} {
	return a.done
}

// Run starts the application and blocks until all servers have stopped.
//
// All registered servers are started concurrently inside an errgroup. The
// method returns when:
//   - A registered OS signal (SIGTERM/SIGINT/SIGQUIT) is received.
//   - The provided ctx is cancelled.
//   - Any server's Start returns an error (server crash) or nil (server
//     self-exit).
//
// On any of these triggers, every server receives a Stop call with a fresh
// context derived from context.Background() — NOT from the run context — so
// the configured stopTimeout is honoured even after a.cancel() fires.
//
// If no servers are registered, Run blocks until ctx is cancelled, a
// signal is received, or [Stop] is called. This is useful for pure worker
// applications that do not expose a network server but still want graceful
// shutdown.
//
// Run must be called at most once per [*App] instance. Calling Run a
// second time returns [ErrAppAlreadyRunning] immediately.
func (a *App) Run(ctx context.Context) error {
	if !a.running.CompareAndSwap(false, true) {
		return ErrAppAlreadyRunning
	}

	runCtx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()

	eg, egCtx := errgroup.WithContext(runCtx)

	// firstStopErr captures the first non-nil error from any server's Stop
	// call. errgroup only retains the first non-nil error across ALL
	// goroutines; when Start returns context.Canceled (which is filtered
	// below) before a Stop watcher finishes, Stop errors would be silently
	// lost. We capture them separately and surface them after eg.Wait()
	// (FINDING-4).
	var firstStopErr error
	var stopErrOnce sync.Once

	for _, srv := range a.opts.servers {
		srv := srv
		// Stop watcher: when egCtx is cancelled (signal received or a server
		// crashed / self-exited), stop the server with a fresh timeout context
		// derived from context.Background(). We must NOT derive from runCtx /
		// egCtx, because cancel() would immediately invalidate it, rendering
		// stopTimeout useless (BUG-1 regression guard).
		eg.Go(func() error {
			<-egCtx.Done()
			stopCtx, cancel := context.WithTimeout(context.Background(), a.opts.stopTimeout)
			defer cancel()
			err := srv.Stop(stopCtx)
			if err != nil {
				stopErrOnce.Do(func() { firstStopErr = err })
			}
			return err
		})
		// Start the server. If Start returns an error, errgroup cancels egCtx
		// (BUG-3). If Start returns nil (server self-exited), we explicitly
		// trigger a full shutdown so other servers also stop (ISSUE-3).
		eg.Go(func() error {
			err := srv.Start(egCtx)
			if err == nil {
				a.triggerCancel()
			}
			return err
		})
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, a.opts.sigs...)
	// ISSUE-4: stop relaying signals when Run exits to avoid leaking the
	// signal channel across multiple Run/Stop cycles.
	defer signal.Stop(c)

	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			// Triggered by a server crash or external cancellation; nothing to do.
		case <-c:
			// Signal received: cancel the main context, which cascades to egCtx
			// and triggers all servers' Stop watchers.
			a.triggerCancel()
		}
		return nil
	})

	err := eg.Wait()

	// Signal that shutdown is complete so [Stop] can return.
	a.closeOnce.Do(func() { close(a.done) })

	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	// The errgroup error was nil or context.Canceled; surface any Stop error
	// that would otherwise be swallowed (FINDING-4).
	if firstStopErr != nil {
		return firstStopErr
	}
	return nil
}

// Stop gracefully stops the application by cancelling the main context and
// waiting for all registered servers to finish shutting down.
//
// Stop does NOT call Server.Stop directly — it only triggers cancellation and
// lets the Stop watchers inside [Run] perform the actual shutdown. This avoids
// double-Stop when Stop is called concurrently with an active Run (ISSUE-1).
//
// Stop must be called from a different goroutine than [Run]. If Run has not
// been started, Stop blocks until ctx is done (there is nothing to stop).
func (a *App) Stop(ctx context.Context) error {
	a.triggerCancel()

	select {
	case <-a.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// triggerCancel safely calls a.cancel if it has been set by [Run]. Safe to
// call from any goroutine, including before [Run] is called.
func (a *App) triggerCancel() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
}
