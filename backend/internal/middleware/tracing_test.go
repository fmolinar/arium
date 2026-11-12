package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracing(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	router := chi.NewRouter()
	router.Use(Tracing(provider))
	router.Use(Logging)
	router.Route("/api/v1", func(r chi.Router) {
		news := chi.NewRouter()
		news.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {})
		r.Mount("/news", news)
	})

	var logs bytes.Buffer
	setLogOutput(t, &logs)

	const clientTrace = "4bf92f3577b34da6a3ce929d0e0e4736"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/news/abc", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	req.Header.Set("traceparent", "00-"+clientTrace+"-00f067aa0ba902b7-01")
	router.ServeHTTP(httptest.NewRecorder(), req)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nope", nil))

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}

	span := spans[0]
	if got, want := span.Name(), "GET /api/v1/news/{id}"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}

	attrs := map[string]string{}
	for _, kv := range span.Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	if got := attrs["http.route"]; got != "/api/v1/news/{id}" {
		t.Errorf("http.route = %q", got)
	}
	if got := attrs["client.address"]; got != "203.0.113.7" {
		t.Errorf("client.address = %q, want the CF-Connecting-IP address", got)
	}

	// A client's traceparent is linked, not continued.
	if span.Parent().IsValid() || span.SpanContext().TraceID().String() == clientTrace {
		t.Error("span continued the client's trace")
	}
	if links := span.Links(); len(links) != 1 || links[0].SpanContext.TraceID().String() != clientTrace {
		t.Errorf("links = %v, want one to the client's trace", links)
	}

	if got, want := spans[1].Name(), "GET"; got != want {
		t.Errorf("unmatched span name = %q, want %q", got, want)
	}

	// The access log line carries the request span's trace ID.
	var line struct {
		TraceID string `json:"trace_id"`
	}
	if err := json.Unmarshal(bytes.SplitN(logs.Bytes(), []byte("\n"), 2)[0], &line); err != nil {
		t.Fatal(err)
	}
	if got, want := line.TraceID, span.SpanContext().TraceID().String(); got != want {
		t.Errorf("access log trace_id = %q, want %q", got, want)
	}
}
