// Package telemetry sets up OpenTelemetry metrics exported in the Prometheus
// format, served on their own listener for Prometheus to scrape.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Metrics owns a meter provider whose instruments are exposed, together with
// Go runtime and process metrics, by Handler.
type Metrics struct {
	provider *sdkmetric.MeterProvider
	handler  http.Handler
}

// NewMetrics creates the meter provider for service and registers it as the
// global one.
func NewMetrics(service string) (*Metrics, error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry))
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(resource.NewSchemaless(attribute.String("service.name", service))),
	)
	otel.SetMeterProvider(provider)

	return &Metrics{
		provider: provider,
		handler:  promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}, nil
}

// Meter returns a named meter from the provider.
func (m *Metrics) Meter(name string) metric.Meter {
	return m.provider.Meter(name)
}

// Handler serves the metrics in the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return m.handler
}

// Serve exposes Handler at /metrics on addr until ctx is cancelled. It runs in
// the background and logs, rather than returns, a listener failure, so a
// metrics problem never takes the service down.
func (m *Metrics) Serve(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", m.handler)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("metrics listening", "address", addr)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics server", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = srv.Shutdown(shutdownCtx)
	}()
}

// Shutdown releases the meter provider.
func (m *Metrics) Shutdown(ctx context.Context) error {
	return m.provider.Shutdown(ctx)
}
