// Package as provides primitives for building long-running, supervised
// services in Go.
//
// A Service is any type that implements Name, Namespace, Version, Init, Run,
// and Close. Run one service with Run / RunAndExit, or several cooperating
// services with RunGroup / RunGroupAndExit. The run context is cancelled on
// SIGINT or SIGTERM so services can shut down gracefully by blocking on
// <-ctx.Done() and returning ctx.Err() from Run.
//
// # Supervision
//
// Services can be configured to restart automatically on error or on
// recovered panic, bounded by a grace period and a grace count (see
// Options). Each iteration runs the full (init → run → close) cycle; Init
// and Close must therefore be idempotent.
//
// # Context utilities
//
// The ctx passed to Init, Run, and Close carries service identity (Name,
// Namespace, Version), a configured *slog.Logger (Logger), an env prefix
// derived from the service identity (EnvPrefix), OpenTelemetry providers
// (TracerProvider, MeterProvider, Tracer, Meter, TextMapPropagator), and
// typed-config helpers (GetEnv, LookupEnv, LoadEnv).
//
// # Options and environment
//
// Options are applied via Option func helpers (WithRestartOnError,
// WithGracePeriod, WithShutdownTimeout, etc.) and then merged with
// environment variables prefixed by the normalized "<namespace>_<name>_"
// (see NormalizeEnvKey). So WithGraceCount(3) can always be overridden at
// runtime by setting PREFIX_GRACE_COUNT in the environment.
package as
