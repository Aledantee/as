package as

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.aledante.io/ae"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"golang.org/x/sync/errgroup"
)

// Run starts the service in a new background context with the given options.
// Blocks until the service exits. Returns any error encountered during execution
// or initialization. Convenience wrapper for RunC.
func Run(svc Service, opts ...Option) error {
	return RunGroupC([]Service{svc}, context.Background(), opts...)
}

// RunGroup starts the service in a new background context with the given options.
// Blocks until the service exits. Returns any error encountered during execution
// or initialization. Convenience wrapper for RunC.
func RunGroup(svcs []Service, opts ...Option) error {
	return RunGroupC(svcs, context.Background(), opts...)
}

// RunAndExit starts the service in a background context and forcibly
// exits the process if the service exits with an error other than context.Canceled.
// Intended for main-functions. Errors are reported, then ae.Exit is called.
func RunAndExit(svc Service, opts ...Option) {
	RunGroupAndExitC([]Service{svc}, context.Background(), opts...)
}

// RunGroupAndExit starts the service in a background context and forcibly
// exits the process if the service exits with an error other than context.Canceled.
// Intended for main-functions. Errors are reported, then ae.Exit is called.
func RunGroupAndExit(svcs []Service, opts ...Option) {
	RunGroupAndExitC(svcs, context.Background(), opts...)
}

// RunAndExitC starts the service in a given context and forcibly
// exits the process if the service returns error other than context.Canceled.
// Used for robust always-on daemons; prints errors and performs ae.Exit.
func RunAndExitC(svc Service, ctx context.Context, opts ...Option) {
	RunGroupAndExitC([]Service{svc}, ctx, opts...)
}

// RunGroupAndExitC starts the service in a given context and forcibly
// exits the process if the service returns error other than context.Canceled.
// Used for robust always-on daemons; prints errors and performs ae.Exit.
func RunGroupAndExitC(svcs []Service, ctx context.Context, opts ...Option) {
	if err := RunGroupC(svcs, ctx, opts...); err != nil {
		if !errors.Is(err, context.Canceled) {
			ae.Print(err, ae.PrintFrameFilters(func(frame *ae.StackFrame) bool {
				return !strings.HasPrefix(frame.Func, "go.aledante.io/as.")
			}))
		}

		ae.Exit(err)
	}
}

// RunC starts the service in the provided context with the given options.
// Returns when the service exits, with any final error.
func RunC(svc Service, ctx context.Context, opts ...Option) error {
	return RunGroupC([]Service{svc}, ctx, opts...)
}

// RunGroupC starts the service in the provided context with the given options.
// Returns when the service exits, with any final error.
func RunGroupC(svcs []Service, ctx context.Context, opts ...Option) error {
	if len(svcs) == 0 {
		return nil
	}

	if err := validateServices(svcs); err != nil {
		return err
	}

	ctx, otelShutdown, err := initOtel(ctx)
	if err != nil {
		return ae.Wrap("OTEL initialization failed", err)
	}
	if otelShutdown != nil {
		defer func() {
			if shutdownErr := otelShutdown(ctx); shutdownErr != nil {
				Logger(ctx).Error(
					"OTEL shutdown failed",
					"error", shutdownErr,
				)
			}
		}()
	}

	errGroup, ctx := errgroup.WithContext(ctx)
	for _, svc := range svcs {
		errGroup.Go(func() error {
			return runLoop(svc, ctx, applyOptions(svc.Name(), svc.Namespace(), opts))
		})
	}

	return errGroup.Wait()
}

func validateServices(svcs []Service) error {
	var errs []error

	type identity struct {
		name, namespace string
	}
	identities := make(map[identity]struct{})

	for _, svc := range svcs {
		ident := identity{svc.Name(), svc.Namespace()}
		if _, ok := identities[ident]; ok {
			errs = append(errs, ae.New().Attr("name", svc.Name()).
				Attr("namespace", svc.Namespace()).
				Msg("duplicate service identity"))
			continue
		}
		identities[ident] = struct{}{}

		if svc.Name() == "" {
			errs = append(errs, errors.New("service name cannot be empty"))
		}
		if svc.Namespace() == "" {
			errs = append(errs, errors.New("service namespace cannot be empty"))
		}
	}

	if len(svcs) > 1 {
		return ae.WrapMany("invalid service group", errs...)
	}

	return ae.WrapMany("invalid service", errs...)
}

