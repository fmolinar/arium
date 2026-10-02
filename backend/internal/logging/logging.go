// Package logging sets up the JSON slog logger shared by the API and the
// collector. Records logged with a context carry that context's chi request
// ID and OpenTelemetry trace/span IDs, so log lines can be joined to requests
// and traces.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

// New returns a JSON logger writing to w at the given level, with every record
// tagged with service.
func New(w io.Writer, service string, level slog.Leveler) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})

	return slog.New(contextHandler{handler}).With("service", service)
}

// Setup builds a logger for service writing to stdout at the level named by
// LOG_LEVEL (debug, info, warn or error; default info) and makes it the
// default, which also routes the standard log package through it.
func Setup(service string) *slog.Logger {
	level, err := ParseLevel(os.Getenv("LOG_LEVEL"))

	logger := New(os.Stdout, service, level)
	slog.SetDefault(logger)

	if err != nil {
		logger.Warn("invalid LOG_LEVEL, using info", "error", err)
	}

	return logger
}

// ParseLevel parses a level name case-insensitively. An empty name is info.
func ParseLevel(name string) (slog.Level, error) {
	var level slog.Level
	if strings.TrimSpace(name) == "" {
		return level, nil
	}

	err := level.UnmarshalText([]byte(name))

	return level, err
}

// Fatal logs msg at error level and exits with status 1.
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

// contextHandler adds the request and trace IDs found in a record's context.
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := chimiddleware.GetReqID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}

	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
