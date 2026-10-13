package user

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/middleware"
)

func Routes(h *Handler, cfg config.Config) chi.Router {
	router := chi.NewRouter()

	// Responses carry tokens and profiles, so nothing may cache them. Bodies
	// are a few short fields; cap them so a huge one can't tie up the decoder.
	router.Use(chimiddleware.NoCache)
	router.Use(chimiddleware.RequestSize(maxBodyBytes))

	// Both run bcrypt, so limiting them per client slows password guessing
	// and keeps a flood from pinning the CPU.
	router.Group(func(router chi.Router) {
		router.Use(middleware.RateLimit(10, 10))

		router.Post("/register", h.Register)
		router.Post("/login", h.Login)
	})

	router.Group(func(router chi.Router) {
		router.Use(middleware.Auth(cfg))

		router.Get("/me", h.Me)
		router.Patch("/me", h.UpdateMe)
	})

	return router
}
