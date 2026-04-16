package as

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"go.aledante.io/ae"
)

// captureStdout redirects os.Stdout to a pipe for the duration of fn and
// returns whatever was written. The pipe makes stdout not-a-TTY, which keeps
// any isatty-based decisions deterministic.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()
	_ = w.Close()
	return <-done
}

func TestIsFrameworkFrame(t *testing.T) {
	t.Parallel()

	cases := []struct {
		frame *ae.StackFrame
		want  bool
	}{
		{nil, false},
		{&ae.StackFrame{Func: "go.aledante.io/as.RunC"}, true},
		{&ae.StackFrame{Func: "go.aledante.io/as.(*foo).bar"}, true},
		{&ae.StackFrame{Func: "go.aledante.io/asx.Other"}, false},
		{&ae.StackFrame{Func: "example.com/myapp/svc.Run"}, false},
		{&ae.StackFrame{Func: "go.aledante.io/ae.New"}, false},
	}

	for _, tc := range cases {
		got := isFrameworkFrame(tc.frame)
		if got != tc.want {
			name := "<nil>"
			if tc.frame != nil {
				name = tc.frame.Func
			}
			t.Errorf("isFrameworkFrame(%q) = %v, want %v", name, got, tc.want)
		}
	}
}

func TestEffectiveLogColors(t *testing.T) {
	// Force stdout to a pipe so isatty.IsTerminal returns false; this is the
	// only deterministic state for tests run in arbitrary environments.
	_ = captureStdout(t, func() {
		cases := []struct {
			name string
			opts Options
			want bool
		}{
			{"explicit colors wins", Options{LogColors: true}, true},
			{"auto without TTY is off", Options{LogAutoColors: true}, false},
			{"debug without TTY is off", Options{LogDebug: true}, false},
			{"all flags off is off", Options{}, false},
			{"explicit colors beats auto", Options{LogColors: true, LogAutoColors: true}, true},
		}

		for _, tc := range cases {
			if got := effectiveLogColors(tc.opts); got != tc.want {
				t.Errorf("%s: effectiveLogColors = %v, want %v", tc.name, got, tc.want)
			}
		}
	})
}

func TestPrintRunError_JsonModeProducesJson(t *testing.T) {
	out := captureStdout(t, func() {
		printRunError(errors.New("boom"), Options{LogJson: true})
	})

	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "{") {
		t.Errorf("output does not start with '{': %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("output missing message %q: %q", "boom", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("JSON output contains ANSI escapes: %q", out)
	}
}

func TestPrintRunError_NoColorsByDefault(t *testing.T) {
	out := captureStdout(t, func() {
		printRunError(errors.New("boom"), Options{})
	})

	if strings.Contains(out, "\x1b[") {
		t.Errorf("output contains ANSI escapes when colors are off: %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("output missing message %q: %q", "boom", out)
	}
}

func TestPrintRunError_ColorsWhenEnabled(t *testing.T) {
	// fatih/color disables ANSI when stdout is not a TTY (which it isn't
	// under captureStdout's pipe). Force it on for this test.
	prev := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = prev })

	out := captureStdout(t, func() {
		printRunError(ae.New().Msg("boom"), Options{LogColors: true})
	})

	if !strings.Contains(out, "\x1b[") {
		t.Errorf("output missing ANSI escapes when LogColors=true: %q", out)
	}
}
