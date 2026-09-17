package user

import (
	"github.com/go-chi/chi/v5"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/middleware"
)

func Routes(h *Handler, cfg config.Config) chi.Router {
	router := chi.NewRouter()

	router.Post("/register", h.Register)
	router.Post("/login", h.Login)

	router.Group(func(router chi.Router) {
		router.Use(middleware.Auth(cfg))

		router.Get("/me", h.Me)
		router.Patch("/me", h.UpdateMe)
	})

	return router
}
