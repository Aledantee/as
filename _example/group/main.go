// Group example: two cooperating services (api + worker) in the same
// namespace, started together via RunGroupAndExit. They share one ctx —
// if either fails or a shutdown signal arrives, both exit gracefully.
//
// Run with:
//
//	go run ./_example/group
//
// Graceful shutdown: Ctrl-C (SIGINT) or `kill -TERM <pid>`.
//
// (name, namespace) pairs must be unique within the group; here we use
// ("api", "demo") and ("worker", "demo").
package main

import (
	"context"
	"time"

	"go.aledante.io/as"
)

func main() {
	as.RunGroupAndExit(
		[]as.Service{
			&apiService{},
			&workerService{},
		},
		// Each service gets up to 5s for Close; the group waits for all of
		// them before returning.
		as.WithShutdownTimeout(5*time.Second),
		as.WithLogJson(false),
	)
}

// --- apiService ---------------------------------------------------------

type apiService struct{}

func (s *apiService) Name() string      { return "api" }
func (s *apiService) Namespace() string { return "demo" }
func (s *apiService) Version() string   { return "1.0.0" }

func (s *apiService) Init(ctx context.Context) error {
	as.Logger(ctx).Info("api: init", "env_prefix", as.EnvPrefix(ctx))
	return nil
}

// Run blocks until the shared group ctx is cancelled. A failure in any peer
// triggers that cancellation so the api shuts down in lockstep.
func (s *apiService) Run(ctx context.Context) error {
	log := as.Logger(ctx)
	log.Info("api: running")

	<-ctx.Done()
	log.Info("api: shutdown signalled")
	return ctx.Err()
}

func (s *apiService) Close(ctx context.Context) error {
	as.Logger(ctx).Info("api: closing")
	return nil
}

// --- workerService ------------------------------------------------------

type workerService struct{}

func (s *workerService) Name() string      { return "worker" }
func (s *workerService) Namespace() string { return "demo" }
func (s *workerService) Version() string   { return "1.0.0" }

func (s *workerService) Init(ctx context.Context) error {
	as.Logger(ctx).Info("worker: init", "env_prefix", as.EnvPrefix(ctx))
	return nil
}

// Run ticks periodically and exits cleanly on shared ctx cancellation.
func (s *workerService) Run(ctx context.Context) error {
	log := as.Logger(ctx)
	log.Info("worker: running")

	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("worker: shutdown signalled")
			return ctx.Err()
		case <-ticker.C:
			log.Info("worker: processed a job")
		}
	}
}

func (s *workerService) Close(ctx context.Context) error {
	as.Logger(ctx).Info("worker: closing")
	return nil
}
