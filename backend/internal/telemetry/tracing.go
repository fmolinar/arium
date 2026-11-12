package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Tracing owns the tracer provider. Spans are exported over OTLP/HTTP to the
// endpoint in the standard OTEL_EXPORTER_OTLP_ENDPOINT (or
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT) variable; the other OTEL_* exporter and
// sampler variables apply too. With neither endpoint set, tracing is off and
// Provider returns a no-op provider.
type Tracing struct {
	provider *sdktrace.TracerProvider // nil when tracing is off
}

// NewTracing creates the tracer provider for service and registers it, with
// the W3C trace context propagator, as the global one.
func NewTracing(ctx context.Context, service string) (*Tracing, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		slog.Info("tracing disabled: no OTLP endpoint set")
		return &Tracing{}, nil
	}

	// New doesn't connect: an unreachable collector costs dropped spans,
	// logged by the batcher, and never fails a request.
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", service))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	slog.Info("tracing enabled")

	return &Tracing{provider: provider}, nil
}

// Provider returns the tracer provider, a no-op one when tracing is off.
func (t *Tracing) Provider() trace.TracerProvider {
	if t.provider == nil {
		return noop.NewTracerProvider()
	}

	return t.provider
}

// Shutdown flushes buffered spans and releases the provider.
func (t *Tracing) Shutdown(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}

	return t.provider.Shutdown(ctx)
}
