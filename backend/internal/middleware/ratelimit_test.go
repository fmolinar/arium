package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit(t *testing.T) {
	handler := Logging(RateLimit(10, 3)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))

	send := func(headers map[string]string) int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := range 3 {
		if code := send(nil); code != http.StatusOK {
			t.Fatalf("request %d within the burst: got %d", i+1, code)
		}
	}
	if code := send(nil); code != http.StatusTooManyRequests {
		t.Fatalf("request past the burst: got %d, want 429", code)
	}

	// Headers a client can set must not buy a fresh limit.
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP", "True-Client-IP"} {
		if code := send(map[string]string{h: "203.0.113.9"}); code != http.StatusTooManyRequests {
			t.Fatalf("spoofed %s: got %d, want 429", h, code)
		}
	}

	// Cloudflare's CF-Connecting-IP identifies a different client.
	if code := send(map[string]string{"CF-Connecting-IP": "198.51.100.7"}); code != http.StatusOK {
		t.Fatalf("other client behind Cloudflare: got %d, want 200", code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}
