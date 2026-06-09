package wind_test

import (
	"context"
	"fmt"

	"github.com/tx7do/go-wind"
)

// ExampleNew demonstrates creating an [*wind.App] with composable options.
// The framework does not start any server until [App.Run] is called.
func ExampleNew() {
	app := wind.New(
		wind.WithID("svc-1"),
		wind.WithName("user-service"),
		wind.WithVersion("v1.0.0"),
	)

	fmt.Println("ID:", app.ID())
	fmt.Println("Name:", app.Name())
	fmt.Println("Version:", app.Version())

	// Output:
	// ID: svc-1
	// Name: user-service
	// Version: v1.0.0
}

// ExampleWithMetadata shows how to attach request-scoped metadata to a
// context and read it back. Each call deep-copies the map, so the parent
// context is never mutated (BUG-2 regression guard).
func ExampleWithMetadata() {
	ctx := context.Background()

	// Set a trace ID.
	ctx = wind.WithTraceID(ctx, "trace-abc")

	// Set a user ID on the same chain.
	ctx = wind.WithUserID(ctx, "user-42")

	fmt.Println(wind.GetTraceID(ctx))
	fmt.Println(wind.GetUserID(ctx))

	// Output:
	// trace-abc
	// user-42
}

// ExampleApp_Instance demonstrates building a [*wind.Instance] from the
// app's configured identity fields. This is a convenience helper — callers
// still choose whether and how to register the instance.
func ExampleApp_Instance() {
	app := wind.New(
		wind.WithID("svc-1"),
		wind.WithName("user-service"),
		wind.WithVersion("v1.0.0"),
	)

	inst := app.Instance("grpc://0.0.0.0:9000")

	fmt.Println(inst.ID, inst.Name, inst.Version)
	fmt.Println(inst.Endpoints[0])

	// Output:
	// svc-1 user-service v1.0.0
	// grpc://0.0.0.0:9000
}
