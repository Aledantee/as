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

// Custom slog levels for the two non-standard names documented by
// Options.LogLevel ("fatal", "panic"). slog does not define these, so we
// position them above LevelError at the conventional offsets (+4 / +8).
const (
	levelFatal = slog.LevelError + 4
	levelPanic = slog.LevelError + 8
)

func initLogger(ctx context.Context, opts Options) *slog.Logger {
	level := slog.LevelInfo

	switch opts.LogLevel {
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	case "debug":
		level = slog.LevelDebug
	case "fatal":
		level = levelFatal
	case "panic":
		level = levelPanic
	default:
		level = slog.LevelInfo
	}

	if opts.LogDebug {
		level = slog.LevelDebug
		// Per Options.LogDebug docstring: "Implicitly disables JSON logging
		// when enabled."
		opts.LogJson = false
	}

	var handler slog.Handler
	if opts.LogJson {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else if effectiveLogColors(opts) {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
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

// effectiveLogColors reports whether colored output should be used for the
// given options. Colors are enabled when LogColors is set explicitly, or
// when LogAutoColors / LogDebug is set and stdout is a terminal.
func effectiveLogColors(opts Options) bool {
	if opts.LogColors {
		return true
	}
	if opts.LogAutoColors || opts.LogDebug {
		return isatty.IsTerminal(os.Stdout.Fd())
	}
	return false
}
