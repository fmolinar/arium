package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Tracing starts a server span for every request, following the OpenTelemetry
// HTTP semantic conventions. Mount it first, outside Logging, so the access log
// line carries the trace and span IDs.
//
// The API is a public endpoint: a traceparent header from the client starts a
// new trace linked to the client's, rather than continuing it, so callers
// can't pick trace IDs or force sampling.
func Tracing(provider trace.TracerProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		named := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			// Routing is done now.
			route := routePattern(r)
			name := r.Method
			if !knownMethods[name] {
				name = "_OTHER"
			}
			if route != "unmatched" {
				name += " " + route
			}

			span := trace.SpanFromContext(r.Context())
			span.SetName(name)
			span.SetAttributes(
				attribute.String("http.route", route),
				// otelhttp reads the client address from X-Forwarded-For,
				// which clients can forge; use the one the logs trust.
				attribute.String("client.address", trustedClientIP(r)),
			)
		})

		return otelhttp.NewHandler(named, "http.server",
			otelhttp.WithTracerProvider(provider),
			// Metrics records the HTTP metrics, with route labels.
			otelhttp.WithMeterProvider(metricnoop.NewMeterProvider()),
			otelhttp.WithPropagators(propagation.TraceContext{}),
			otelhttp.WithPublicEndpointFn(func(*http.Request) bool { return true }),
		)
	}
}
