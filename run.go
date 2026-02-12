package as

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"time"

	"go.aledante.io/ae"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

// Run starts the service in a new background context with the given options.
// The context is cancelled on SIGINT or SIGKILL so the service can shut down gracefully.
// Blocks until the service exits. Returns any error encountered during execution
// or initialization. Convenience wrapper for RunC.
func Run(svc Service, opts ...Option) error {
	return RunC(svc, context.Background(), opts...)
}

// RunAndExit starts the service in a background context. The context is cancelled
// on SIGINT or SIGKILL for graceful shutdown. Exits the process only if the service
// returns an error other than context.Canceled. Intended for main; errors are reported, then ae.Exit is called.
func RunAndExit(svc Service, opts ...Option) {
	RunAndExitC(svc, context.Background(), opts...)
}

// RunAndExitC starts the service; the run context is cancelled on SIGINT or SIGKILL.
// Exits the process only if the service returns an error other than context.Canceled.
// Used for robust always-on daemons; prints errors and performs ae.Exit.
func RunAndExitC(svc Service, ctx context.Context, opts ...Option) {
	if err := RunC(svc, ctx, opts...); err != nil {
		if !errors.Is(err, context.Canceled) {
			ae.Print(err, ae.PrintFrameFilters(func(frame *ae.StackFrame) bool {
				return !strings.HasPrefix(frame.Func, "go.aledante.io/as.")
			}))
		}

		ae.Exit(err)
	}
}

// RunC starts the service with the given options. The run context is cancelled
// when the process receives SIGINT or SIGKILL, so Run can return and Close runs for cleanup.
// Returns when the service exits, with any final error.
func RunC(svc Service, ctx context.Context, opts ...Option) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()

	if err := validateService(svc); err != nil {
		return ae.New().
			Fatal().
			Cause(err).
			Msg("invalid service")
	}

	options := applyOptions(svc.Name(), svc.Namespace(), opts)

	// Add error attributes to the contextÏ
	ctx = ae.WithOtelAttribute(ctx,
		semconv.ServiceNameKey.String(svc.Name()),
		semconv.ServiceVersionKey.String(svc.Version()),
		semconv.ServiceNamespaceKey.String(svc.Namespace()),
	)

	// Add service attributes to the context
	ctx = withName(ctx, svc.Name())
	ctx = withVersion(ctx, svc.Version())
	ctx = withNamespace(ctx, svc.Namespace())
	ctx = withEnvPrefix(ctx, options.EnvPrefix)

	// Create initial logger
	ctx = WithLogger(ctx, initLogger(ctx, options))

	// Initialize OTEL
	ctx, otelShutdown, err := initOtel(ctx)
	if err != nil {
		return ae.New().
			Fatal().
			Cause(err).
			Msg("failed to initialize OTEL")
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

	return runLoop(svc, ctx, options)
}

func validateService(svc Service) error {
	var errs []error

	if svc.Name() == "" {
		errs = append(errs, errors.New("service name cannot be empty"))
	}
	if svc.Namespace() == "" {
		errs = append(errs, errors.New("service namespace cannot be empty"))
	}

	return ae.WrapMany("invalid service", errs...)
}

// runLoop is the internal orchestration entry point. It handles logger creation,
// tracks running state, and enforces debug level, and supervises the lifecycle loop.
func runLoop(svc Service, ctx context.Context, opts Options) error {
	graceStart := time.Now()
	graceCount := 0

	for {
		err, isPanic := runOnce(svc, ctx, opts)
		if err == nil {
			return nil
		}

		if !opts.RestartOnError || !ae.IsRecoverable(err) {
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

func runOnce(svc Service, ctx context.Context, opts Options) (err error, isPanic bool) {
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
		return ae.Wrap("service initialization failed", err), false
	}

	Logger(ctx).Debug("starting service")
	if err = svc.Run(ctx); err != nil {
		// Do not handle context.Canceled errors here, since they are expected and we should clean up on cancellation
		if !errors.Is(err, context.Canceled) {
			return ae.Wrap("service run failed", err), false
		}
	}

	// Cleanup is not returned as an error, since it's not critical.
	Logger(ctx).Debug("shutting down service")
	err = svc.Close(ctx)
	if err != nil {
		Logger(ctx).Error("service shutdown failed", "error", err)
	}

	return nil, false
}
