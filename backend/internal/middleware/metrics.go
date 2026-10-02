package middleware

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// durationBuckets are the OpenTelemetry semantic-convention boundaries for
// http.server.request.duration, in seconds.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// knownMethods bounds the http.request.method label; anything else is
// recorded as "_OTHER", as the semantic conventions recommend.
var knownMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodConnect: true,
	http.MethodOptions: true, http.MethodTrace: true,
}

// Metrics records RED metrics for every request, following the OpenTelemetry
// HTTP semantic conventions: the http.server.request.duration histogram
// (whose count gives the rate and, by status code, the errors) labelled with
// method, chi route and status, and http.server.active_requests. Mount it
// outside Recoverer so panics are counted as 500s.
func Metrics(meter metric.Meter) func(http.Handler) http.Handler {
	duration, err := meter.Float64Histogram("http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	if err != nil {
		otel.Handle(err)
	}

	active, err := meter.Int64UpDownCounter("http.server.active_requests",
		metric.WithDescription("Number of HTTP server requests in flight."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		otel.Handle(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method := r.Method
			if !knownMethods[method] {
				method = "_OTHER"
			}
			methodAttr := attribute.String("http.request.method", method)

			ctx := r.Context()
			active.Add(ctx, 1, metric.WithAttributes(methodAttr))
			defer active.Add(ctx, -1, metric.WithAttributes(methodAttr))

			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
				methodAttr,
				attribute.String("http.route", routePattern(r)),
				attribute.Int("http.response.status_code", statusCode(ww)),
			))
		})
	}
}
