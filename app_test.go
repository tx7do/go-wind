package wind

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// waitFor blocks until ch is closed or the deadline elapses, failing the test
// on timeout. This keeps tests deterministic without arbitrary sleeps.
func waitFor(t *testing.T, name string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s did not happen within 3s", name)
	}
}

// ---------------------------------------------------------------------------
// BUG-1 regression: the context passed to Server.Stop during graceful shutdown
// must NOT be already cancelled — otherwise stopTimeout is meaningless.
// ---------------------------------------------------------------------------

func TestApp_Run_GracefulShutdown_StopContextIsValid(t *testing.T) {
	srv := newMockServer("srv-1")

	app := New(WithServer(srv))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	// Wait for the server to actually start before triggering shutdown.
	waitFor(t, "server start", srv.started)

	// Trigger graceful shutdown by cancelling the Run context.
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if !srv.stopCalled.Load() {
		t.Fatal("expected Stop to be called, but it was not")
	}

	// The crucial BUG-1 assertion: stopCtx must have been alive at the
	// moment Stop was called. We use the snapshot because the caller cancels
	// stopCtx via defer right after Stop returns.
	if err := srv.stopContextErr(); err != nil {
		t.Fatalf("Stop context was already cancelled at call time: %v — stopTimeout window is broken (BUG-1)", err)
	}
}

// ---------------------------------------------------------------------------
// BUG-3 regression: when one server crashes, every other server must receive
// a Stop call so the whole app exits cleanly.
// ---------------------------------------------------------------------------

func TestApp_Run_ServerCrash_StopsOtherServers(t *testing.T) {
	crashErr := errors.New("boom")
	// crashSrv returns startErr immediately, simulating a crash.
	crashSrv := newMockServer("crash").withStartErr(crashErr)
	// healthySrv blocks until its context is cancelled.
	healthySrv := newMockServer("healthy")

	app := New(WithServer(crashSrv, healthySrv))

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(context.Background()) }()

	// The crash error should propagate from Run.
	if err := <-errCh; !errors.Is(err, crashErr) {
		t.Fatalf("expected crash error %v, got %v", crashErr, err)
	}

	// The healthy server must have been stopped (BUG-3 fix).
	if !healthySrv.stopCalled.Load() {
		t.Fatal("healthy server was not stopped after crash — overall graceful exit broken (BUG-3)")
	}
}

// ---------------------------------------------------------------------------
// BUG-3 regression (Stop propagation): the stop context delivered to servers
// during a crash-induced shutdown must also be valid (BUG-1 applies here too).
// ---------------------------------------------------------------------------

func TestApp_Run_ServerCrash_StopContextIsValid(t *testing.T) {
	crashErr := errors.New("kaboom")
	crashSrv := newMockServer("crash").withStartErr(crashErr)
	healthySrv := newMockServer("healthy")

	app := New(WithServer(crashSrv, healthySrv))

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(context.Background()) }()

	if err := <-errCh; !errors.Is(err, crashErr) {
		t.Fatalf("expected crash error %v, got %v", crashErr, err)
	}

	waitFor(t, "healthy server stop", healthySrv.stopped)

	if err := healthySrv.stopContextErr(); err != nil {
		t.Fatalf("Stop context was already cancelled during crash shutdown: %v (BUG-1)", err)
	}
}

// ---------------------------------------------------------------------------
// Verify that multiple servers all start and all stop during a normal
// graceful shutdown.
// ---------------------------------------------------------------------------

func TestApp_Run_MultipleServers_AllStartAndStop(t *testing.T) {
	s1 := newMockServer("srv-1")
	s2 := newMockServer("srv-2")
	s3 := newMockServer("srv-3")

	app := New(WithServer(s1, s2, s3))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	waitFor(t, "srv-1 start", s1.started)
	waitFor(t, "srv-2 start", s2.started)
	waitFor(t, "srv-3 start", s3.started)

	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for _, s := range []*mockServer{s1, s2, s3} {
		if !s.startCalled.Load() {
			t.Errorf("%s was never started", s.name)
		}
		if !s.stopCalled.Load() {
			t.Errorf("%s was never stopped", s.name)
		}
	}
}

