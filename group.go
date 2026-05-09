package as

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.aledante.io/ae"
)

// RunGroup starts multiple services concurrently in a new background context.
// The context is cancelled on SIGINT or SIGTERM so every service can shut
// down gracefully. Blocks until all services exit; returns the combined
// error (or nil when every service exited cleanly). Convenience wrapper for
// RunGroupC.
func RunGroup(svcs []Service, opts ...Option) error {
	return RunGroupC(svcs, context.Background(), opts...)
}

// RunGroupC runs multiple services concurrently with a shared ctx and
// options. Each service runs its own (Init → Run → Close) lifecycle, with
// its own name/namespace/env-prefix/logger context values; they all share
// the same cancellation. When any service exits with an error the shared
// ctx is cancelled so the peers can shut down. Returns once every service
// has exited, with the combined error or nil.
//
// Service (name, namespace) pairs must be unique within the group; an empty
// group or a group containing an invalid service returns an error without
// starting any goroutines.
func RunGroupC(svcs []Service, ctx context.Context, opts ...Option) error {
	if err := validateGroup(svcs); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer cancel()

	groupCtx, groupCancel := context.WithCancel(ctx)
	defer groupCancel()

	errs := make(chan error, len(svcs))
	var wg sync.WaitGroup

	for _, svc := range svcs {
		svc := svc
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runService(svc, groupCtx, opts...); err != nil {
				errs <- err
				groupCancel()
			}
		}()
	}

	wg.Wait()
	close(errs)

	var collected []error
	for err := range errs {
		collected = append(collected, err)
	}
	return ae.WrapMany("service group failed", collected...)
}

// RunGroupAndExit starts a group and calls ae.Exit on failure other than
// context.Canceled.
func RunGroupAndExit(svcs []Service, opts ...Option) {
	RunGroupAndExitC(svcs, context.Background(), opts...)
}

// RunGroupAndExitC runs the group and, on failure other than
// context.Canceled, prints the error and calls ae.Exit.
func RunGroupAndExitC(svcs []Service, ctx context.Context, opts ...Option) {
	if err := RunGroupC(svcs, ctx, opts...); err != nil {
		if !errors.Is(err, context.Canceled) {
			var fmtOpts Options
			if len(svcs) > 0 {
				fmtOpts = applyOptions(svcs[0].Name(), svcs[0].Namespace(), opts)
			}
			printRunError(err, fmtOpts)
		}
		ae.Exit(err)
	}
}

// validateGroup returns a non-nil error when the group is empty, contains a
// service that fails validateService, or contains duplicate (name, namespace)
// pairs.
func validateGroup(svcs []Service) error {
	if len(svcs) == 0 {
		return ae.New().
			Fatal().
			Msg("service group cannot be empty")
	}

	var errs []error
	seen := map[string]struct{}{}

	for i, svc := range svcs {
		if svc == nil {
			errs = append(errs, ae.New().
				Fatal().
				Msgf("service at index %d is nil", i))
			continue
		}
		if err := validateService(svc); err != nil {
			errs = append(errs, err)
			continue
		}
		key := svc.Namespace() + "/" + svc.Name()
		if _, dup := seen[key]; dup {
			errs = append(errs, ae.New().
				Fatal().
				Msgf("duplicate service in group: %s", key))
		}
		seen[key] = struct{}{}
	}

	if err := ae.WrapMany("invalid service group", errs...); err != nil {
		return ae.New().Fatal().Cause(err).Msg("invalid service group")
	}
	return nil
}
