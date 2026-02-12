package main

import (
	"context"

	"go.aledante.io/as"
)

func main() {
	// RunAndExit runs the service and calls os.Exit on non-cancel errors.
	// Use this in main for daemon-style binaries.
	as.RunAndExit(&service{})
}

// service implements as.Service with minimal behavior for demonstration.
type service struct {
}

// Name returns the service name (used for logging, env prefix, OTEL, etc.).
func (s *service) Name() string {
	return "example"
}

// Namespace returns the service namespace (e.g. for grouping or multi-tenant config).
func (s *service) Namespace() string {
	return "service"
}

// Version returns the service version (for OTEL and diagnostics).
func (s *service) Version() string {
	return "1.0.0"
}

// Init runs once before Run. Use it to load config, open connections, or validate env.
// This should be idempotent and tolerant of being run multiple times.
func (s *service) Init(ctx context.Context) error {
	logger := as.Logger(ctx)

	logger.Info("env prefix", "prefix", as.EnvPrefix(ctx))
	return nil
}

// Run is the main event loop. It should block until shutdown or error.
// The context is cancelled on SIGINT/SIGKILL; typically block with <-ctx.Done() and return ctx.Err().
// Returning an error (other than context.Canceled) stops the service and may trigger exit/restart.
// This should be idempotent and tolerant of being run multiple times.
func (s *service) Run(ctx context.Context) error {
	as.Logger(ctx).Info("running")
	return nil
}

// Close is called after Run returns. Use it to release resources and flush state.
// This should be idempotent and tolerant of being run multiple times.
func (s *service) Close(ctx context.Context) error {
	as.Logger(ctx).Info("closing")
	return nil
}
