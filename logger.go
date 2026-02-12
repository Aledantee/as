package as

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
)

// loggerKey is the key used to store the logger in the context.
type loggerKey struct{}

// WithLogger returns a new context.Context that associates the provided logger with ctx.
// If logger is nil and no logger is already set in the context, slog.Default() is used.
// If logger is nil and a logger is already set, the context is left unchanged.
// Use Logger(ctx) to retrieve the logger later. This is intended for attaching a contextual logger to a request or service context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		_, ok := ctx.Value(loggerKey{}).(*slog.Logger)
		if ok {
			return ctx
		}

		return context.WithValue(ctx, loggerKey{}, slog.Default())
	}

	return context.WithValue(ctx, loggerKey{}, logger)
}

// Logger returns the service logger from the context.
// If no logger is set, a default logger is returned. This happens when the context is not created from the context
// of a service.
func Logger(ctx context.Context) *slog.Logger {
	v, ok := ctx.Value(loggerKey{}).(*slog.Logger)
	if !ok {
		return slog.Default()
	}

	return v
}

func initLogger(ctx context.Context, opts Options) *slog.Logger {
	level := slog.LevelInfo

	switch opts.LogLevel {
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	case "debug":
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	if opts.LogDebug {
		level = slog.LevelDebug
	}

	if opts.LogAutoColors || opts.LogDebug {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			opts.LogColors = true
		}
	}

	var handler slog.Handler
	if opts.LogJson {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		if opts.LogColors {
			handler = tint.NewHandler(os.Stdout, &tint.Options{
				Level: level,
			})
		} else {
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: level,
			})
		}
	}

	logger := slog.New(handler)

	if svcName := Name(ctx); svcName != "" {
		logger = logger.With("service", svcName)
	}
	if svcVersion := Version(ctx); svcVersion != "" {
		logger = logger.With("version", svcVersion)
	}
	if svcNamespace := Namespace(ctx); svcNamespace != "" {
		logger = logger.With("namespace", svcNamespace)
	}

	return logger
}
