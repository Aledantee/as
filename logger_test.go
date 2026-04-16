package as_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"go.aledante.io/as"
)

func TestLogger_ReturnsDefaultWhenUnset(t *testing.T) {
	t.Parallel()

	got := as.Logger(context.Background())
	if got != slog.Default() {
		t.Errorf("Logger(background) = %p, want slog.Default() = %p", got, slog.Default())
	}
}

func TestWithLogger_StoresProvidedLogger(t *testing.T) {
	t.Parallel()

	custom := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := as.WithLogger(context.Background(), custom)
	if got := as.Logger(ctx); got != custom {
		t.Errorf("Logger(ctx) = %p, want custom logger %p", got, custom)
	}
}

func TestWithLogger_NilWithNoExistingUsesDefault(t *testing.T) {
	t.Parallel()

	ctx := as.WithLogger(context.Background(), nil)
	if got := as.Logger(ctx); got != slog.Default() {
		t.Errorf("Logger after WithLogger(bg, nil) = %p, want slog.Default()", got)
	}
}

func TestWithLogger_NilPreservesExistingLogger(t *testing.T) {
	t.Parallel()

	custom := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := as.WithLogger(context.Background(), custom)
	ctx = as.WithLogger(ctx, nil)

	if got := as.Logger(ctx); got != custom {
		t.Errorf("Logger after WithLogger(_, nil) on ctx with existing logger = %p, want custom %p", got, custom)
	}
}