// runLoop is the internal orchestration entry point. It handles logger creation,
// tracks running state, and enforces debug level, and supervises the lifecycle loop.
func runLoop(svc Service, ctx context.Context, opts Options) error {
	// Add error attributes to the context
	ctx = ae.WithOtelAttribute(ctx,
		semconv.ServiceNameKey.String(svc.Name()),
		semconv.ServiceVersionKey.String(svc.Version()),
		semconv.ServiceNamespaceKey.String(svc.Namespace()),
	)

	// Add service attributes to the context
	ctx = withName(ctx, svc.Name())
	ctx = withVersion(ctx, svc.Version())
	ctx = withNamespace(ctx, svc.Namespace())
	ctx = withEnvPrefix(ctx, opts.EnvPrefix)

	// Create initial logger
	ctx = WithLogger(ctx, initLogger(ctx, opts))

	graceStart := time.Now()
	graceCount := 0

	for {
		err, isInternal, isPanic := runOnce(svc, ctx, opts)
		if err == nil {
			return nil
		}

		if isInternal || !opts.RestartOnError {
			return err
		}

		graceCount++

		logAttrs := []any{
			"error", err,
		}
		if opts.GracePeriod > 0 {
			logAttrs = append(logAttrs, "grace_period", opts.GracePeriod.String())
		}
		if opts.GraceCount > 0 {
			logAttrs = append(logAttrs, "grace_count", opts.GraceCount, "grace_count_remaining", opts.GraceCount-graceCount)
		}

		if opts.GracePeriod > 0 && time.Since(graceStart) > opts.GracePeriod {
			Logger(ctx).Error(
				"service failed, exceeded grace period",
				logAttrs...,
			)
			return err
		}

		if opts.GraceCount > 0 && graceCount > opts.GraceCount {
			Logger(ctx).Error(
				"service failed, exceeded grace count",
				logAttrs...,
			)
			return err
		}

		restartDelay := opts.RestartOnErrorDelay
		if isPanic {
			if !opts.RestartOnPanic {
				return err
			}

			if opts.RestartOnPanicDelay > 0 {
				restartDelay = opts.RestartOnPanicDelay
			}
		}

		logAttrs = append(logAttrs, "restart_delay", restartDelay.String())

		if restartDelay > 0 {
			Logger(ctx).Error("service failed, restarting after delay", logAttrs...)
			time.Sleep(restartDelay)
		} else {
			Logger(ctx).Error("service failed, restarting immediately", logAttrs...)
		}
	}
}

func runOnce(svc Service, ctx context.Context, opts Options) (err error, isInternal bool, isPanic bool) {
	if opts.RecoverPanic {
		defer func() {
			if cause := recover(); cause != nil {
				var errCause error
				switch x := cause.(type) {
				case error:
					errCause = x
				default:
					errCause = ae.Msgf("%v", x)
				}

				isPanic = true
				err = ae.NewC(ctx).
					Cause(errCause).
					Stack().
					Related(err).
					Msg("panic")
			}
		}()
	}

	Logger(ctx).Debug("initializing service")
	if err := svc.Init(ctx); err != nil {
		return ae.Wrap("service initialization failed", err), false, false
	}

	Logger(ctx).Debug("starting service")
	if err = svc.Run(ctx); err != nil {
		// Do not handle context.Canceled errors here, since they are expected and we should clean up on cancellation
		if !errors.Is(err, context.Canceled) {
			return ae.Wrap("service run failed", err), true, false
		}
	}

	// Cleanup is not returned as an error, since it's not critical.
	Logger(ctx).Debug("shutting down service")
	err = svc.Close(ctx)
	if err != nil {
		Logger(ctx).Error("service shutdown failed", "error", err)
	}

	return nil, false, false
}
