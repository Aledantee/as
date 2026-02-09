package as

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"go.aledante.io/ae"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Service represents a long-running, self-contained application process.
// It tracks metadata about the service (name, domain, version) and delegates
// to client-provided hooks for initialization, execution, and shutdown.
// Service manages its own status and provides controlled run loops for
// restart-on-error and graceful shutdown.
type Service struct {
	// Name is the human-friendly identifier for the service instance.
	Name string
	// Namespace is a logical grouping for the service, useful for organizing monitoring,
	// tracing, and configuration. E.g. "monitoring", "billing", etc.
	Namespace string
	// Version is a semantic version tag for the service.
	// Should be either SemVer or CalVer
	Version string
	// InitFunc is called once before starting RunFunc. It is used for setup and
	// validation. If it returns an error, RunFunc is not invoked.
	// This is optional (may be nil).
	InitFunc func(ctx context.Context) error
	// RunFunc is the main execution function (service body). It must not be nil.
	// Should block for the lifetime of the service unless canceled.
	RunFunc func(ctx context.Context) error
	// ShutdownFunc is called once after RunFunc exits, for resource deallocation,
	// flushing, or deregistration. ShutdownFunc is optional (may be nil).
	ShutdownFunc func(ctx context.Context) error

	// running guards against multiple concurrent or repeated invocations.
	running      atomic.Bool
	otelShutdown func(ctx context.Context) error
}

// Run starts the service in a new background context with the given options.
// Blocks until the service exits. Returns any error encountered during execution
// or initialization. Convenience wrapper for RunC.
func (s *Service) Run(opts ...Option) error {
	return s.RunC(context.Background())
}

// RunToCompletion starts the service in a background context and forcibly
// exits the process if the service exits with an error other than context.Canceled.
// Intended for main-functions. Panics or errors are reported, then os.Exit is called.
func (s *Service) RunToCompletion(opts ...Option) {
	s.RunToCompletionC(context.Background(), opts...)
}

// RunToCompletionC starts the service in a given context and forcibly
// exits the process if the service returns error other than context.Canceled.
// Used for robust always-on daemons; prints errors and performs ae.Exit.
func (s *Service) RunToCompletionC(ctx context.Context, opts ...Option) {
	if err := s.RunC(ctx, opts...); err != nil {
		if !errors.Is(err, context.Canceled) {
			ae.Print(err, ae.PrintFrameFilters(func(frame *ae.StackFrame) bool {
				return strings.HasPrefix(frame.Func, "go.aledante.io/as.(*Service)")
			}))
		}

		ae.Exit(err)
	}
}

// RunC starts the service in the provided context with the given options.
// Returns when the service exits, with any final error.
func (s *Service) RunC(ctx context.Context, opts ...Option) error {
	s.validate()

	return s.runLoop(ctx, applyOptions(s.Name, s.Namespace, opts))
}

func (s *Service) validate() {
	if s.Name == "" {
		panic("service name must be set")
	}
	if s.Version == "" {
		panic("service version must be set")
	}
}

// runLoop is the internal orchestration entry point. It handles logger creation,
// tracks running state, and enforces debug level, and supervises the lifecycle loop.
func (s *Service) runLoop(ctx context.Context, opts Options) error {
	// Add error attributes to the context
	ctx = ae.WithOtelAttribute(ctx,
		semconv.ServiceNameKey.String(s.Name),
		semconv.ServiceVersionKey.String(s.Version),
		semconv.ServiceNamespaceKey.String(s.Namespace),
	)

	// Add service attributes to the context
	ctx = withName(ctx, s.Name)
	ctx = withVersion(ctx, s.Version)
	ctx = withNamespace(ctx, s.Namespace)
	ctx = withEnvPrefix(ctx, opts.EnvPrefix)

	// Create initial logger
	ctx = WithLogger(ctx, initLogger(ctx, opts))

	if s.running.Swap(true) {
		return ae.MsgC(ctx, "already running")
	}

	graceStart := time.Now()
	graceCount := 1

	for {
		err, isInternal, isPanic := s.runOnce(ctx, opts)
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
			logAttrs = append(logAttrs, "grace_count", opts.GraceCount, "grace_counter", graceCount)
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

		logAttrs = append(logAttrs, "restart_delay", restartDelay)

		if restartDelay > 0 {
			Logger(ctx).Error("service failed, restarting after delay", logAttrs...)
			time.Sleep(restartDelay)
		} else {
			Logger(ctx).Error("service failed, restarting immediately", logAttrs...)
		}
	}
}

func (s *Service) runOnce(ctx context.Context, opts Options) (err error, isInternal bool, isPanic bool) {
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
	ctx, err, isInternal = s.init(ctx, opts)
	if err != nil {
		return ae.WrapC(ctx, "service initialization failed", err), isInternal, false
	}

	Logger(ctx).Debug("starting service")
	err, isInternal = s.run(ctx, opts)
	if err != nil {
		return ae.WrapC(ctx, "service failed", err), isInternal, false
	}

	// Cleanup is not returned as an error, since it's not critical.
	Logger(ctx).Debug("shutting down service")
	err = s.shutdown(ctx, opts)
	if err != nil {
		Logger(ctx).Error("service shutdown failed", "error", err)
	}

	return nil, false, false
}

// init runs the InitFunc if present, and can decorate or validate the context.
// Returns the possibly updated context and any error from initialization.
func (s *Service) init(ctx context.Context, opts Options) (context.Context, error, bool) {
	var err error
	ctx, s.otelShutdown, err = initOtel(ctx)
	if err != nil {
		return ctx, err, true
	}

	if s.InitFunc != nil {
		if err = s.InitFunc(ctx); err != nil {
			return ctx, err, false
		}
	}

	return ctx, nil, false
}

// run executes the RunFunc with the configured context and options.
// Returns any user or system error from execution.
func (s *Service) run(ctx context.Context, opts Options) (error, bool) {
	if s.RunFunc != nil {
		if err := s.RunFunc(ctx); err != nil {
			return err, false
		}
	}

	return nil, false
}

// cleanup invokes the CleanupFunc if defined, using the provided context and options.
// Returns any error during cleanup.
func (s *Service) shutdown(ctx context.Context, opts Options) error {
	var errs []error

	if s.ShutdownFunc != nil {
		if err := s.ShutdownFunc(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if err := s.otelShutdown(ctx); err != nil {
		errs = append(errs, err)
	}

	return ae.WrapMany("shutdown failed", errs...)
}
