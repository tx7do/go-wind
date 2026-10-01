package wind

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestBeforeStartHookAbort verifies that a failing beforeStart hook aborts
// startup: no server is started and Run returns the hook error.
func TestBeforeStartHookAbort(t *testing.T) {
	srv := newMockServer("srv")
	hookErr := errors.New("lock acquisition failed")

	app := New(
		WithServer(srv),
		WithBeforeStart(func(ctx context.Context) error { return hookErr }),
	)

	runErr := make(chan error, 1)
	go func() { runErr <- app.Run(context.Background()) }()

	select {
	case err := <-runErr:
		if !errors.Is(err, hookErr) {
			t.Fatalf("expected hook error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after beforeStart hook failure")
	}

	if srv.startCalled.Load() {
		t.Fatal("server must not start when beforeStart hook fails")
	}
	if !errors.Is(app.Err(), hookErr) {
		t.Fatalf("Err() must expose the hook error, got %v", app.Err())
	}
}

// TestAfterStartHookOrder verifies that beforeStart and afterStart hooks run
// in registration order, that afterStart runs after the server Start has been
// invoked, and that the shutdown phases still observe beforeStop/afterStop.
func TestAfterStartHookOrder(t *testing.T) {
	var order []string

	srv := newMockServer("srv")
	// Stop the app once afterStart has fired.
	var app *App
	app = New(
		WithServer(srv),
		WithBeforeStart(func(ctx context.Context) error {
			order = append(order, "beforeStart")
			return nil
		}),
		WithAfterStart(func(ctx context.Context) error {
			order = append(order, "afterStart")
			go func() {
				_ = app.Stop(context.Background())
			}()
			return nil
		}),
		WithBeforeStop(func(ctx context.Context) error {
			order = append(order, "beforeStop")
			return nil
		}),
		WithAfterStop(func(ctx context.Context) error {
			order = append(order, "afterStop")
			return nil
		}),
	)

	runErr := make(chan error, 1)
	go func() { runErr <- app.Run(context.Background()) }()

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not finish in time")
	}

	want := []string{"beforeStart", "afterStart", "beforeStop", "afterStop"}
	if len(order) != len(want) {
		t.Fatalf("hook order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("hook order = %v, want %v", order, want)
		}
	}
}

// TestAfterStartHookFailureTriggersShutdown verifies that a failing afterStart
// hook triggers graceful shutdown and its error is returned from Run.
func TestAfterStartHookFailureTriggersShutdown(t *testing.T) {
	srv := newMockServer("srv")
	hookErr := errors.New("registry unavailable")

	app := New(
		WithServer(srv),
		WithAfterStart(func(ctx context.Context) error { return hookErr }),
	)

	runErr := make(chan error, 1)
	go func() { runErr <- app.Run(context.Background()) }()

	select {
	case err := <-runErr:
		if !errors.Is(err, hookErr) {
			t.Fatalf("expected afterStart hook error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after afterStart hook failure")
	}

	if !srv.stopCalled.Load() {
		t.Fatal("server must be stopped after afterStart hook failure")
	}
}