// ---------------------------------------------------------------------------
// ISSUE-1 regression: Stop() called from another goroutine while Run() is
// active must trigger exactly ONE Stop per server — never a double-Stop.
// ---------------------------------------------------------------------------

func TestApp_Stop_NoDoubleStop(t *testing.T) {
	s1 := newMockServer("srv-1")
	s2 := newMockServer("srv-2")

	app := New(WithServer(s1, s2))

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(context.Background()) }()

	waitFor(t, "srv-1 start", s1.started)
	waitFor(t, "srv-2 start", s2.started)

	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("Stop returned unexpected error: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for _, s := range []*mockServer{s1, s2} {
		if count := s.stopCount.Load(); count != 1 {
			t.Errorf("%s was stopped %d times, expected exactly 1 (ISSUE-1 double-Stop)", s.name, count)
		}
	}
}

// ---------------------------------------------------------------------------
// ISSUE-3 regression: when a server's Start returns nil (self-exit), the
// application must still shut down all other servers cleanly.
// ---------------------------------------------------------------------------

func TestApp_Run_ServerSelfExit_StopsOtherServers(t *testing.T) {
	// selfExitSrv returns nil from Start immediately, simulating a clean exit.
	selfExitSrv := newMockServer("self-exit").withSelfExit()
	healthySrv := newMockServer("healthy")

	app := New(WithServer(selfExitSrv, healthySrv))

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(context.Background()) }()

	// Run should return nil — self-exit is not an error.
	if err := <-errCh; err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The healthy server must have been stopped (ISSUE-3 fix).
	if !healthySrv.stopCalled.Load() {
		t.Fatal("healthy server was not stopped after self-exit — app hung indefinitely (ISSUE-3)")
	}
}

// ---------------------------------------------------------------------------
// FINDING-1 regression: calling Run more than once on the same [*App] must
// return [wind.ErrAppAlreadyRunning] instead of silently racing or corrupting
// internal state.
// ---------------------------------------------------------------------------

func TestApp_Run_DoubleRun_ReturnsErrAppAlreadyRunning(t *testing.T) {
	srv := newMockServer("srv-1")
	app := New(WithServer(srv))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	waitFor(t, "server start", srv.started)

	// Second Run must be rejected immediately.
	if err := app.Run(context.Background()); !errors.Is(err, ErrAppAlreadyRunning) {
		t.Fatalf("expected ErrAppAlreadyRunning, got %v (FINDING-1)", err)
	}

	// Clean up: trigger graceful shutdown.
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("first Run returned unexpected error: %v", err)
	}
}

func TestApp_Stop_GracefulShutdown(t *testing.T) {
	s1 := newMockServer("srv-1")
	s2 := newMockServer("srv-2")

	app := New(WithServer(s1, s2))

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(context.Background()) }()

	waitFor(t, "srv-1 start", s1.started)
	waitFor(t, "srv-2 start", s2.started)

	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("Stop returned unexpected error: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if !s1.stopCalled.Load() {
		t.Error("srv-1 was not stopped")
	}
	if !s2.stopCalled.Load() {
		t.Error("srv-2 was not stopped")
	}
}

// ---------------------------------------------------------------------------
// FINDING-3: when no servers are registered, Run must still return when the
// context is cancelled. This supports pure worker applications.
// ---------------------------------------------------------------------------

func TestApp_Run_ZeroServers_StopsOnCancel(t *testing.T) {
	app := New()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	// Give Run time to enter the blocking signal goroutine.
	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after ctx cancel (FINDING-3)")
	}
}

// ---------------------------------------------------------------------------
// FINDING-4: when a server's Stop returns an error, that error must
// propagate from Run — not be silently swallowed by errgroup's
// context.Canceled.
//
// Note: this test relies on a deterministic scheduling order: the Stop
// watcher goroutine reaches <-egCtx.Done() before the Start goroutine
// (Start must first execute Store + close before blocking on <-ctx.Done()).
// This ordering has been verified stable across 100+ runs at GOMAXPROCS=1.
// The firstStopErr / stopErrOnce mechanism in Run provides an additional
// safety net regardless of scheduling order.
// ---------------------------------------------------------------------------

