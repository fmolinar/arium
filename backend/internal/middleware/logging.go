package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// Logging assigns each request an ID (echoed in X-Request-Id), resolves the
// client IP (see realIP) and writes one JSON access log line per request
// through the default slog logger. Mount it first so every later log line in
// the request carries the request ID.
func Logging(next http.Handler) http.Handler {
	return chimiddleware.RequestID(
		realIP(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-Id", chimiddleware.GetReqID(r.Context()))

				start := time.Now()
				ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

				next.ServeHTTP(ww, r)

				status := statusCode(ww)
				level := slog.LevelInfo
				if status >= http.StatusInternalServerError {
					level = slog.LevelError
				}

				slog.LogAttrs(r.Context(), level, "request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("route", routePattern(r)),
					slog.Int("status", status),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
					slog.String("remote_ip", clientIP(r)),
					slog.String("user_agent", r.UserAgent()),
				)
			}),
		),
	)
}

// Recoverer turns a panic in a handler into a 500 and logs it with its stack
// trace as a single JSON record, unlike chi's Recoverer, which prints a
// multi-line stack to stderr.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rvr := recover()
			if rvr == nil {
				return
			}
			if rvr == http.ErrAbortHandler {
				// The client went away; net/http handles this panic itself.
				panic(rvr)
			}

			slog.ErrorContext(r.Context(), "panic serving request",
				"panic", rvr,
				"stack", string(debug.Stack()),
				"method", r.Method,
				"path", r.URL.Path,
			)

			if r.Header.Get("Connection") != "Upgrade" {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// realIP sets RemoteAddr to the CF-Connecting-IP header when there is one.
// Requests from the internet reach the API through Cloudflare, which always
// overwrites that header with the real client address. It deliberately
// ignores X-Forwarded-For, X-Real-IP and True-Client-IP (chi's RealIP trusts
// them): any client can set those, and would get a fresh rate limit and a
// forged address in the logs.
func realIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := net.ParseIP(r.Header.Get("CF-Connecting-IP")); ip != nil {
			r.RemoteAddr = ip.String()
		}

		next.ServeHTTP(w, r)
	})
}

// clientIP is r's client address without the port, after realIP.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}

// statusCode is the status the handler wrote, or 200 if it wrote nothing.
func statusCode(ww chimiddleware.WrapResponseWriter) int {
	if status := ww.Status(); status != 0 {
		return status
	}

	return http.StatusOK
}

// routePattern is the chi route that matched r, such as "/api/v1/news/{id}", or
// "unmatched" when none did. Read it after the handler ran, once routing is
// complete. Using the pattern rather than the path keeps metric labels and
// log fields low-cardinality.
func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}

	return "unmatched"
}
