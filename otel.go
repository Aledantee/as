package as

import (
	"context"
	"errors"

	"go.aledante.io/ae"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/metric"
	metricNoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

const (
	tracerName = "flowseer/common/svc"
	meterName  = "flowseer/common/svc"
)

// tracerProviderKey is the key type for storing the TracerProvider in context.
type tracerProviderKey struct{}

// withTracerProvider returns a new context with a trace.TracerProvider set.
// If provider is nil, a no-op TracerProvider will be used.
func withTracerProvider(ctx context.Context, provider trace.TracerProvider) context.Context {
	if provider == nil {
		provider = traceNoop.NewTracerProvider()
	}
	return context.WithValue(ctx, tracerProviderKey{}, provider)
}

// TracerProvider extracts the trace.TracerProvider from the context, or returns a no-op provider if not set.
func TracerProvider(ctx context.Context) trace.TracerProvider {
	v, ok := ctx.Value(tracerProviderKey{}).(trace.TracerProvider)
	if !ok {
		return traceNoop.NewTracerProvider()
	}
	return v
}

// tracerKey is the key type for storing the Tracer in context.
type tracerKey struct{}

// withTracer returns a new context with a trace.Tracer set.
// If tracer is nil, a default tracer from the context's TracerProvider is used.
func withTracer(ctx context.Context, tracer trace.Tracer) context.Context {
	if tracer == nil {
		tracer = TracerProvider(ctx).Tracer(tracerName)
	}
	return context.WithValue(ctx, tracerKey{}, tracer)
}

// Tracer extracts the trace.Tracer from the context, or creates one from the context's TracerProvider if not found.
func Tracer(ctx context.Context) trace.Tracer {
	v, ok := ctx.Value(tracerKey{}).(trace.Tracer)
	if !ok {
		return TracerProvider(ctx).Tracer(tracerName)
	}
	return v
}

// meterProviderKey is the key type for storing the MeterProvider in context.
type meterProviderKey struct{}

// withMeterProvider returns a new context with a metric.MeterProvider set.
// If provider is nil, a no-op MeterProvider is used.
func withMeterProvider(ctx context.Context, provider metric.MeterProvider) context.Context {
	if provider == nil {
		provider = metricNoop.NewMeterProvider()
	}
	return context.WithValue(ctx, meterProviderKey{}, provider)
}

// MeterProvider extracts the metric.MeterProvider from the context, or returns a no-op provider if not found.
func MeterProvider(ctx context.Context) metric.MeterProvider {
	v, ok := ctx.Value(meterProviderKey{}).(metric.MeterProvider)
	if !ok {
		return metricNoop.NewMeterProvider()
	}
	return v
}

// meterKey is the key type for storing the Meter in context.
type meterKey struct{}

// withMeter returns a new context with a metric.Meter set.
// If meter is nil, a default meter from the context's MeterProvider is used.
func withMeter(ctx context.Context, meter metric.Meter) context.Context {
	if meter == nil {
		meter = MeterProvider(ctx).Meter(meterName)
	}
	return context.WithValue(ctx, meterKey{}, meter)
}

// Meter extracts the metric.Meter from the context, or creates one from the context's MeterProvider if not found.
func Meter(ctx context.Context) metric.Meter {
	v, ok := ctx.Value(meterKey{}).(metric.Meter)
	if !ok {
		return MeterProvider(ctx).Meter(meterName)
	}
	return v
}

type textMapPropagatorKey struct{}

func withTextMapPropagator(ctx context.Context, propagator propagation.TextMapPropagator) context.Context {
	if propagator == nil {
		propagator = propagation.NewCompositeTextMapPropagator()
	}

	return context.WithValue(ctx, textMapPropagatorKey{}, propagator)
}

func TextMapPropagator(ctx context.Context) propagation.TextMapPropagator {
	v, ok := ctx.Value(textMapPropagatorKey{}).(propagation.TextMapPropagator)
	if !ok {
		return propagation.NewCompositeTextMapPropagator()
	}

	return v
}

// initOtel initializes OpenTelemetry providers or resources for the given context.
// This currently panics as it is not implemented.
func initOtel(ctx context.Context) (context.Context, func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	shutdown := func(shutdownCtx context.Context) error {
		var errs []error

		for _, shutdownFunc := range shutdownFuncs {
			err := shutdownFunc(shutdownCtx)
			if err != nil {
				errs = append(errs, err)
			}
		}

		return ae.WrapMany("OTEL shutdown failed", errs...)
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceNameKey.String(Name(ctx)),
		semconv.ServiceVersionKey.String(Version(ctx)),
		semconv.ServiceNamespaceKey.String(Namespace(ctx)),
	))
	if err != nil {
		if errors.Is(err, resource.ErrSchemaURLConflict) {
			// As defined by resource.Merge, the Merge returns a resource with an empty schema URL when the error
			// is resource.ErrSchemaURLConflict.
			// Additionally, Merge will use the first non-empty schema URL when merging resources, so this effectively
			// sets the schema URL to the one which would is used by resource.Default.
			res, _ = resource.Merge(resource.Default(), res)

			Logger(ctx).Warn(
				"OTEL resource schema URL conflict. Overwriting to the default one",
				"schema_url", res.SchemaURL(),
			)
		} else {
			return ctx, noopShutdown, ae.Wrap("failed to create OTEL resource", err)
		}
	}

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
	)
	ctx = withTextMapPropagator(ctx, propagator)

	spanExporter, err := autoexport.NewSpanExporter(ctx, autoexport.WithFallbackSpanExporter(noopSpanExporterFunc))
	if err != nil {
		return ctx, noopShutdown, ae.Wrap("failed to create OTEL span exporter", err)
	}

	shutdownFuncs = append(shutdownFuncs, spanExporter.Shutdown)
	if isNoopSpanExporter(spanExporter) {
		Logger(ctx).Warn("using a no-op OTEL span exporter. Set OTEL_TRACES_EXPORTER and related env vars as required")
	}

	tracerProvider := traceSdk.NewTracerProvider(
		traceSdk.WithBatcher(spanExporter),
		traceSdk.WithResource(res),
	)
	ctx = withTracerProvider(ctx, tracerProvider)
	ctx = withTracer(ctx, tracerProvider.Tracer(tracerName))

	metricReader, err := autoexport.NewMetricReader(ctx,
		autoexport.WithFallbackMetricReader(noopMetricReaderFunc),
	)
	if err != nil {
		return ctx, noopShutdown, ae.Wrap("failed to create OTEL metric reader", err)
	}

	shutdownFuncs = append(shutdownFuncs, metricReader.Shutdown)

	if isNoopMetricReader(metricReader) {
		Logger(ctx).Warn("using a no-op OTEL metric exporter. Set OTEL_METRICS_EXPORTER and related env vars as required")
	}

	meterProvider := metricSdk.NewMeterProvider(
		metricSdk.WithReader(metricReader),
		metricSdk.WithResource(res),
	)
	ctx = withMeterProvider(ctx, meterProvider)
	ctx = withMeter(ctx, meterProvider.Meter(meterName))

	// We will be missing go.schedule.duration, but that is only exposed by runtime.NewProducer which we cannot add
	// to the autoexport reader... but it does not seem to be of much use
	_ = runtime.Start(
		runtime.WithMeterProvider(meterProvider),
	)

	return ctx, shutdown, nil
}

func noopShutdown(ctx context.Context) error {
	return nil
}

type noopSpanExporter struct{}

func (n noopSpanExporter) ExportSpans(ctx context.Context, spans []traceSdk.ReadOnlySpan) error {
	return nil
}

func (n noopSpanExporter) Shutdown(ctx context.Context) error {
	return nil
}

func noopSpanExporterFunc(ctx context.Context) (traceSdk.SpanExporter, error) {
	return noopSpanExporter{}, nil
}

func isNoopSpanExporter(se traceSdk.SpanExporter) bool {
	_, ok := se.(noopSpanExporter)
	return ok
}

type noopMetricReader struct {
	*metricSdk.ManualReader
}

func noopMetricReaderFunc(ctx context.Context) (metricSdk.Reader, error) {
	return noopMetricReader{metricSdk.NewManualReader()}, nil
}

func isNoopMetricReader(se metricSdk.Reader) bool {
	_, ok := se.(noopMetricReader)
	return ok
}
