<div align="center">

# Go Wind

### A Minimalist, Composable Microservice Framework for Go

Lego-like Architecture · Interface-Driven · Zero Magic · Production-Ready

[中文](./README.md) · English · [日本語](./README_ja.md)

</div>

---

## Design Philosophy

> **Not a bundled framework — a box of building blocks.**

go-wind embraces the native Go philosophy: **composition over inheritance, interfaces over implementations.** The framework defines only protocols and lifecycle scaffolding — no infrastructure is hard-wired. Every module (transport, registry, config, logging) exposes a minimal interface; callers assemble them like Lego bricks.

| Bundled Frameworks | go-wind |
|:---:|:---:|
| Locks you into gRPC + etcd + zap | You pick gRPC or HTTP — you decide |
| Framework owns everything | Framework owns only lifecycle |
| Upgrading = upgrading the whole stack | Upgrading = upgrading the skeleton |
| Steep learning curve | Read the source in 5 minutes |

---

## Key Features

- **Composable Assembly** — Four modules (transport / registry / config / logging) are fully interface-based with zero hard-coded dependencies
- **Graceful Lifecycle** — Signal-aware, timeout-controlled server start/stop; a single server crash cascades a full graceful shutdown
- **Non-Intrusive Context** — TraceID / UserID / ColorTag propagated via context with deep-copy to prevent data races
- **Minimal Log Facade** — 4-method interface + `With`; adapt slog / zap / zerolog / kratos log in a few lines
- **Functional Options** — `WithServer`, `WithName`… chainable, type-safe, readable configuration
- **Zero External Dependencies** — Only `golang.org/x/sync`; the framework itself is under 500 lines

---

## Quick Start

### Installation

```bash
go get github.com/tx7do/go-wind
```

### Minimal Example

```go
package main

import (
    "context"
    "log"

    wind "github.com/tx7do/go-wind"
    "github.com/tx7do/go-wind/transport"
)

// MyServer implements transport.Server
type MyServer struct{}

func (s *MyServer) Start(ctx context.Context) error {
    <-ctx.Done()
    return ctx.Err()
}

func (s *MyServer) Stop(ctx context.Context) error {
    // Perform cleanup (ctx carries a timeout)
    return nil
}

func main() {
    app := wind.New(
        wind.WithID("order-service-01"),
        wind.WithName("order-service"),
        wind.WithVersion("v1.0.0"),
        wind.WithServer(&MyServer{}),
    )

    if err := app.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### Multiple Servers

```go
app := wind.New(
    wind.WithName("gateway"),
    wind.WithServer(grpcServer, httpServer, wsServer),
)

// All three servers start concurrently and stop gracefully on signal
app.Run(ctx)
```

### Composable Registry Assembly

```go
app := wind.New(
    wind.WithName("user-service"),
    wind.WithServer(grpcServer),
)

// Registry, logging, config — all assembled by you; the framework makes no assumptions
inst := app.Instance("grpc://0.0.0.0:9000")

// Use your chosen registry implementation
registrar.Register(ctx, inst)
defer registrar.Deregister(ctx, inst)

app.Run(ctx)
```

### Logging Integration

```go
import windlog "github.com/tx7do/go-wind/log"

// Use the built-in slog adapter
windlog.SetLogger(windlog.NewSlogLogger())

// Level filtering: only WARN and above
filtered := windlog.LevelFilter{
    Logger: windlog.NewSlogLogger(),
    Level:  windlog.LevelWarn,
}
windlog.SetLogger(filtered)

// Or adapt your own logging backend
windlog.SetLogger(myZapAdapter{})
```

### Advanced Configuration

```go
app := wind.New(
    wind.WithServer(grpcServer),
    wind.WithStopTimeout(30*time.Second),  // custom graceful shutdown timeout
    wind.WithSignal(syscall.SIGTERM),       // custom signal set
    wind.WithLogger(myLogger),              // app-level logger
)

// App-level logger; falls back to the global logger if not set
app.Logger().Info(ctx, "starting")

