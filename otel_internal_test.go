package as

import (
	"context"
	"strings"
	"testing"

	metricNoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

// otelProbeService records the ctx passed to Run and exercises the
// OpenTelemetry accessors from inside Run, as a real service would.
type otelProbeService struct {
	name   string
	runCtx context.Context
	runErr error
}

func (s *otelProbeService) Name() string                  { return s.name }
func (s *otelProbeService) Namespace() string             { return "ns" }
func (s *otelProbeService) Version() string               { return "1.0.0" }
func (s *otelProbeService) Init(context.Context) error    { return nil }
func (s *otelProbeService) Close(context.Context) error   { return nil }
func (s *otelProbeService) Run(ctx context.Context) error { return s.probe(ctx) }

func (s *otelProbeService) probe(ctx context.Context) error {
	s.runCtx = ctx

	_, span := Tracer(ctx).Start(ctx, "probe-span")
	span.End()

	counter, err := Meter(ctx).Int64Counter("probe.counter")
	if err != nil {
		s.runErr = err
		return err
	}
	counter.Add(ctx, 1)

	TextMapPropagator(ctx).Inject(ctx, propagation.MapCarrier{})
	return nil
}

// isolateOtelEnv clears the env vars that select OTel behavior so the tests
// do not depend on the caller's environment. An empty OTEL_*_EXPORTER makes
// autoexport use the library's no-op fallback, which logs the warning.
func isolateOtelEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"NS_SVC_OTEL_DISABLED",
		"NS_SVC_LOG_JSON",
		"NS_SVC_LOG_DEBUG",
		"NS_SVC_LOG_LEVEL",
		"OTEL_SDK_DISABLED",
		"OTEL_TRACES_EXPORTER",
		"OTEL_METRICS_EXPORTER",
	} {
		t.Setenv(k, "")
	}
}

// assertNoopOtel checks that every accessor on ctx returns a no-op
// implementation.
func assertNoopOtel(t *testing.T, ctx context.Context) {
	t.Helper()

	if ctx == nil {
		t.Fatal("Run was not called")
	}
	if got := TracerProvider(ctx); !isType[traceNoop.TracerProvider](got) {
		t.Errorf("TracerProvider = %T, want traceNoop.TracerProvider", got)
	}
	if got := Tracer(ctx); !isType[traceNoop.Tracer](got) {
		t.Errorf("Tracer = %T, want traceNoop.Tracer", got)
	}
	if got := MeterProvider(ctx); !isType[metricNoop.MeterProvider](got) {
		t.Errorf("MeterProvider = %T, want metricNoop.MeterProvider", got)
	}
	if got := Meter(ctx); !isType[metricNoop.Meter](got) {
		t.Errorf("Meter = %T, want metricNoop.Meter", got)
	}

	carrier := propagation.MapCarrier{}
	TextMapPropagator(ctx).Inject(ctx, carrier)
	if len(carrier) != 0 {
		t.Errorf("TextMapPropagator.Inject wrote %d fields, want 0 (empty composite)", len(carrier))
	}
}

