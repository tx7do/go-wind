package wind

import (
	"context"
	"sync"
	"sync/atomic"
)

// mockServer is a test double for transport.Server. It records every call and
// allows tests to synchronize on Start/Stop without sleeping, as well as
// inspect the context handed to Stop (BUG-1 / BUG-3 regression coverage).
type mockServer struct {
	name     string
	endpoint string

	// startErr, if non-nil, is returned immediately by Start without blocking,
	// simulating a server crash (BUG-3 scenario).
	startErr error
	// selfExit, if true, makes Start return nil immediately, simulating a
	// server that decides to exit on its own without error (ISSUE-3 scenario).
	selfExit bool
	// stopErr, if non-nil, is returned by Stop.
	stopErr error

	startCalled atomic.Bool
	stopCalled  atomic.Bool
	// stopCount tracks the exact number of times Stop was invoked. This is
	// used to assert no double-Stop occurs (ISSUE-1 regression).
	stopCount atomic.Int32

	// started / stopped are closed the moment Start / Stop are invoked, so
	// tests can synchronize deterministically.
	started chan struct{}
	stopped chan struct{}

	// stopCtxErr captures ctx.Err() at the exact moment Stop is invoked.
	// We snapshot the error because the caller will cancel stopCtx via defer
	// after Stop returns — the reference alone would always read as cancelled.
	stopCtxErr error
	stopCtxMu  sync.Mutex
}

func newMockServer(name string) *mockServer {
	return &mockServer{
		name:     name,
		endpoint: "mock://" + name,
		started:  make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// withStartErr configures the server to return err from Start immediately,
// simulating a crash.
func (m *mockServer) withStartErr(err error) *mockServer {
	m.startErr = err
	return m
}

// withSelfExit configures Start to return nil immediately, simulating a server
// that exits on its own without error.
func (m *mockServer) withSelfExit() *mockServer {
	m.selfExit = true
	return m
}

// withEndpoint overrides the default mock endpoint string.
func (m *mockServer) withEndpoint(ep string) *mockServer {
	m.endpoint = ep
	return m
}

// withStopErr configures the server to return err from Stop.
func (m *mockServer) withStopErr(err error) *mockServer {
	m.stopErr = err
	return m
}

// Start implements transport.Server. It signals started, then either returns
// immediately (crash / self-exit) or blocks until ctx is cancelled (normal run).
func (m *mockServer) Start(ctx context.Context) error {
	m.startCalled.Store(true)
	close(m.started)
	if m.startErr != nil {
		return m.startErr
	}
	if m.selfExit {
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
}

// Stop implements transport.Server. It records the context so tests can verify
// the timeout window, then returns stopErr (if any).
func (m *mockServer) Stop(ctx context.Context) error {
	m.stopCalled.Store(true)
	m.stopCount.Add(1)
	m.stopCtxMu.Lock()
	m.stopCtxErr = ctx.Err() // snapshot now, before the caller cancels it
	m.stopCtxMu.Unlock()
	close(m.stopped)
	return m.stopErr
}

// Endpoint implements transport.Server, returning the mock endpoint.
func (m *mockServer) Endpoint() string {
	return m.endpoint
}

// stopContextErr returns ctx.Err() as it was at the exact moment Stop was
// called. This is the reliable way to assert BUG-1: the context must have
// been alive when Stop was invoked.
func (m *mockServer) stopContextErr() error {
	m.stopCtxMu.Lock()
	defer m.stopCtxMu.Unlock()
	return m.stopCtxErr
}