// Wait for the app to finish (useful for external supervision)
<-app.Done()
```

---

## Module Architecture

```mermaid
graph LR
    APP["wind.App<br/>Lifecycle orchestration"]
    APP -->|"manages"| SERVER["transport.Server<br/>interface contract"]
    APP -.->|"caller assembles"| REG["registry<br/>Registrar / Discovery"]
    APP -.->|"caller assembles"| CFG["config<br/>Reader / Watcher"]
    APP -.->|"caller assembles"| LOG["log<br/>Logger"]
```

> Dashed lines indicate modules the framework does NOT hard-wire — callers assemble them freely.

```text
go-wind/
├── app.go              Core engine: App lifecycle management
├── context.go          Request-scoped metadata propagation (TraceID / UserID / Metadata)
├── instance.go         Service instance model & context binding
├── transport/          Transport abstraction (Server / Transporter)
├── registry/           Service registration & discovery abstraction (Registrar / Discovery)
├── config/             Config source abstraction (Reader / ReadCloser / Watcher / ValueWatcher)
└── log/                Log facade (Logger interface + LevelFilter + slog adapter + nop impl)
```

### Module Overview

| Module | Core Interfaces | Responsibility |
|:---|:---|:---|
| `wind` | `App`, `Option` | Application lifecycle orchestration, graceful shutdown |
| `wind` | `Instance` | Service instance modeling, context propagation |
| `wind` | `Metadata` | Request-scoped metadata (TraceID etc.) propagation |
| `transport` | `Server`, `Transporter` | Transport-layer abstraction for any protocol |
| `registry` | `Registrar`, `Discovery`, `Watcher` | Service registration, discovery & change watch |
| `config` | `Reader`, `ReadCloser`, `Watcher`, `ValueWatcher` | Config loading & hot-reload watching |
| `log` | `Logger`, `LevelFilter` | Logging facade adaptable to any backend; level-filter wrapper |

---

## Lifecycle & Graceful Shutdown

The core capability of go-wind is **reliable application lifecycle management**:

```mermaid
graph TB
    S1["srv.Start"] --> EG
    S2["srv.Start"] --> EG
    S3["srv.Start"] --> EG
    EG["errgroup (egCtx)"]

    SIG["Signal received<br/>SIGTERM / SIGINT / SIGQUIT"] --> TC
    CTX["ctx cancelled"] --> TC
    CRASH["Server crash / exit"] --> TC

    TC["triggerCancel()"] -->|"cancels"| EG

    EG --> Stop1["srv.Stop<br/>independent timeout ctx"]
    EG --> Stop2["srv.Stop<br/>independent timeout ctx"]
    EG --> Stop3["srv.Stop<br/>independent timeout ctx"]
```

**Design Highlights:**

| Mechanism | Description |
|:---|:---|
| Independent stop context | The Stop context is derived from `context.Background()` — **not** from the run context — ensuring the timeout window is real |
| Crash cascade | Any server crash or self-exit automatically triggers graceful shutdown of all other servers via errgroup |
| No double-Stop | `App.Stop()` only triggers cancellation; it never calls `Server.Stop()` directly — shutdown logic is centralized |
| Signal awareness | Listens for `SIGTERM` / `SIGINT` / `SIGQUIT` by default; fully customizable |

---

## Design Principles

### 1. Minimal Interfaces

Each interface defines only the essential methods. For example, `Logger` has just 4 log methods + `With`; adapting any backend requires only a few lines of glue code.

### 2. Zero Implicit Dependencies

The framework makes no assumptions about your registry, config center, or logging library. `go.mod` has a single dependency: `golang.org/x/sync`.

### 3. Context-Native

Every interface takes `context.Context` as its first parameter, consistent with the Go standard library philosophy — supporting tracing and timeout propagation.

### 4. Concurrency-Safe

Global state (logger) and metadata propagation are concurrency-safe. `WithTraceID` deep-copies shared maps to prevent data races.

---

## Requirements

| Item | Requirement |
|:---|:---|
| Go version | 1.21+ |
| Dependencies | Only `golang.org/x/sync` |

---

## License

[MIT License](./LICENSE)
