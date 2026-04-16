package as_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.aledante.io/as"
)

func TestRunGroupC_RejectsEmptyGroup(t *testing.T) {
	isolateEnv(t)

	err := as.RunGroupC(nil, context.Background())
	if err == nil {
		t.Fatal("RunGroupC(nil) returned nil, want error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want it to mention 'empty'", err.Error())
	}
}

func TestRunGroupC_RejectsDuplicateServices(t *testing.T) {
	isolateEnv(t)

	a := newValidService()
	b := newValidService()

	err := as.RunGroupC([]as.Service{a, b}, context.Background())
	if err == nil {
		t.Fatal("RunGroupC with duplicates returned nil, want error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want it to mention 'duplicate'", err.Error())
	}
	// No lifecycle callbacks should have been invoked.
	if a.InitCallCount()+b.InitCallCount() != 0 {
		t.Errorf("Init was called despite duplicate validation failure")
	}
}

func TestRunGroupC_RejectsInvalidService(t *testing.T) {
	isolateEnv(t)

	valid := newValidService()
	invalid := newValidService()
	invalid.NameStr = ""

	err := as.RunGroupC([]as.Service{valid, invalid}, context.Background())
	if err == nil {
		t.Fatal("RunGroupC with invalid service returned nil, want error")
	}
	if valid.InitCallCount()+invalid.InitCallCount() != 0 {
		t.Errorf("Init was called despite validation failure")
	}
}

func TestRunGroupC_RunsAllServicesToCompletion(t *testing.T) {
	isolateEnv(t)

	a := newValidService()
	a.NameStr = "svc-a"
	b := newValidService()
	b.NameStr = "svc-b"

	err := as.RunGroupC([]as.Service{a, b}, context.Background())
	if err != nil {
		t.Fatalf("RunGroupC: %v", err)
	}
	for name, svc := range map[string]*testService{"a": a, "b": b} {
		if svc.InitCallCount() != 1 || svc.RunCallCount() != 1 || svc.CloseCallCount() != 1 {
			t.Errorf("%s lifecycle counts: init=%d run=%d close=%d, want 1/1/1",
				name, svc.InitCallCount(), svc.RunCallCount(), svc.CloseCallCount())
		}
	}
}

func TestRunGroupC_ErrorInOneCancelsPeers(t *testing.T) {
	isolateEnv(t)

	// Peer A fails fast with a fatal error. Peer B blocks on ctx.Done
	// and must be released by the shared ctx cancellation the group performs.
	a := newValidService()
	a.NameStr = "faulter"
	a.RunFn = func(ctx context.Context, _ int) error { return errors.New("boom") }

	var bDone atomic.Bool
	b := newValidService()
	b.NameStr = "blocker"
	b.RunFn = func(ctx context.Context, _ int) error {
		<-ctx.Done()
		bDone.Store(true)
		return ctx.Err()
	}

	done := make(chan error, 1)
	go func() {
		done <- as.RunGroupC([]as.Service{a, b}, context.Background(),
			as.WithRestartOnError(false),
		)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("RunGroupC returned nil, want error (peer A failed)")
		}
		if !bDone.Load() {
			t.Error("peer B never observed cancellation; shared ctx must be cancelled when a peer errors")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunGroupC did not return; peer B may be stuck")
	}
}

func TestRunGroupC_CallerContextCancellationStopsAll(t *testing.T) {
	isolateEnv(t)

	parent, cancel := context.WithCancel(context.Background())

	a := newValidService()
	a.NameStr = "a"
	a.RunFn = func(ctx context.Context, _ int) error {
		<-ctx.Done()
		return ctx.Err()
	}
	b := newValidService()
	b.NameStr = "b"
	b.RunFn = func(ctx context.Context, _ int) error {
		<-ctx.Done()
		return ctx.Err()
	}

	done := make(chan error, 1)
	go func() { done <- as.RunGroupC([]as.Service{a, b}, parent) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("RunGroupC after caller cancel: err = %v, want nil or context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunGroupC did not return after caller ctx was cancelled")
	}
}
