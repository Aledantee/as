package as_test

// This file covers Run, RunC, and their lifecycle/restart semantics. The
// RunAndExit / RunAndExitC wrappers are intentionally not exercised here:
// they call ae.Exit on error, which terminates the test process. They are
// thin wrappers over RunC, and the underlying logic is covered below.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.aledante.io/ae"
	"go.aledante.io/as"
)

// Isolate each subtest from env bleed coming from the test harness: all
// built-in Options are loaded from env with the normalized "NS_SVC_" prefix,
// so any such env var in the caller's environment would leak in.
func isolateEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"NS_SVC_RESTART_ON_ERROR",
		"NS_SVC_RESTART_ON_ERROR_DELAY",
		"NS_SVC_RESTART_ON_PANIC",
		"NS_SVC_RESTART_ON_PANIC_DELAY",
		"NS_SVC_RECOVER_PANIC",
		"NS_SVC_GRACE_PERIOD",
		"NS_SVC_GRACE_COUNT",
		"NS_SVC_SHUTDOWN_TIMEOUT",
		"NS_SVC_LOG_DEBUG",
		"NS_SVC_LOG_LEVEL",
		"NS_SVC_LOG_JSON",
		"NS_SVC_LOG_COLORS",
		"NS_SVC_LOG_COLORS_AUTO",
	} {
		t.Setenv(k, "")
	}
}

// ----- Validation -----

func TestRunC_RejectsEmptyName(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.NameStr = ""

	err := as.RunC(svc, context.Background())
	if err == nil {
		t.Fatal("RunC with empty Name() returned nil, want error")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error message = %q, want to mention 'name'", err.Error())
	}
	if svc.InitCallCount() != 0 || svc.RunCallCount() != 0 || svc.CloseCallCount() != 0 {
		t.Errorf("lifecycle methods were invoked despite validation failure: init=%d run=%d close=%d",
			svc.InitCallCount(), svc.RunCallCount(), svc.CloseCallCount())
	}
}

func TestRunC_RejectsEmptyNamespace(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.NamespaceStr = ""

	err := as.RunC(svc, context.Background())
	if err == nil {
		t.Fatal("RunC with empty Namespace() returned nil, want error")
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Errorf("error message = %q, want to mention 'namespace'", err.Error())
	}
}

// ----- Lifecycle -----

func TestRunC_CallsInitRunCloseInOrder(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	err := as.RunC(svc, context.Background())
	if err != nil {
		t.Fatalf("RunC returned unexpected error: %v", err)
	}

	got := svc.Calls()
	want := []string{"init", "run", "close"}
	if !equalStrings(got, want) {
		t.Errorf("call order = %v, want %v", got, want)
	}
}

func TestRunC_ContextCarriesServiceIdentity(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.NameStr = "my-name"
	svc.NamespaceStr = "my-ns"
	svc.VersionStr = "9.9.9"

	err := as.RunC(svc, context.Background())
	if err != nil {
		t.Fatalf("RunC returned error: %v", err)
	}

	// Each ctx passed to Init/Run/Close must carry the documented values.
	for name, ctx := range map[string]context.Context{
		"init":  svc.InitCtx(0),
		"run":   svc.RunCtx(0),
		"close": svc.CloseCtx(0),
	} {
		if ctx == nil {
			t.Fatalf("%s ctx was nil", name)
		}
		if got := as.Name(ctx); got != svc.NameStr {
			t.Errorf("%s: Name(ctx) = %q, want %q", name, got, svc.NameStr)
		}
		if got := as.Namespace(ctx); got != svc.NamespaceStr {
			t.Errorf("%s: Namespace(ctx) = %q, want %q", name, got, svc.NamespaceStr)
		}
		if got := as.Version(ctx); got != svc.VersionStr {
			t.Errorf("%s: Version(ctx) = %q, want %q", name, got, svc.VersionStr)
		}
		if got := as.Logger(ctx); got == nil {
			t.Errorf("%s: Logger(ctx) returned nil", name)
		}
		// Default prefix is NormalizeEnvKey("<ns>_<name>_") + "_"
		if got := as.EnvPrefix(ctx); got != "MY_NS_MY_NAME_" {
			t.Errorf("%s: EnvPrefix(ctx) = %q, want %q", name, got, "MY_NS_MY_NAME_")
		}
	}
}

func TestRunC_RunReturningContextCanceledIsGraceful(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error { return context.Canceled }

	err := as.RunC(svc, context.Background())
	if err != nil {
		t.Errorf("RunC returned error %v, want nil (context.Canceled from Run must be graceful)", err)
	}

	// Close should still be called for cleanup (documented).
	if svc.CloseCallCount() != 1 {
		t.Errorf("Close call count = %d, want 1 (Close must run even on Canceled)", svc.CloseCallCount())
	}
}

// ----- Error propagation -----

func TestRunC_InitErrorWithRestartDisabledStopsImmediately(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.InitFn = func(ctx context.Context, _ int) error { return errors.New("boom") }

	err := as.RunC(svc, context.Background(), as.WithRestartOnError(false))
	if err == nil {
		t.Fatal("RunC returned nil, want error from Init")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %q, want it to wrap 'boom'", err.Error())
	}
	if svc.RunCallCount() != 0 {
		t.Errorf("Run was called %d times despite Init failure; want 0", svc.RunCallCount())
	}
}

