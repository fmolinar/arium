package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

func TestContextIDs(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "test", slog.LevelInfo).With("k", "v")

	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")

	ctx := context.WithValue(context.Background(), chimiddleware.RequestIDKey, "host/abc-000001")
	ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))

	logger.InfoContext(ctx, "hello")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v: %s", err, buf.String())
	}

	want := map[string]string{
		"msg":        "hello",
		"service":    "test",
		"k":          "v",
		"request_id": "host/abc-000001",
		"trace_id":   "4bf92f3577b34da6a3ce929d0e0e4736",
		"span_id":    "00f067aa0ba902b7",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %v, want %q", key, got[key], value)
		}
	}
}

func TestNoContextIDs(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, "test", slog.LevelInfo).Info("hello")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"request_id", "trace_id", "span_id"} {
		if _, ok := got[key]; ok {
			t.Errorf("unexpected %s in %s", key, buf.String())
		}
	}
}

func TestParseLevel(t *testing.T) {
	for name, want := range map[string]slog.Level{"": slog.LevelInfo, "debug": slog.LevelDebug, "WARN": slog.LevelWarn, "error": slog.LevelError} {
		got, err := ParseLevel(name)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", name, got, err, want)
		}
	}

	if _, err := ParseLevel("loud"); err == nil {
		t.Error("ParseLevel(loud) succeeded")
	}
}
