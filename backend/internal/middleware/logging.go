package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func Logging(next http.Handler) http.Handler {
	return middleware.RequestID(
		middleware.RealIP(
			middleware.Logger(next),
		),
	)
}
