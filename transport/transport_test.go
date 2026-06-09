package transport

import (
	"context"
	"testing"
)

// --- compile-time interface assertions ---

type mockServer struct{}

func (mockServer) Start(context.Context) error { return nil }
func (mockServer) Stop(context.Context) error  { return nil }

var _ Server = mockServer{}

type mockTransporter struct{}

func (mockTransporter) Kind() string      { return "mock" }
func (mockTransporter) Endpoint() string  { return ":0" }
func (mockTransporter) Operation() string { return "" }

var _ Transporter = mockTransporter{}

// --- context propagation tests ---

func TestWithTransporter_RoundTrip(t *testing.T) {
	tr := mockTransporter{}
	ctx := WithTransporter(context.Background(), tr)

	got, ok := TransporterFromContext(ctx)
	if !ok {
		t.Fatal("TransporterFromContext returned ok=false")
	}
	if got.Kind() != "mock" {
		t.Fatalf("expected kind 'mock', got %q", got.Kind())
	}
}

func TestTransporterFromContext_EmptyContext(t *testing.T) {
	_, ok := TransporterFromContext(context.Background())
	if ok {
		t.Fatal("expected ok=false for empty context")
	}
}
