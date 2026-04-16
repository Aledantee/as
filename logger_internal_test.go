package as

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// runWithStdoutCaptured redirects os.Stdout for the duration of fn and
// returns anything written to it. Tests that call this helper must not run
// in parallel with each other because os.Stdout is process-global.
func runWithStdoutCaptured(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = original
	captured := <-done
	_ = r.Close()

	return captured
}

func TestInitLogger_LogDebugDisablesJSON(t *testing.T) {
	// Docstring on LogDebug: "Implicitly disables JSON logging when enabled."
	opts := DefaultOptions()
	opts.LogDebug = true
	opts.LogJson = true
	opts.LogAutoColors = false
	opts.LogColors = false

	out := runWithStdoutCaptured(t, func() {
		logger := initLogger(context.Background(), opts)
		logger.Info("probe-message")
	})

	trimmed := strings.TrimSpace(out)
	if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"msg":"probe-message"`) {
		t.Errorf("output is JSON-formatted but LogDebug=true should disable JSON; got %q", trimmed)
	}
}

func TestInitLogger_LogDebugForcesDebugLevel(t *testing.T) {
	// Docstring on LogDebug: "Implicitly sets the log level to debug, ignoring
	// any other log level settings."
	opts := DefaultOptions()
	opts.LogDebug = true
	opts.LogLevel = "error" // should be overridden
	opts.LogJson = false
	opts.LogAutoColors = false
	opts.LogColors = false

	out := runWithStdoutCaptured(t, func() {
		logger := initLogger(context.Background(), opts)
		logger.Debug("probe-debug")
	})

	if !strings.Contains(out, "probe-debug") {
		t.Errorf("Debug message missing from output; LogDebug=true must enable debug level even when LogLevel=error. Got: %q", out)
	}
}

func TestInitLogger_LogLevelFatalSuppressesInfoAndWarn(t *testing.T) {
	// Docstring on LogLevel: "Valid values are: debug, info, warn, error,
	// fatal, panic". "fatal" should therefore be honored and treated as a
	// stricter level than error — Info and Warn records must be dropped.
	opts := DefaultOptions()
	opts.LogDebug = false // overrides the vcsModified-derived default
	opts.LogLevel = "fatal"
	opts.LogJson = false
	opts.LogAutoColors = false
	opts.LogColors = false

	out := runWithStdoutCaptured(t, func() {
		logger := initLogger(context.Background(), opts)
		logger.Info("probe-info")
		logger.Warn("probe-warn")
		logger.Log(context.Background(), slog.LevelError+4, "probe-fatal")
	})

	if strings.Contains(out, "probe-info") {
		t.Errorf("Info record appeared despite LogLevel=fatal; got %q", out)
	}
	if strings.Contains(out, "probe-warn") {
		t.Errorf("Warn record appeared despite LogLevel=fatal; got %q", out)
	}
}
