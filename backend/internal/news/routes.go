package news

import "github.com/go-chi/chi/v5"

func Routes(h *Handler) chi.Router {
	router := chi.NewRouter()

	router.Get("/", h.List)

	return router
}
