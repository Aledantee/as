package as_test

import (
	"context"
	"sync"
)

// testService is a configurable fake implementation of as.Service used across
// the test suite. Tests customize behavior via InitFn, RunFn, CloseFn; call
// counts and captured contexts are available via the recorded methods.
type testService struct {
	NameStr      string
	NamespaceStr string
	VersionStr   string

	InitFn  func(ctx context.Context, call int) error
	RunFn   func(ctx context.Context, call int) error
	CloseFn func(ctx context.Context, call int) error

	mu         sync.Mutex
	calls      []string
	initCalls  int
	runCalls   int
	closeCalls int
	initCtxs   []context.Context
	runCtxs    []context.Context
	closeCtxs  []context.Context
}

func (s *testService) Name() string      { return s.NameStr }
func (s *testService) Namespace() string { return s.NamespaceStr }
func (s *testService) Version() string   { return s.VersionStr }

func (s *testService) Init(ctx context.Context) error {
	s.mu.Lock()
	s.calls = append(s.calls, "init")
	s.initCalls++
	s.initCtxs = append(s.initCtxs, ctx)
	fn := s.InitFn
	call := s.initCalls
	s.mu.Unlock()

	if fn != nil {
		return fn(ctx, call)
	}
	return nil
}

func (s *testService) Run(ctx context.Context) error {
	s.mu.Lock()
	s.calls = append(s.calls, "run")
	s.runCalls++
	s.runCtxs = append(s.runCtxs, ctx)
	fn := s.RunFn
	call := s.runCalls
	s.mu.Unlock()

	if fn != nil {
		return fn(ctx, call)
	}
	return nil
}

func (s *testService) Close(ctx context.Context) error {
	s.mu.Lock()
	s.calls = append(s.calls, "close")
	s.closeCalls++
	s.closeCtxs = append(s.closeCtxs, ctx)
	fn := s.CloseFn
	call := s.closeCalls
	s.mu.Unlock()

	if fn != nil {
		return fn(ctx, call)
	}
	return nil
}

func (s *testService) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *testService) InitCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initCalls
}

func (s *testService) RunCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runCalls
}

func (s *testService) CloseCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeCalls
}

func (s *testService) InitCtx(i int) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i >= len(s.initCtxs) {
		return nil
	}
	return s.initCtxs[i]
}

func (s *testService) RunCtx(i int) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i >= len(s.runCtxs) {
		return nil
	}
	return s.runCtxs[i]
}

func (s *testService) CloseCtx(i int) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i >= len(s.closeCtxs) {
		return nil
	}
	return s.closeCtxs[i]
}

// newValidService returns a testService with non-empty identity fields so
// RunC's validation passes. Behavior defaults to nil (=> each lifecycle
// callback returns nil), which causes Run to exit immediately.
func newValidService() *testService {
	return &testService{
		NameStr:      "svc",
		NamespaceStr: "ns",
		VersionStr:   "1.0.0",
	}
}
