package as

import (
	"testing"
	"time"
)

func TestApplyOptions_DefaultsAreMergedWhenEnvIsEmpty(t *testing.T) {
	// No env vars set with the derived prefix: option helpers take effect.
	got := applyOptions("svc", "ns", []Option{
		WithGraceCount(5),
		WithRestartOnErrorDelay(time.Millisecond),
	})

	if got.GraceCount != 5 {
		t.Errorf("GraceCount = %d, want 5 (from option helper)", got.GraceCount)
	}
	if got.RestartOnErrorDelay != time.Millisecond {
		t.Errorf("RestartOnErrorDelay = %v, want %v", got.RestartOnErrorDelay, time.Millisecond)
	}
	// Options that were not set should retain the DefaultOptions values.
	if !got.RestartOnError {
		t.Errorf("RestartOnError = false, want true (default)")
	}
}

func TestApplyOptions_LogLevelDefaultsToInfo(t *testing.T) {
	got := applyOptions("svc", "ns", nil)
	if got.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q (per LogLevel envDefault / docstring)", got.LogLevel, "info")
	}
}

func TestApplyOptions_EnvOverridesOptionHelpers(t *testing.T) {
	t.Setenv("NS_SVC_GRACE_COUNT", "7")

	got := applyOptions("svc", "ns", []Option{WithGraceCount(1)})
	if got.GraceCount != 7 {
		t.Errorf("GraceCount = %d, want 7 (env must override option helper)", got.GraceCount)
	}
}

func TestApplyOptions_DefaultPrefix(t *testing.T) {
	got := applyOptions("svc", "ns", nil)
	if got.EnvPrefix != "NS_SVC_" {
		t.Errorf("EnvPrefix = %q, want %q", got.EnvPrefix, "NS_SVC_")
	}
}

func TestApplyOptions_DefaultPrefix_NoNamespace(t *testing.T) {
	got := applyOptions("svc", "", nil)
	if got.EnvPrefix != "SVC_" {
		t.Errorf("EnvPrefix = %q, want %q", got.EnvPrefix, "SVC_")
	}
}

func TestApplyOptions_CustomEnvPrefixNormalized(t *testing.T) {
	got := applyOptions("svc", "ns", []Option{
		func(o *Options) { o.EnvPrefix = "my.odd-prefix" },
	})

	if got.EnvPrefix != "MY_ODD_PREFIX_" {
		t.Errorf("EnvPrefix = %q, want %q (should be normalized + '_' suffix)", got.EnvPrefix, "MY_ODD_PREFIX_")
	}
}

func TestDefaultOptions_LogDebugTracksVCSModified(t *testing.T) {
	// Per the LogDebug docstring: "Defaults to true when the source tree has
	// local modifications." The default must therefore mirror vcsModified().
	if got, want := DefaultOptions().LogDebug, vcsModified(); got != want {
		t.Errorf("DefaultOptions().LogDebug = %v, want %v (= vcsModified())", got, want)
	}
}

func TestApplyOptions_DisableEnvPrefix(t *testing.T) {
	// With DisableEnvPrefix, unprefixed env vars should fill in Options
	// and the effective EnvPrefix in the resulting Options should be empty.
	t.Setenv("GRACE_COUNT", "5")

	got := applyOptions("svc", "ns", []Option{WithDisableEnvPrefix(true)})
	if got.GraceCount != 5 {
		t.Errorf("GraceCount = %d, want 5 (unprefixed env must be picked up)", got.GraceCount)
	}
	if got.EnvPrefix != "" {
		t.Errorf("EnvPrefix = %q, want empty (DisableEnvPrefix must clear it)", got.EnvPrefix)
	}
}
