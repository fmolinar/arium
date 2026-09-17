package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/pkg/response"
)

type contextKey string

const (
	userIDKey   contextKey = "userID"
	userRoleKey contextKey = "userRole"
)

const tokenTTL = 24 * time.Hour

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// NewToken issues a signed JWT identifying userID, carrying role as a custom claim.
func NewToken(cfg config.Config, userID, role string) (string, error) {
	now := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	})

	return token.SignedString([]byte(cfg.JWTSecret))
}

// Auth validates the Bearer token on each request and attaches the subject
// and role to the request context. Requests without a valid token are rejected.
func Auth(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				response.Error(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}

			parsed := &claims{}

			token, err := jwt.ParseWithClaims(raw, parsed, func(*jwt.Token) (any, error) {
				return []byte(cfg.JWTSecret), nil
			}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
			if err != nil || !token.Valid {
				response.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, parsed.Subject)
			ctx = context.WithValue(ctx, userRoleKey, parsed.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

func UserRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(userRoleKey).(string)
	return role, ok
}
