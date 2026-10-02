package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/fmolinar/arium/backend/internal/logging"
)

// testRouter mirrors server.routes: global middleware plus a mounted
// sub-router, so route patterns come out as they do in the API.
func testRouter(provider *sdkmetric.MeterProvider) http.Handler {
	router := chi.NewRouter()
	router.Use(Logging)
	router.Use(Metrics(provider.Meter("test")))
	router.Use(Recoverer)

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {})
	router.Route("/api/v1", func(r chi.Router) {
		news := chi.NewRouter()
		news.Get("/", func(w http.ResponseWriter, r *http.Request) {})
		news.Get("/{id}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
		news.Get("/boom", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
		r.Mount("/news", news)
	})

	return router
}

func TestMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	handler := testRouter(provider)

	setLogOutput(t, &bytes.Buffer{})

	for _, path := range []string{"/health", "/api/v1/news", "/api/v1/news/", "/api/v1/news/abc", "/api/v1/news/def", "/api/v1/news/boom", "/nope"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}

	got := map[string]uint64{} // "route status" -> count
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "http.server.request.duration" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Histogram[float64]).DataPoints {
				route, _ := dp.Attributes.Value(attribute.Key("http.route"))
				status, _ := dp.Attributes.Value(attribute.Key("http.response.status_code"))
				method, _ := dp.Attributes.Value(attribute.Key("http.request.method"))
				if method.AsString() != http.MethodGet {
					t.Errorf("method = %q", method.AsString())
				}
				got[route.AsString()+" "+status.Emit()] += dp.Count
			}
		}
	}

	want := map[string]uint64{
		"/health 200":           1,
		"/api/v1/news 200":      2,
		"/api/v1/news/{id} 404": 2,
		"/api/v1/news/boom 500": 1,
		"unmatched 404":         1,
	}
	if len(got) != len(want) {
		t.Errorf("got series %v, want %v", got, want)
	}
	for key, n := range want {
		if got[key] != n {
			t.Errorf("%s: count %d, want %d (all: %v)", key, got[key], n, got)
		}
	}
}

func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	setLogOutput(t, &buf)

	handler := testRouter(sdkmetric.NewMeterProvider())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/news/abc", nil))

	requestID := rec.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Error("no X-Request-Id header")
	}

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("access log is not one JSON line: %v: %q", err, buf.String())
	}

	want := map[string]any{
		"msg":        "request",
		"level":      "INFO",
		"method":     "GET",
		"path":       "/api/v1/news/abc",
		"route":      "/api/v1/news/{id}",
		"status":     float64(404),
		"request_id": requestID,
	}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("%s = %v, want %v", key, line[key], value)
		}
	}
}

func TestRecovererLogs(t *testing.T) {
	var buf bytes.Buffer
	setLogOutput(t, &buf)

	rec := httptest.NewRecorder()
	testRouter(sdkmetric.NewMeterProvider()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/news/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want a panic line and an access line, got %q", buf.String())
	}

	var panicLine map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &panicLine); err != nil {
		t.Fatal(err)
	}
	if panicLine["level"] != "ERROR" || panicLine["panic"] != "boom" || panicLine["stack"] == "" || panicLine["request_id"] == nil {
		t.Errorf("panic line = %v", panicLine)
	}
}

// setLogOutput points the default slog logger at w for the test.
func setLogOutput(t *testing.T, w *bytes.Buffer) {
	t.Helper()

	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	slog.SetDefault(logging.New(w, "test", slog.LevelInfo))
}
