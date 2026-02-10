# as

**as** is a Go library for building long-running, supervised services. Implement the `Service` interface with `Init`, `Run`, and `Close`; run one or multiple services with configurable restart policies, structured logging, OpenTelemetry integration, and context-based configuration.

## Features

- **Service interface** — `Name()`, `Namespace()`, `Version()`, `Init(ctx)`, `Run(ctx)`, `Close(ctx)`
- **Single or group** — Run one service with `Run` / `RunAndExit`, or multiple with `RunGroup` / `RunGroupAndExit`
- **Supervision** — Optional restart on error or panic with configurable grace period and count
- **Structured logging** — `slog`-based logger in context (JSON or tint-colored), with service name, version, and namespace
- **OpenTelemetry** — Traces and metrics via autoexport; service attributes attached to context
- **Environment config** — Prefixed env vars and `LoadEnv[T]` for typed config from context; env key normalization for POSIX-safe names
- **Context utilities** — `Name`, `Namespace`, `Version`, `Logger`, `Tracer`, `Meter`, `EnvPrefix`, `GetEnv`, `LookupEnv`, `LoadEnv[T]` from context

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

type myService struct {
	name, namespace, version string
}

func (m *myService) Name() string      { return m.name }
func (m *myService) Namespace() string { return m.namespace }
func (m *myService) Version() string   { return m.version }

func (m *myService) Init(ctx context.Context) error {
	as.Logger(ctx).Info("initialized")
	return nil
}

func (m *myService) Run(ctx context.Context) error {
	as.Logger(ctx).Info("running")
	<-ctx.Done()
	return ctx.Err()
}

func (m *myService) Close(ctx context.Context) error {
	as.Logger(ctx).Info("shutting down")
	return nil
}

