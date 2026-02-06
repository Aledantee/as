# as

**as** is a Go library for building long-running, supervised services. It provides a single `Service` type with configurable init/run/shutdown lifecycle, automatic restart policies, structured logging, OpenTelemetry integration, and context-based configuration.

## Features

- **Lifecycle hooks** — `InitFunc`, `RunFunc`, and `ShutdownFunc` for setup, main loop, and cleanup
- **Supervision** — Optional restart on error or panic with configurable grace period and count
- **Structured logging** — `slog`-based logger in context (JSON or tint-colored), with service name, version, and namespace
- **OpenTelemetry** — Traces and metrics via autoexport; service attributes attached to context
- **Environment config** — Prefixed env vars and `LoadEnv[T]` for typed config from context
- **Context utilities** — `Name`, `Namespace`, `Version`, `Logger`, `Tracer`, `Meter` from context

## Installation

```bash
go get go.aledante.io/as
```

Requires Go 1.25.6 or later.

## Quick example

```go
package main

import (
	"context"
	"time"

	"go.aledante.io/as"
)

func main() {
	svc := &as.Service{
		Name:      "my-service",
		Namespace: "platform",
		Version:   "1.0.0",
		InitFunc: func(ctx context.Context) error {
			as.Logger(ctx).Info("initialized")
			return nil
		},
		RunFunc: func(ctx context.Context) error {
			as.Logger(ctx).Info("running")
			<-ctx.Done()
			return ctx.Err()
		},
		ShutdownFunc: func(ctx context.Context) error {
			as.Logger(ctx).Info("shutting down")
			return nil
		},
	}

	opts := []as.Option{
		as.WithRestartOnError(true),
		as.WithGracePeriod(1 * time.Minute),
		as.WithGraceCount(3),
	}

	svc.RunToCompletionC(context.Background(), opts...)
}
```

## Service lifecycle

1. **Validate** — `Name` and `Version` must be set (panic otherwise).
2. **Loop** — On each iteration (including after a restart), the service runs:
   - **Init** — OpenTelemetry is initialized, then `InitFunc` is run if set. On error, the iteration fails (and may trigger a restart if configured).
   - **Run** — `RunFunc` is executed. It should block until the context is canceled or an error occurs.
   - **Shutdown** — `ShutdownFunc` is called (if set), then OpenTelemetry is shut down. Shutdown errors are logged but do not change the process exit behavior.

When restart is enabled, the **entire** cycle (init → run → shutdown) is repeated after an error or recovered panic, subject to `GracePeriod` and `GraceCount`. Init and shutdown run on every iteration, so they should be idempotent or tolerant of being run multiple times.

## Options

Use `as.DefaultOptions()` or pass `as.Option` funcs into `Run`, `RunC`, or `RunToCompletionC`:

| Option | Description |
|--------|-------------|
| `RestartOnError` | Restart the service when `RunFunc` returns an error |
| `RestartOnErrorDelay` | Delay between restarts after an error |
| `RestartOnPanic` | Restart after a recovered panic |
| `RestartOnPanicDelay` | Delay after a panic (defaults to `RestartOnErrorDelay` if zero) |
| `RecoverPanic` | Recover panics in the run loop and treat them as errors |
| `GracePeriod` | Max time after first start during which restarts are allowed |
| `GraceCount` | Max number of restarts after the first start |
| `ShutdownTimeout` | Max time to wait for shutdown |
| `LogDebug` | Enable debug-level logging (defaults to true when build has local modifications) |
| `LogJson` | Use JSON logging |
| `LogColors` / `LogAutoColors` | Colorized output (auto: when stdout is a TTY) |
| `EnvPrefix` | Prefix for environment variables (default derived from service name/namespace) |

## Context utilities

The context passed to `InitFunc` is augmented by init (e.g. OpenTelemetry); `RunFunc` and `ShutdownFunc` receive that derived child context. In all cases it carries:

- **Identity** — `as.Name(ctx)`, `as.Namespace(ctx)`, `as.Version(ctx)`
- **Logging** — `as.Logger(ctx)` returns an `*slog.Logger` with service metadata
- **Environment** — `as.GetEnv(ctx, key)`, `as.LookupEnv(ctx, key)`, `as.LoadEnv[T](ctx)` using the configured `EnvPrefix`
- **OpenTelemetry** — `as.Tracer(ctx)`, `as.Meter(ctx)` for tracing and metrics

## Running the service

- **`Run(opts...)`** — Runs with `context.Background()`, blocks until the service exits, returns the final error.
- **`RunC(ctx, opts...)`** — Same as `Run` but uses the given context (e.g. for cancellation).
- **`RunToCompletion(opts...)`** / **`RunToCompletionC(ctx, opts...)`** — Run the service and, if it exits with an error other than `context.Canceled`, print the error and call `ae.Exit(err)`. Intended for `main()` of always-on daemons.
