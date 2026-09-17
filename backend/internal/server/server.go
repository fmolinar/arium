package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/middleware"
)

type Server struct {
	httpServer *http.Server
	db         *mongo.Client
}

func New(cfg config.Config, db *mongo.Client) *Server {
	server := &Server{
		db: db,
	}

	server.httpServer = &http.Server{
		Addr:              cfg.Address,
		Handler:           server.routes(cfg),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return server
}

func (s *Server) routes(cfg config.Config) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.Logging)
	router.Use(chimiddleware.Recoverer)
	router.Use(chimiddleware.Timeout(30 * time.Second))

	router.Use(middleware.CORS(cfg))

	router.Get("/health", s.health)

	router.Route("/api/v1", func(router chi.Router) {
		// Mount feature routes here:
		// router.Mount("/users", user.Routes(userHandler))
	})

	return router
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx, nil); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  "database unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