func main() {
	svc := &myService{
		name:      "my-service",
		namespace: "platform",
		version:   "1.0.0",
	}

	opts := []as.Option{
		as.WithRestartOnError(true),
		as.WithGracePeriod(1 * time.Minute),
		as.WithGraceCount(3),
	}

	as.RunAndExit(svc, opts...)
}
```

## Service lifecycle

1. **Validate** — Each service must have non-empty `Name()` and `Namespace()`. In a group, (name, namespace) must be unique.
2. **Loop** — On each iteration (including after a restart), the service runs:
   - **Init** — OpenTelemetry is initialized, then `Init(ctx)` is called. On error, the iteration fails (and may trigger a restart if configured).
   - **Run** — `Run(ctx)` is executed. It should block until the context is canceled or an error occurs.
   - **Close** — `Close(ctx)` is called, then OpenTelemetry is shut down. Close errors are logged but do not change the process exit behavior.

When restart is enabled, the **entire** cycle (init → run → close) is repeated after an error or recovered panic, subject to `GracePeriod` and `GraceCount`. Init and Close run on every iteration, so they should be idempotent or tolerant of being run multiple times.

## Options

Use `as.DefaultOptions()` or pass `as.Option` funcs into `Run`, `RunC`, `RunGroup`, `RunGroupC`, `RunAndExit`, `RunAndExitC`, `RunGroupAndExit`, or `RunGroupAndExitC`:

| Option | Description |
|--------|-------------|
| `RestartOnError` | Restart the service when `Run` returns an error |
| `RestartOnErrorDelay` | Delay between restarts after an error |
| `RestartOnPanic` | Restart after a recovered panic |
| `RestartOnPanicDelay` | Delay after a panic (defaults to `RestartOnErrorDelay` if zero) |
| `RecoverPanic` | Recover panics in the run loop and treat them as errors |
| `GracePeriod` | Max time after first start during which restarts are allowed |
| `GraceCount` | Max number of restarts after the first start |
| `ShutdownTimeout` | Max time to wait for shutdown |
| `LogDebug` | Enable debug-level logging |
| `LogJson` | Use JSON logging |
| `LogColors` / `LogAutoColors` | Colorized output (auto: when stdout is a TTY) |
| `EnvPrefix` | Prefix for option env vars. If empty, defaults to `<namespace>_<name>_` (namespace omitted if empty); the prefix is normalized via NormalizeEnvKey. Options are then loaded from env (e.g. `PREFIX_RESTART_ON_ERROR`, `PREFIX_GRACE_PERIOD`). |
| `DisableEnvPrefix` | When true, no env prefix is applied when loading options (or for context env helpers); option env names are used as-is. |

## Environment variables

Options (restart, logging, shutdown, etc.) are merged with the environment after applying any `Option` funcs. The effective prefix is the normalized value of either `EnvPrefix` (if set) or `<namespace>_<name>_` (namespace omitted when empty). Each option is read from a prefixed env var; the following names are used (with the prefix applied):

| Env var | Description |
|---------|-------------|
| `RESTART_ON_ERROR` | Restart the service when `Run` returns an error |
| `RESTART_ON_ERROR_DELAY` | Delay between restarts after an error (e.g. `10s`) |
| `RESTART_ON_PANIC` | Restart after a recovered panic |
| `RESTART_ON_PANIC_DELAY` | Delay after a panic (e.g. `5s`); if zero, `RESTART_ON_ERROR_DELAY` is used |
| `RECOVER_PANIC` | Recover panics in the run loop and treat them as errors |
| `GRACE_PERIOD` | Max time after first start during which restarts are allowed (e.g. `1m`) |
| `GRACE_COUNT` | Max number of restarts after the first start |
| `SHUTDOWN_TIMEOUT` | Max time to wait for shutdown (e.g. `30s`) |
| `LOG_DEBUG` | Enable debug-level logging |
| `LOG_JSON` | Use JSON logging |
| `LOG_COLORS` | Force colorized output |
| `LOG_COLORS_AUTO` | Colorize when stdout is a TTY |

### Environment key normalization

Option prefixes and environment variable keys used with `GetEnv` / `LookupEnv` are normalized via `NormalizeEnvKey` so that names are POSIX-safe and consistent. Normalization:

- Decomposes accented Unicode (e.g. "é" → "e"); combining marks are removed
- Converts letters to uppercase
- Replaces non-alphanumeric characters with a single underscore and trims leading/trailing underscores

Resulting keys use only `[A-Z0-9_]`. Example: `"my-Énv.key"` → `"MY_ENV_KEY"`. The option prefix (from `EnvPrefix` or `<namespace>_<name>_`) is normalized before reading options from the environment. `as.GetEnv(ctx, key)` and `as.LookupEnv(ctx, key)` apply the same normalization to the key used for lookup. `as.LoadEnv[T](ctx)` uses the prefix from context (set from options when the service runs).

## Context utilities

The context passed to `Init`, `Run`, and `Close` carries:

- **Identity** — `as.Name(ctx)`, `as.Namespace(ctx)`, `as.Version(ctx)`
- **Logging** — `as.Logger(ctx)` returns an `*slog.Logger` with service metadata
- **Environment** — The env prefix (from `EnvPrefix` or default `<namespace>_<name>_`, normalized) is set in context. Use `as.GetEnv(ctx, key)`, `as.LookupEnv(ctx, key)`, `as.LoadEnv[T](ctx)`.
- **OpenTelemetry** — `as.Tracer(ctx)`, `as.Meter(ctx)` for tracing and metrics

## Running the service

- **`Run(svc, opts...)`** — Runs a single service with `context.Background()`, blocks until it exits, returns the final error.
- **`RunC(svc, ctx, opts...)`** — Same as `Run` but uses the given context (e.g. for cancellation).
- **`RunGroup(svcs, opts...)`** / **`RunGroupC(svcs, ctx, opts...)`** — Run multiple services in an errgroup; all share the same context and options; returns when the first fails or context is canceled.
- **`RunAndExit(svc, opts...)`** / **`RunAndExitC(svc, ctx, opts...)`** — Run one service and, if it exits with an error other than `context.Canceled`, print the error and call `ae.Exit(err)`. Intended for `main()` of always-on daemons.
- **`RunGroupAndExit(svcs, opts...)`** / **`RunGroupAndExitC(svcs, ctx, opts...)`** — Same for a group of services.
