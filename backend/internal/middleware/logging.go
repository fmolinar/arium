package middleware

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func Logging(next http.Handler) http.Handler {
	return chimiddleware.RequestID(
		chimiddleware.RealIP(
			chimiddleware.Logger(next),
		),
	)
}
