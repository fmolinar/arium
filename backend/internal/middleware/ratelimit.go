package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/fmolinar/arium/backend/pkg/response"
)

// idleLimiterTTL is how long a client's limiter is kept after its last
// request. Past that it would be full again anyway, so dropping it is free.
const idleLimiterTTL = 10 * time.Minute

type rateClient struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit allows each client perMinute requests a minute on average, with
// bursts of up to burst, and answers the rest with 429. It's meant for the
// unauthenticated, bcrypt-heavy routes (login, register), where it slows
// password guessing and keeps a flood of requests from pinning the CPU.
// Clients are told apart by clientIP, so mount it after Logging. Limits are
// per process, which is fine for a single API instance.
func RateLimit(perMinute, burst int) func(http.Handler) http.Handler {
	every := time.Minute / time.Duration(perMinute)
	retryAfter := strconv.Itoa(int(math.Ceil(every.Seconds())))

	var (
		mu        sync.Mutex
		clients   = map[string]*rateClient{}
		lastPrune = time.Now()
	)

	allow := func(key string) bool {
		mu.Lock()
		defer mu.Unlock()

		now := time.Now()
		if now.Sub(lastPrune) > time.Minute {
			for k, c := range clients {
				if now.Sub(c.lastSeen) > idleLimiterTTL {
					delete(clients, k)
				}
			}
			lastPrune = now
		}

		c, ok := clients[key]
		if !ok {
			c = &rateClient{limiter: rate.NewLimiter(rate.Every(every), burst)}
			clients[key] = c
		}
		c.lastSeen = now

		return c.limiter.AllowN(now, 1)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allow(clientIP(r)) {
				w.Header().Set("Retry-After", retryAfter)
				response.Error(w, http.StatusTooManyRequests, "too many requests, try again later")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