func TestApp_Run_StopError_Propagates(t *testing.T) {
	stopErr := errors.New("stop failed")
	srv := newMockServer("srv-1").withStopErr(stopErr)

	app := New(WithServer(srv))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	waitFor(t, "server start", srv.started)

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, stopErr) {
			t.Fatalf("expected stop error %v, got %v (FINDING-4)", stopErr, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// ---------------------------------------------------------------------------
// Lifecycle hooks: WithBeforeStop and WithAfterStop must be called in the
// correct order relative to server Start/Stop.
// ---------------------------------------------------------------------------

func TestApp_Run_LifecycleHooks_Order(t *testing.T) {
	srv := newMockServer("srv-1")

	var beforeCalled atomic.Int32
	var afterCalled atomic.Int32

	app := New(
		WithServer(srv),
		WithBeforeStop(func(ctx context.Context) error {
			beforeCalled.Store(1)
			return nil
		}),
		WithAfterStop(func(ctx context.Context) error {
			// AfterStop should run after server Stop
			if !srv.stopCalled.Load() {
				t.Error("afterStop ran before server was stopped")
			}
			afterCalled.Store(1)
			return nil
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	waitFor(t, "server start", srv.started)

	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if beforeCalled.Load() != 1 {
		t.Error("beforeStop hook was not called")
	}
	if afterCalled.Load() != 1 {
		t.Error("afterStop hook was not called")
	}
}

// ---------------------------------------------------------------------------
// Ordering guarantee: BeforeStop hooks must complete BEFORE any server's
// Stop is called. The old design ran them concurrently inside the same
// errgroup, creating a race (BUG). This test blocks the hook until the test
// signals it to proceed, then asserts that Server.Stop has NOT been called
// while the hook was still blocking.
// ---------------------------------------------------------------------------

func TestApp_Run_BeforeStopRunsBeforeStop(t *testing.T) {
	srv := newMockServer("srv-1")

	beforeProceed := make(chan struct{})

	app := New(
		WithServer(srv),
		WithBeforeStop(func(ctx context.Context) error {
			// Block until the test signals us to proceed.
			<-beforeProceed
			return nil
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	waitFor(t, "server start", srv.started)

	cancel()

	// Give shutdown time to reach the BeforeStop hook. The hook is now
	// blocking, so Server.Stop should NOT have been called yet.
	time.Sleep(50 * time.Millisecond)

	select {
	case <-srv.stopped:
		t.Fatal("server Stop was called while beforeStop hook was still blocking (ordering BUG)")
	default:
		// Good: Stop hasn't been called yet because beforeStop is blocking.
	}

	// Release the hook; Run should now proceed to Phase 3 (Server.Stop).
	close(beforeProceed)

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// App.Err must return the same error that Run returned, after Done is closed.
// ---------------------------------------------------------------------------

func TestApp_Err_ReturnsRunError(t *testing.T) {
	crashErr := errors.New("crash")
	crashSrv := newMockServer("crash").withStartErr(crashErr)

	app := New(WithServer(crashSrv))

	runErrCh := make(chan error, 1)
	go func() { runErrCh <- app.Run(context.Background()) }()

	runErr := <-runErrCh

	// Wait for Done to close.
	<-app.Done()

	// Err should return the same error.
	if got := app.Err(); !errors.Is(got, crashErr) {
		t.Fatalf("Err() = %v, want %v", got, crashErr)
	}
	if !errors.Is(runErr, crashErr) {
		t.Fatalf("Run returned %v, want %v", runErr, crashErr)
	}
}

// ---------------------------------------------------------------------------
// App.Err must return nil before Run has completed.
// ---------------------------------------------------------------------------

func TestApp_Err_NilBeforeRunCompletes(t *testing.T) {
	app := New()
	if err := app.Err(); err != nil {
		t.Fatalf("Err() before Run should return nil, got %v", err)
	}
}
