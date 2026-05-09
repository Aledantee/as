// Advanced example: a single service demonstrating option helpers, typed
// environment config via LoadEnv[T], proper blocking Run, and cooperative
// Close with the ShutdownTimeout deadline.
//
// Run with:
//
//	go run ./_example/advanced
//
// Override config via env (prefix = "DEMO_API_", normalized from namespace_name):
//
//	DEMO_API_PORT=9090 DEMO_API_MESSAGE="hello" go run ./_example/advanced
//
// Graceful shutdown: Ctrl-C (SIGINT) or `kill -TERM <pid>`.
package main

import (
	"context"
	"fmt"
	"time"

	"go.aledante.io/as"
)

func main() {
	as.RunAndExit(
		&apiService{},
		// Supervision: restart on errors, but cap to 5 attempts in any 2-minute window.
		as.WithRestartOnError(true),
		as.WithGracePeriod(2*time.Minute),
		as.WithGraceCount(5),
		// Give Close up to 10s to flush before the library abandons it.
		as.WithShutdownTimeout(10*time.Second),
		// Human-readable logs for local dev; flip to JSON in prod.
		as.WithLogJson(false),
		as.WithLogDebug(true),
	)
}

// Config is filled by LoadEnv[Config] from prefixed environment variables.
// With namespace "demo" and name "api", the prefix is "DEMO_API_", so the
// active keys are DEMO_API_PORT and DEMO_API_MESSAGE.
type Config struct {
	Port    int    `env:"PORT" envDefault:"8080"`
	Message string `env:"MESSAGE" envDefault:"hello from advanced example"`
}

type apiService struct {
	cfg Config
}

func (s *apiService) Name() string      { return "api" }
func (s *apiService) Namespace() string { return "demo" }
func (s *apiService) Version() string   { return "1.0.0" }

// Init loads configuration from the environment and records it on the
// service. Runs on every (init → run → close) iteration, so it must be
// idempotent.
func (s *apiService) Init(ctx context.Context) error {
	cfg, err := as.LoadEnv[Config](ctx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	s.cfg = cfg

	log := as.Logger(ctx)
	log.Info("configuration loaded",
		"port", cfg.Port,
		"message", cfg.Message,
		"env_prefix", as.EnvPrefix(ctx),
	)
	return nil
}

// Run is the main loop. It must block until the context is cancelled (by a
// signal or a peer failure in a group) and return ctx.Err() for a graceful
// exit. Here it ticks once per second to prove liveness.
func (s *apiService) Run(ctx context.Context) error {
	log := as.Logger(ctx)
	log.Info("service starting", "port", s.cfg.Port)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("context cancelled, exiting run loop")
			return ctx.Err()
		case t := <-ticker.C:
			log.Debug("tick", "at", t.Format(time.RFC3339), "message", s.cfg.Message)
		}
	}
}

// Close performs cleanup. The ctx carries the ShutdownTimeout deadline, so
// a cooperative Close honours it by selecting on <-ctx.Done(). This matches
// how net/http.Server.Shutdown behaves.
func (s *apiService) Close(ctx context.Context) error {
	log := as.Logger(ctx)
	log.Info("closing service; flushing state")

	// Simulate non-trivial cleanup that respects the shutdown deadline.
	select {
	case <-time.After(500 * time.Millisecond):
		log.Info("cleanup complete")
		return nil
	case <-ctx.Done():
		log.Warn("shutdown deadline hit before cleanup finished", "error", ctx.Err())
		return ctx.Err()
	}
}