func TestRunC_NonRecoverableErrorSkipsRestart(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error {
		return ae.New().Fatal().Msg("unrecoverable")
	}

	err := as.RunC(svc, context.Background(),
		as.WithRestartOnError(true),
		as.WithRestartOnErrorDelay(0),
		as.WithGraceCount(5),
	)
	if err == nil {
		t.Fatal("RunC returned nil, want error")
	}
	if svc.RunCallCount() != 1 {
		t.Errorf("Run called %d times, want exactly 1 (non-recoverable errors must not restart)", svc.RunCallCount())
	}
	// Per README: each iteration is (init → run → close). Close must run
	// after Run even on failure, so resources acquired in Init are released.
	if svc.CloseCallCount() != 1 {
		t.Errorf("Close called %d times, want 1 (Close must run after Run even on failure)", svc.CloseCallCount())
	}
}

func TestRunC_RestartsOnRecoverableErrorUpToGraceCount(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error { return errors.New("transient") }

	err := as.RunC(svc, context.Background(),
		as.WithRestartOnError(true),
		as.WithRestartOnErrorDelay(0),
		as.WithGracePeriod(0),
		as.WithGraceCount(2),
	)
	if err == nil {
		t.Fatal("RunC returned nil, want error after exhausting restarts")
	}
	// 1 initial call + 2 allowed restarts = 3 total Run invocations.
	if got := svc.RunCallCount(); got != 3 {
		t.Errorf("Run call count = %d, want 3 (initial + GraceCount=2 restarts)", got)
	}
	// Per README: each iteration is (init → run → close). Close runs
	// once per iteration, so 3 Run invocations imply 3 Close invocations.
	if got := svc.CloseCallCount(); got != 3 {
		t.Errorf("Close call count = %d, want 3 (one Close per iteration, incl. failing ones)", got)
	}
	// The call order should reflect the documented cycle repeated three times.
	want := []string{"init", "run", "close", "init", "run", "close", "init", "run", "close"}
	if got := svc.Calls(); !equalStrings(got, want) {
		t.Errorf("call sequence = %v, want %v", got, want)
	}
}

func TestRunC_StopsRestartingAfterGracePeriod(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error { return errors.New("transient") }

	gracePeriod := 50 * time.Millisecond
	start := time.Now()
	err := as.RunC(svc, context.Background(),
		as.WithRestartOnError(true),
		as.WithRestartOnErrorDelay(10*time.Millisecond),
		as.WithGracePeriod(gracePeriod),
		as.WithGraceCount(0), // disabled; only grace period limits
	)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("RunC returned nil, want error after exceeding grace period")
	}
	if elapsed < gracePeriod {
		t.Errorf("elapsed = %v, want at least %v (loop must honor grace period)", elapsed, gracePeriod)
	}
	// Loose upper bound to catch runaway restart storms.
	if elapsed > 2*time.Second {
		t.Errorf("elapsed = %v, far exceeds grace period %v", elapsed, gracePeriod)
	}
	// One Close per iteration, regardless of how many iterations ran.
	if runs, closes := svc.RunCallCount(), svc.CloseCallCount(); runs != closes {
		t.Errorf("Run=%d Close=%d, want equal (one Close per iteration)", runs, closes)
	}
}

// ----- Panic handling -----

func TestRunC_RecoverPanicConvertsToError(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error { panic("explode") }

	err := as.RunC(svc, context.Background(),
		as.WithRecoverPanic(true),
		as.WithRestartOnPanic(false),
		as.WithRestartOnError(false),
	)
	if err == nil {
		t.Fatal("RunC returned nil, want converted panic error")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("error = %q, want it to reference 'panic'", err.Error())
	}
	// A panic is still a Run completion from the cycle's perspective: Close
	// must be invoked to release whatever Init acquired.
	if svc.CloseCallCount() != 1 {
		t.Errorf("Close called %d times, want 1 (Close must run after a recovered panic)", svc.CloseCallCount())
	}
}

func TestRunC_NoRecoverPanicPropagates(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error { panic("boom") }

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate with RecoverPanic=false, but no panic occurred")
		}
	}()

	_ = as.RunC(svc, context.Background(),
		as.WithRecoverPanic(false),
		as.WithRestartOnError(false),
	)
}

func TestRunC_PanicDelayFallsBackToErrorDelayWhenZero(t *testing.T) {
	isolateEnv(t)

	errorDelay := 30 * time.Millisecond

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, call int) error {
		if call == 1 {
			panic("once")
		}
		return nil
	}

	start := time.Now()
	err := as.RunC(svc, context.Background(),
		as.WithRecoverPanic(true),
		as.WithRestartOnError(true),
		as.WithRestartOnPanic(true),
		as.WithRestartOnErrorDelay(errorDelay),
		as.WithRestartOnPanicDelay(0), // per docstring: fall back to error delay
		as.WithGraceCount(2),
	)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunC returned %v, want nil (service recovers on second run)", err)
	}
	if elapsed < errorDelay {
		t.Errorf("elapsed = %v, want at least %v (panic delay should fall back to error delay)", elapsed, errorDelay)
	}
	// Two iterations (the panic one and the recovering one), each ending in Close.
	if svc.CloseCallCount() != 2 {
		t.Errorf("Close called %d times, want 2 (one per iteration incl. the panicking one)", svc.CloseCallCount())
	}
}

