package as_test

import (
	"testing"
	"time"

	"go.aledante.io/as"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	got := as.DefaultOptions()
	// Per the docstrings on Options fields. LogLevel is covered separately
	// because its documented "info" default is supplied via envDefault, not
	// by DefaultOptions itself. LogDebug is covered separately because its
	// documented default depends on VCS state.
	want := as.Options{
		RestartOnError:      true,
		RestartOnErrorDelay: 10 * time.Second,
		RestartOnPanic:      true,
		RestartOnPanicDelay: 0,
		RecoverPanic:        true,
		GracePeriod:         1 * time.Minute,
		GraceCount:          3,
		ShutdownTimeout:     30 * time.Second,
		LogJson:             true,
		LogColors:           false,
		LogAutoColors:       true,
		EnvPrefix:           "",
		DisableEnvPrefix:    false,
	}

	if got.RestartOnError != want.RestartOnError {
		t.Errorf("RestartOnError = %v, want %v", got.RestartOnError, want.RestartOnError)
	}
	if got.RestartOnErrorDelay != want.RestartOnErrorDelay {
		t.Errorf("RestartOnErrorDelay = %v, want %v", got.RestartOnErrorDelay, want.RestartOnErrorDelay)
	}
	if got.RestartOnPanic != want.RestartOnPanic {
		t.Errorf("RestartOnPanic = %v, want %v", got.RestartOnPanic, want.RestartOnPanic)
	}
	if got.RestartOnPanicDelay != want.RestartOnPanicDelay {
		t.Errorf("RestartOnPanicDelay = %v, want %v", got.RestartOnPanicDelay, want.RestartOnPanicDelay)
	}
	if got.RecoverPanic != want.RecoverPanic {
		t.Errorf("RecoverPanic = %v, want %v", got.RecoverPanic, want.RecoverPanic)
	}
	if got.GracePeriod != want.GracePeriod {
		t.Errorf("GracePeriod = %v, want %v", got.GracePeriod, want.GracePeriod)
	}
	if got.GraceCount != want.GraceCount {
		t.Errorf("GraceCount = %v, want %v", got.GraceCount, want.GraceCount)
	}
	if got.ShutdownTimeout != want.ShutdownTimeout {
		t.Errorf("ShutdownTimeout = %v, want %v", got.ShutdownTimeout, want.ShutdownTimeout)
	}
	if got.LogJson != want.LogJson {
		t.Errorf("LogJson = %v, want %v", got.LogJson, want.LogJson)
	}
	if got.LogColors != want.LogColors {
		t.Errorf("LogColors = %v, want %v", got.LogColors, want.LogColors)
	}
	if got.LogAutoColors != want.LogAutoColors {
		t.Errorf("LogAutoColors = %v, want %v", got.LogAutoColors, want.LogAutoColors)
	}
	if got.EnvPrefix != want.EnvPrefix {
		t.Errorf("EnvPrefix = %q, want %q", got.EnvPrefix, want.EnvPrefix)
	}
	if got.DisableEnvPrefix != want.DisableEnvPrefix {
		t.Errorf("DisableEnvPrefix = %v, want %v", got.DisableEnvPrefix, want.DisableEnvPrefix)
	}
}

func TestWithOptionHelpers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		apply  as.Option
		verify func(t *testing.T, o as.Options)
	}{
		{
			name:   "WithRestartOnError sets field",
			apply:  as.WithRestartOnError(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "RestartOnError", o.RestartOnError, true) },
		},
		{
			name:   "WithRestartOnError can disable",
			apply:  as.WithRestartOnError(false),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "RestartOnError", o.RestartOnError, false) },
		},
		{
			name:   "WithRestartOnErrorDelay",
			apply:  as.WithRestartOnErrorDelay(7 * time.Second),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "RestartOnErrorDelay", o.RestartOnErrorDelay, 7*time.Second) },
		},
		{
			name:   "WithRestartOnPanic",
			apply:  as.WithRestartOnPanic(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "RestartOnPanic", o.RestartOnPanic, true) },
		},
		{
			name:   "WithRestartOnPanicDelay",
			apply:  as.WithRestartOnPanicDelay(3 * time.Second),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "RestartOnPanicDelay", o.RestartOnPanicDelay, 3*time.Second) },
		},
		{
			name:   "WithRecoverPanic",
			apply:  as.WithRecoverPanic(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "RecoverPanic", o.RecoverPanic, true) },
		},
		{
			name:   "WithGracePeriod",
			apply:  as.WithGracePeriod(5 * time.Minute),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "GracePeriod", o.GracePeriod, 5*time.Minute) },
		},
		{
			name:   "WithGraceCount",
			apply:  as.WithGraceCount(9),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "GraceCount", o.GraceCount, 9) },
		},
		{
			name:   "WithShutdownTimeout",
			apply:  as.WithShutdownTimeout(15 * time.Second),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "ShutdownTimeout", o.ShutdownTimeout, 15*time.Second) },
		},
		{
			name:   "WithLogDebug",
			apply:  as.WithLogDebug(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "LogDebug", o.LogDebug, true) },
		},
		{
			name:   "WithLogJson",
			apply:  as.WithLogJson(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "LogJson", o.LogJson, true) },
		},
		{
			name:   "WithLogColors",
			apply:  as.WithLogColors(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "LogColors", o.LogColors, true) },
		},
		{
			name:   "WithLogAutoColors",
			apply:  as.WithLogAutoColors(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "LogAutoColors", o.LogAutoColors, true) },
		},
		{
			name:   "WithDisableEnvPrefix",
			apply:  as.WithDisableEnvPrefix(true),
			verify: func(t *testing.T, o as.Options) { mustEqual(t, "DisableEnvPrefix", o.DisableEnvPrefix, true) },
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var o as.Options
			tc.apply(&o)
			tc.verify(t, o)
		})
	}
}

func mustEqual[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}
