package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/middleware"
	"github.com/fmolinar/arium/backend/internal/user"
	"github.com/fmolinar/arium/backend/pkg/response"
)

type Server struct {
	httpServer *http.Server
	db         *mongo.Client
}

func New(cfg config.Config, db *mongo.Client, userHandler *user.Handler) *Server {
	server := &Server{
		db: db,
	}

	server.httpServer = &http.Server{
		Addr:              cfg.Address,
		Handler:           server.routes(cfg, userHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return server
}

func (s *Server) routes(cfg config.Config, userHandler *user.Handler) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.Logging)
	router.Use(chimiddleware.Recoverer)
	router.Use(chimiddleware.Timeout(30 * time.Second))

	router.Use(middleware.CORS(cfg))

	router.Get("/health", s.health)

	router.Route("/api/v1", func(router chi.Router) {
		router.Mount("/users", user.Routes(userHandler, cfg))
	})

	return router
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx, nil); err != nil {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  "database unavailable",
		})
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