func TestRunC_RestartOnPanicDisabledDoesNotRestart(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error { panic("nope") }

	err := as.RunC(svc, context.Background(),
		as.WithRecoverPanic(true),
		as.WithRestartOnError(true),
		as.WithRestartOnPanic(false),
		as.WithGraceCount(5),
	)
	if err == nil {
		t.Fatal("RunC returned nil, want error")
	}
	if got := svc.RunCallCount(); got != 1 {
		t.Errorf("Run call count = %d, want 1 (RestartOnPanic=false must prevent restart)", got)
	}
	if svc.CloseCallCount() != 1 {
		t.Errorf("Close called %d times, want 1 (Close must run after a recovered panic)", svc.CloseCallCount())
	}
}

// ----- Close -----

func TestRunC_CloseErrorDoesNotAffectReturn(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	svc.CloseFn = func(ctx context.Context, _ int) error { return errors.New("close failed") }

	err := as.RunC(svc, context.Background())
	if err != nil {
		t.Errorf("RunC returned %v, want nil (Close errors must be logged only, not returned)", err)
	}
}

// ----- Caller context propagation -----

// ctxMarkerKey is a unique type used as a context.Value key in the test below.
type ctxMarkerKey struct{}

func TestRunC_CallerContextValuesReachService(t *testing.T) {
	isolateEnv(t)

	// Values attached to the caller's ctx must be visible inside the service
	// callbacks. RunC's doc implies the passed ctx is the parent of the
	// service context; if RunC discards it, this assertion fails.
	parent := context.WithValue(context.Background(), ctxMarkerKey{}, "marker")

	svc := newValidService()
	if err := as.RunC(svc, parent); err != nil {
		t.Fatalf("RunC: %v", err)
	}

	runCtx := svc.RunCtx(0)
	if runCtx == nil {
		t.Fatal("Run was not called")
	}
	if got := runCtx.Value(ctxMarkerKey{}); got != "marker" {
		t.Errorf("value from caller ctx not present in Run ctx: got %v, want %q (RunC must derive from the caller's ctx)", got, "marker")
	}
}

func TestRunC_CallerContextCancellationStopsService(t *testing.T) {
	isolateEnv(t)

	// Cancelling the caller's ctx must propagate to the service so that
	// long-running Run/Close implementations can observe the cancellation
	// via <-ctx.Done(). If RunC rebuilds a fresh ctx from Background, this
	// will deadlock until the test timeout.
	parent, cancel := context.WithCancel(context.Background())

	svc := newValidService()
	svc.RunFn = func(ctx context.Context, _ int) error {
		<-ctx.Done()
		return ctx.Err()
	}

	done := make(chan error, 1)
	go func() { done <- as.RunC(svc, parent) }()

	// Give Run a moment to reach the blocking select, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		// Cancellation from the caller's ctx is graceful (documented for the
		// signal path; intent is the same for the ctx path).
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("RunC after caller-ctx cancel: err = %v, want nil or context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunC did not return after caller ctx was cancelled")
	}
}

// ----- Shutdown timeout -----

func TestRunC_ShutdownTimeoutAppliesDeadlineToCloseCtx(t *testing.T) {
	isolateEnv(t)

	// Per the ShutdownTimeout docstring, a graceful shutdown longer than
	// the timeout is "forcefully terminated". The natural Go interpretation
	// is that the ctx passed to Close carries a deadline derived from
	// ShutdownTimeout, so a well-behaved Close can honour it via <-ctx.Done().
	timeout := 50 * time.Millisecond

	svc := newValidService()

	err := as.RunC(svc, context.Background(), as.WithShutdownTimeout(timeout))
	if err != nil {
		t.Fatalf("RunC: %v", err)
	}

	closeCtx := svc.CloseCtx(0)
	if closeCtx == nil {
		t.Fatal("Close was not invoked")
	}

	deadline, ok := closeCtx.Deadline()
	if !ok {
		t.Fatal("Close ctx has no deadline; ShutdownTimeout should impose one")
	}
	if until := time.Until(deadline); until > timeout+500*time.Millisecond {
		t.Errorf("Close ctx deadline is %v in the future, want ~%v (ShutdownTimeout)", until, timeout)
	}
}

// ----- Run (background-context wrapper) -----

func TestRun_InvokesService(t *testing.T) {
	isolateEnv(t)

	svc := newValidService()
	if err := as.Run(svc); err != nil {
		t.Fatalf("Run returned %v, want nil", err)
	}
	if svc.RunCallCount() != 1 {
		t.Errorf("Run call count = %d, want 1", svc.RunCallCount())
	}
}

// ----- helpers -----

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