func isType[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func TestRunC_DisableOtel_UsesNoopAndLogsNoWarning(t *testing.T) {
	isolateOtelEnv(t)

	svc := &otelProbeService{name: "svc"}
	var err error
	out := captureStdout(t, func() {
		err = RunC(svc, context.Background(),
			WithDisableOtel(true),
			WithLogJson(false),
			WithLogDebug(true),
		)
	})
	if err != nil {
		t.Fatalf("RunC: %v", err)
	}
	if svc.runErr != nil {
		t.Fatalf("creating an instrument failed: %v", svc.runErr)
	}

	assertNoopOtel(t, svc.runCtx)

	if strings.Contains(out, "no-op OTEL") || strings.Contains(out, "level=WARN") {
		t.Errorf("output contains an OTEL warning with DisableOtel set:\n%s", out)
	}
}

func TestRunC_DisableOtel_FromPrefixedEnv(t *testing.T) {
	isolateOtelEnv(t)
	t.Setenv("NS_SVC_OTEL_DISABLED", "true")

	svc := &otelProbeService{name: "svc"}
	var err error
	out := captureStdout(t, func() {
		err = RunC(svc, context.Background(), WithLogJson(false))
	})
	if err != nil {
		t.Fatalf("RunC: %v", err)
	}

	assertNoopOtel(t, svc.runCtx)

	if strings.Contains(out, "no-op OTEL") {
		t.Errorf("output contains an OTEL warning with NS_SVC_OTEL_DISABLED=true:\n%s", out)
	}
}

func TestRunC_OtelSdkDisabledEnv(t *testing.T) {
	isolateOtelEnv(t)
	t.Setenv("OTEL_SDK_DISABLED", "TRUE")

	svc := &otelProbeService{name: "svc"}
	var err error
	out := captureStdout(t, func() {
		err = RunC(svc, context.Background(), WithLogJson(false))
	})
	if err != nil {
		t.Fatalf("RunC: %v", err)
	}

	assertNoopOtel(t, svc.runCtx)

	if strings.Contains(out, "no-op OTEL") {
		t.Errorf("output contains an OTEL warning with OTEL_SDK_DISABLED=TRUE:\n%s", out)
	}
}

func TestRunC_OtelEnabledByDefault(t *testing.T) {
	// Control for the tests above: without the option the SDK providers are
	// installed and, with no exporter configured, the warning is logged.
	isolateOtelEnv(t)

	svc := &otelProbeService{name: "svc"}
	var err error
	out := captureStdout(t, func() {
		err = RunC(svc, context.Background(), WithLogJson(false))
	})
	if err != nil {
		t.Fatalf("RunC: %v", err)
	}
	if svc.runErr != nil {
		t.Fatalf("creating an instrument failed: %v", svc.runErr)
	}

	if got := TracerProvider(svc.runCtx); !isType[*traceSdk.TracerProvider](got) {
		t.Errorf("TracerProvider = %T, want *traceSdk.TracerProvider", got)
	}
	if !strings.Contains(out, "no-op OTEL") {
		t.Errorf("output lacks the no-op OTEL exporter warning:\n%s", out)
	}
}

func TestRunGroupC_DisableOtelAppliesToEveryService(t *testing.T) {
	isolateOtelEnv(t)
	t.Setenv("NS_A_OTEL_DISABLED", "")
	t.Setenv("NS_B_OTEL_DISABLED", "")

	a := &otelProbeService{name: "a"}
	b := &otelProbeService{name: "b"}
	var err error
	out := captureStdout(t, func() {
		err = RunGroupC([]Service{a, b}, context.Background(),
			WithDisableOtel(true),
			WithLogJson(false),
		)
	})
	if err != nil {
		t.Fatalf("RunGroupC: %v", err)
	}

	assertNoopOtel(t, a.runCtx)
	assertNoopOtel(t, b.runCtx)

	if strings.Contains(out, "no-op OTEL") {
		t.Errorf("output contains an OTEL warning with DisableOtel set:\n%s", out)
	}
}

func TestOtelDisabled(t *testing.T) {
	cases := []struct {
		name   string
		opt    bool
		envVal string
		want   bool
	}{
		{"default", false, "", false},
		{"option", true, "", true},
		{"env true", false, "true", true},
		{"env mixed case", false, "True", true},
		{"env false", false, "false", false},
		{"env other value", false, "1", false},
		{"option wins over env false", true, "false", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(otelSdkDisabledEnvKey, tc.envVal)
			if got := otelDisabled(Options{DisableOtel: tc.opt}); got != tc.want {
				t.Errorf("otelDisabled(DisableOtel=%v, %s=%q) = %v, want %v",
					tc.opt, otelSdkDisabledEnvKey, tc.envVal, got, tc.want)
			}
		})
	}
}
