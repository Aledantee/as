package as

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/fatih/color"
	"go.aledante.io/ae"
)

// TestPrintProbe is a scratch file to dump the pretty-print output for many
// error shapes. It always passes; use `go test -run TestPrintProbe -v` to view.
func TestPrintProbe(t *testing.T) {
	// Force colors off so we see the plain-text rendering pipeline.
	prev := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prev })

	cases := []struct {
		name string
		err  error
	}{
		{
			"stdlib errors.New",
			errors.New("something went wrong"),
		},
		{
			"ae message only",
			ae.New().Msg("basic message"),
		},
		{
			"ae code + exit code",
			ae.New().Code("E_CFG").ExitCode(2).Msg("config broken"),
		},
		{
			"ae exit code only (no code)",
			ae.New().ExitCode(5).Msg("bad exit"),
		},
		{
			"ae tags",
			ae.New().Tags("net", "retryable").Msg("upstream failed"),
		},
		{
			"ae hint",
			ae.New().Hint("try --verbose").Msg("parsing failed"),
		},
		{
			"ae with single attribute (string)",
			ae.New().Attr("host", "db-01").Msg("connection refused"),
		},
		{
			"ae with attribute (non-string value)",
			ae.New().Attr("port", 5432).Attr("retries", 3).Msg("connection refused"),
		},
		{
			"ae with many attributes",
			ae.New().
				Attr("host", "db-01").
				Attr("port", 5432).
				Attr("user", "svc").
				Attr("attempt", 7).
				Attr("region", "eu-west-1").
				Msg("connection refused"),
		},
		{
			"ae with user message",
			ae.New().UserMsg("internal failure", "Something broke, please retry."),
		},
		{
			"ae with timestamp",
			ae.New().Timestamp(time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)).Msg("timed event"),
		},
		{
			"ae with single cause (stdlib)",
			ae.New().Cause(errors.New("root cause")).Msg("operation failed"),
		},
		{
			"ae with single cause (ae)",
			ae.New().Cause(ae.New().Code("E_DB").Msg("db down")).Msg("operation failed"),
		},
		{
			"ae with multiple causes",
			ae.New().
				Cause(errors.New("first")).
				Cause(errors.New("second")).
				Cause(errors.New("third")).
				Msg("many causes"),
		},
		{
			"ae nested causes",
			ae.New().
				Cause(ae.New().
					Cause(ae.New().
						Cause(errors.New("deep root")).
						Msg("mid")).
					Msg("upper")).
				Msg("top"),
		},
		{
			"ae related error",
			ae.New().Related(errors.New("cleanup failure")).Msg("shutdown broke"),
		},
		{
			"ae cause + related",
			ae.New().
				Cause(errors.New("primary fault")).
				Related(errors.New("secondary side-effect")).
				Msg("combined failure"),
		},
		{
			"ae with stack",
			ae.New().Stack().Msg("with stack"),
		},
		{
			"ae with everything (no user msg)",
			ae.New().
				Code("E_EVERYTHING").
				ExitCode(42).
				Tags("kitchen-sink", "verbose").
				Hint("check the manual").
				Attr("attempt", 3).
				Cause(errors.New("root")).
				Related(errors.New("cleanup failed")).
				Stack().
				Msg("combined"),
		},
		{
			"empty message with only causes",
			ae.New().Cause(errors.New("child")).Msg(""),
		},
		{
			"joined errors via errors.Join as cause",
			ae.New().Cause(errors.Join(errors.New("a"), errors.New("b"))).Msg("joined"),
		},
		{
			"cause with its own related (recursion behavior)",
			ae.New().Cause(
				ae.New().Related(errors.New("sibling-of-cause")).Msg("cause with related"),
			).Msg("top"),
		},
	}

	for _, tc := range cases {
		out := captureStdout(t, func() {
			printRunError(tc.err, Options{})
		})
		fmt.Fprintf(os.Stderr, "=== case: %s ===\n%s\n", tc.name, out)
	}
}

// captureStdoutN is a slightly richer capture helper for multi-reads.
var _ = io.Discard
