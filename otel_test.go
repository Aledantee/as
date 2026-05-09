package as_test

import (
	"context"
	"testing"

	"go.aledante.io/as"
	metricNoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

func TestTracerProvider_DefaultsToNoopWhenUnset(t *testing.T) {
	t.Parallel()

	got := as.TracerProvider(context.Background())
	if _, ok := got.(traceNoop.TracerProvider); !ok {
		t.Errorf("TracerProvider(bg) type = %T, want traceNoop.TracerProvider", got)
	}
}

func TestTracer_FallsBackToProviderDerivedTracer(t *testing.T) {
	t.Parallel()

	// With nothing in context, Tracer must not panic and must return a
	// non-nil tracer derived from the (noop) provider.
	tr := as.Tracer(context.Background())
	if tr == nil {
		t.Fatal("Tracer(bg) returned nil")
	}

	// Using the tracer should be a no-op and must not panic.
	_, span := tr.Start(context.Background(), "test-span")
	span.End()
}

func TestMeterProvider_DefaultsToNoopWhenUnset(t *testing.T) {
	t.Parallel()

	got := as.MeterProvider(context.Background())
	if _, ok := got.(metricNoop.MeterProvider); !ok {
		t.Errorf("MeterProvider(bg) type = %T, want metricNoop.MeterProvider", got)
	}
}

func TestMeter_FallsBackToProviderDerivedMeter(t *testing.T) {
	t.Parallel()

	m := as.Meter(context.Background())
	if m == nil {
		t.Fatal("Meter(bg) returned nil")
	}

	// Creating an instrument on the noop meter must not panic.
	if _, err := m.Int64Counter("test.counter"); err != nil {
		t.Errorf("noop Meter.Int64Counter returned error: %v", err)
	}
}

func TestTextMapPropagator_DefaultsToCompositeWhenUnset(t *testing.T) {
	t.Parallel()

	p := as.TextMapPropagator(context.Background())
	if p == nil {
		t.Fatal("TextMapPropagator(bg) returned nil")
	}

	// A fresh composite propagator with no sub-propagators has an empty Fields
	// slice; we only check that Inject/Extract do not panic and the carrier
	// remains unchanged.
	carrier := propagation.MapCarrier{}
	p.Inject(context.Background(), carrier)
	if len(carrier) != 0 {
		t.Errorf("default propagator Inject wrote %d fields, want 0", len(carrier))
	}
	if got := p.Extract(context.Background(), carrier); got == nil {
		t.Error("default propagator Extract returned nil context")
	}
}
