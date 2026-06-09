package wind

import (
	"context"
	"errors"
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
