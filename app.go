// Package wind provides a minimalist microservice framework following a
// composable (Lego-like) design philosophy. The core [App] manages server
// lifecycles, while registration, logging and instance assembly are left
// entirely to the caller.
//
// This is NOT a battery-included framework. Each subsystem (transport,
// log) exposes only interfaces and helper types so that callers mix and
// match implementations as needed.
package wind

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/tx7do/go-wind/log"
	"github.com/tx7do/go-wind/transport"
)

// Option configures an [*App] via functional options.
type Option func(*App)

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
	runErr    error
}

// options holds the fully-resolved configuration for an [App]. All fields are
// unexported; callers set them through [Option] functions and read them back
// through the [App] accessor methods.
type options struct {
	id      string
	name    string
	version string

	logger log.Logger

	sigs        []os.Signal
	stopTimeout time.Duration

	beforeStop []func(ctx context.Context) error
	afterStop  []func(ctx context.Context) error

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

// WithLogger sets an app-specific [log.Logger]. If not set, [App.Logger]
// falls back to the package-level global logger ([log.GetLogger]).
// This allows callers to give each [*App] instance its own logger without
// affecting the global state.
func WithLogger(l log.Logger) Option {
	return func(o *App) { o.opts.logger = l }
}

// WithBeforeStop registers a callback invoked BEFORE any server's Stop is
// called during graceful shutdown. Typical uses include deregistering from
// a service registry, draining an incoming-request queue, or writing a
// final health-check ping.
//
// Multiple callbacks are executed in registration order. If any callback
// returns an error, the error is logged but shutdown continues.
func WithBeforeStop(fn func(ctx context.Context) error) Option {
	return func(o *App) { o.opts.beforeStop = append(o.opts.beforeStop, fn) }
}

// WithAfterStop registers a callback invoked AFTER all servers have stopped.
// Typical uses include closing database connections, flushing log buffers,
// or releasing other resources.
//
// Multiple callbacks are executed in registration order. If any callback
// returns an error, the error is logged but the error is not returned from
// Run (the servers have already stopped successfully).
func WithAfterStop(fn func(ctx context.Context) error) Option {
	return func(o *App) { o.opts.afterStop = append(o.opts.afterStop, fn) }
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

// Logger returns the app-specific logger set via [WithLogger]. If no logger
// was set, it falls back to the package-level global logger ([log.GetLogger]).
func (a *App) Logger() log.Logger {
	if a.opts.logger != nil {
		return a.opts.logger
	}
	return log.GetLogger()
}

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

// Err returns the error that caused [Run] to exit. It must be called after
// [Done] is closed; calling it before returns nil.
//
// This complements [Done] by allowing external supervisors to observe both
// the termination and the outcome without wrapping [Run] in their own
// goroutine:
//
//	<-app.Done()
//	if err := app.Err(); err != nil { ... }
func (a *App) Err() error {
	select {
	case <-a.done:
	default:
		return nil
	}
	return a.runErr
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

	// Start all servers concurrently in an errgroup. Only Start goroutines
	// and the signal watcher are in this group. Stop watchers and lifecycle
	// hooks are handled in separate phases below to guarantee correct
	// ordering: BeforeStop → Server.Stop → AfterStop.
	for _, srv := range a.opts.servers {
		srv := srv
		// Start the server. If Start returns an error, errgroup cancels egCtx
		// (BUG-3). If Start returns nil (server self-exited), we explicitly
		// trigger a full shutdown so other servers also stop (ISSUE-3).
		eg.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("wind: server panicked during Start (endpoint %s): %v", srv.Endpoint(), r)
					a.Logger().Error(egCtx, "server panicked during Start",
						"endpoint", srv.Endpoint(), "panic", r)
				}
			}()
			err = srv.Start(egCtx)
			if err == nil {
				a.Logger().Info(egCtx, "server self-exited, triggering shutdown",
					"endpoint", srv.Endpoint())
				a.triggerCancel()
			} else if !errors.Is(err, context.Canceled) {
				a.Logger().Error(egCtx, "server crashed",
					"endpoint", srv.Endpoint(), "error", err)
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
			// and triggers all Start goroutines to return.
			a.triggerCancel()
		}
		return nil
	})

	// Phase 1 complete: wait for all Start goroutines and the signal watcher
	// to return (i.e. shutdown has been triggered).
	startErr := eg.Wait()

	// Phase 2: BeforeStop hooks — synchronous, BEFORE any server is stopped.
	// Each hook gets a FRESH timeout context created at this point, not at
	// Run start (BUG fix: previously the context timed out during normal
	// operation and hooks always saw an expired deadline).
	for _, fn := range a.opts.beforeStop {
		hookCtx, hookCancel := context.WithTimeout(context.Background(), a.opts.stopTimeout)
		if err := a.runHookSafely(hookCtx, "beforeStop", fn); err != nil {
			a.Logger().Warn(hookCtx, "beforeStop hook error", "error", err)
		}
		hookCancel()
	}

	// Phase 3: Server.Stop — concurrent, each with its own timeout context
	// derived from context.Background(). We must NOT derive from runCtx /
	// egCtx, because cancel() would immediately invalidate it, rendering
	// stopTimeout useless (BUG-1 regression guard).
	//
	// firstStopErr captures the first non-nil error from any server's Stop
	// call (FINDING-4). Since Stop is now outside the errgroup, we capture
	// errors directly rather than relying on errgroup's first-error-wins.
	var firstStopErr error
	var stopErrOnce sync.Once
	var stopWg sync.WaitGroup
	for _, srv := range a.opts.servers {
		srv := srv
		stopWg.Add(1)
		go func() {
			defer stopWg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicErr := fmt.Errorf("wind: server panicked during Stop (endpoint %s): %v", srv.Endpoint(), r)
					a.Logger().Error(context.Background(), "server panicked during Stop",
						"endpoint", srv.Endpoint(), "panic", r)
					stopErrOnce.Do(func() { firstStopErr = panicErr })
				}
			}()
			stopCtx, stopCancel := context.WithTimeout(context.Background(), a.opts.stopTimeout)
			defer stopCancel()
			if err := srv.Stop(stopCtx); err != nil {
				a.Logger().Error(stopCtx, "server stop failed",
					"endpoint", srv.Endpoint(), "error", err)
				stopErrOnce.Do(func() { firstStopErr = err })
			}
		}()
	}
	stopWg.Wait()

	// Phase 4: AfterStop hooks — synchronous, AFTER all servers have stopped.
	for _, fn := range a.opts.afterStop {
		hookCtx, hookCancel := context.WithTimeout(context.Background(), a.opts.stopTimeout)
		if err := a.runHookSafely(hookCtx, "afterStop", fn); err != nil {
			a.Logger().Warn(hookCtx, "afterStop hook error", "error", err)
		}
		hookCancel()
	}

	// Determine the final error to return and store for [Err].
	var runErr error
	if startErr != nil && !errors.Is(startErr, context.Canceled) {
		runErr = startErr
	} else if firstStopErr != nil {
		runErr = firstStopErr
	}

	// Store the error before signaling completion so [Err] can read it
	// safely after [Done] is closed (happens-before via channel close).
	a.runErr = runErr
	a.closeOnce.Do(func() { close(a.done) })
	return runErr
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

// runHookSafely executes a lifecycle hook, recovering from panics and
// converting them to errors. This ensures that a panicking hook does not
// skip subsequent hooks or the Server.Stop phase. If the Server or hook
// needs its own panic handling (e.g. custom recovery), it may add an inner
// defer-recover which takes precedence over this safety net.
func (a *App) runHookSafely(ctx context.Context, name string, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("wind: %s hook panicked: %v", name, r)
			a.Logger().Error(ctx, "hook panicked", "hook", name, "panic", r)
		}
	}()
	return fn(ctx)
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
