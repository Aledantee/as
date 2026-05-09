package as

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.aledante.io/ae"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

// Run starts the service in a new background context with the given options.
// The context is cancelled on SIGINT or SIGTERM so the service can shut down gracefully.
// Blocks until the service exits. Returns any error encountered during execution
// or initialization. Convenience wrapper for RunC.
func Run(svc Service, opts ...Option) error {
	return RunC(svc, context.Background(), opts...)
}

// RunAndExit starts the service in a background context. The context is cancelled
// on SIGINT or SIGTERM for graceful shutdown. Exits the process only if the service
// returns an error other than context.Canceled. Intended for main; errors are reported, then ae.Exit is called.
func RunAndExit(svc Service, opts ...Option) {
	RunAndExitC(svc, context.Background(), opts...)
}

// RunAndExitC starts the service; the run context is cancelled on SIGINT or SIGTERM.
// Exits the process only if the service returns an error other than context.Canceled.
// Used for robust always-on daemons; prints errors and performs ae.Exit.
func RunAndExitC(svc Service, ctx context.Context, opts ...Option) {
	if err := RunC(svc, ctx, opts...); err != nil {
		if !errors.Is(err, context.Canceled) {
			printRunError(err, applyOptions(svc.Name(), svc.Namespace(), opts))
		}

		ae.Exit(err)
	}
}

// isFrameworkFrame reports whether a stack frame originates inside this package
// and should be hidden from user-facing stack traces.
func isFrameworkFrame(frame *ae.StackFrame) bool {
	return frame != nil && strings.HasPrefix(frame.Func, "go.aledante.io/as.")
}

// printRunError renders a final, unrecoverable error in a format that matches
// the runtime logger configuration: JSON when LogJson is set, colored text on
// a TTY, plain text otherwise. Framework frames are hidden so the trace points
// at the user's service code.
func printRunError(err error, opts Options) {
	printerOpts := []ae.PrinterOption{
		ae.PrintFrameFilters(isFrameworkFrame),
	}
	if opts.LogJson {
		printerOpts = append(printerOpts, ae.PrintJSON())
	}
	if !effectiveLogColors(opts) {
		printerOpts = append(printerOpts, ae.NoPrintColors())
	}

	ae.NewPrinter(printerOpts...).Print(err)
}

// RunC starts the service with the given options. The run context is derived
// from the provided ctx and cancelled when the process receives SIGINT or
// SIGTERM, so Run can return and Close runs for cleanup.
// Returns when the service exits, with any final error.
func RunC(svc Service, ctx context.Context, opts ...Option) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer cancel()

	return runService(svc, ctx, opts...)
}

// runService drives a single service through its entire lifecycle without
// installing signal handling. It is called by RunC (which wraps ctx in a
// signal-notified derivative) and by RunGroupC (which installs one signal
// handler for the whole group).
func runService(svc Service, ctx context.Context, opts ...Option) error {
	if err := validateService(svc); err != nil {
		return ae.New().
			Fatal().
			Cause(err).
			Msg("invalid service")
	}

	options := applyOptions(svc.Name(), svc.Namespace(), opts)

	ctx = ae.WithOtelAttribute(ctx,
		semconv.ServiceNameKey.String(svc.Name()),
		semconv.ServiceVersionKey.String(svc.Version()),
		semconv.ServiceNamespaceKey.String(svc.Namespace()),
	)

	ctx = withName(ctx, svc.Name())
	ctx = withVersion(ctx, svc.Version())
	ctx = withNamespace(ctx, svc.Namespace())
	ctx = withEnvPrefix(ctx, options.EnvPrefix)

	ctx = WithLogger(ctx, initLogger(ctx, options))

	ctx, otelShutdown, err := initOtel(ctx)
	if err != nil {
		return ae.New().
			Fatal().
			Cause(err).
			Msg("failed to initialize OTEL")
	}
	if otelShutdown != nil {
		defer func() {
			shutdownCtx, cancel := shutdownContext(ctx, options)
			defer cancel()
			if shutdownErr := otelShutdown(shutdownCtx); shutdownErr != nil {
				Logger(ctx).Error(
					"OTEL shutdown failed",
					"error", shutdownErr,
				)
			}
		}()
	}

	return runLoop(svc, ctx, options)
}

// shutdownContext returns a context suitable for cleanup work (Close / OTEL
// shutdown). It detaches from the parent's cancellation so Close has a fresh
// chance to complete even if the parent was cancelled by a signal, and
// applies options.ShutdownTimeout as a deadline when set.
func shutdownContext(parent context.Context, opts Options) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(parent)
	if opts.ShutdownTimeout > 0 {
		return context.WithTimeout(detached, opts.ShutdownTimeout)
	}
	return detached, func() {}
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

// runLoop is the internal orchestration entry point. It supervises the
// lifecycle loop, applies the restart/grace policy, and returns the final
// error (or nil if the service exited cleanly).
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
			timer := time.NewTimer(restartDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		} else {
			Logger(ctx).Error("service failed, restarting immediately", logAttrs...)
		}
	}
}

// runOnce executes a single (init → run → close) iteration. Close is
// registered as a defer after a successful Init, so it runs on every exit
// path — success, Run error, or recovered panic — per the documented cycle.
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
	if initErr := svc.Init(ctx); initErr != nil {
		return ae.Wrap("service initialization failed", initErr), false
	}

	defer func() {
		shutdownCtx, cancel := shutdownContext(ctx, opts)
		defer cancel()

		Logger(ctx).Debug("shutting down service")
		if closeErr := svc.Close(shutdownCtx); closeErr != nil {
			Logger(ctx).Error("service shutdown failed", "error", closeErr)
		}
	}()

	Logger(ctx).Debug("starting service")
	if runErr := svc.Run(ctx); runErr != nil {
		if !errors.Is(runErr, context.Canceled) {
			return ae.Wrap("service run failed", runErr), false
		}
	}

	return nil, false
}
