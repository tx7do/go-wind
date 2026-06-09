package transport

import (
	"context"
)

// --- compile-time interface assertions ---

type mockServer struct{}

func (mockServer) Start(context.Context) error { return nil }
func (mockServer) Stop(context.Context) error  { return nil }
func (mockServer) Endpoint() string            { return ":0" }

var _ Server = mockServer{}
