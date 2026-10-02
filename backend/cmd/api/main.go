package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/database"
	"github.com/fmolinar/arium/backend/internal/logging"
	"github.com/fmolinar/arium/backend/internal/news"
	"github.com/fmolinar/arium/backend/internal/server"
	"github.com/fmolinar/arium/backend/internal/telemetry"
	"github.com/fmolinar/arium/backend/internal/user"
)

func main() {
	logging.Setup("arium-api")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	metrics, err := telemetry.NewMetrics("arium-api")
	if err != nil {
		logging.Fatal("set up metrics", "error", err)
	}

	defer func() {
		if err := metrics.Shutdown(context.Background()); err != nil {
			slog.Error("shut down metrics", "error", err)
		}
	}()

	if cfg.MetricsAddress != "" {
		metrics.Serve(ctx, cfg.MetricsAddress)
	}

	db, err := database.Connect(ctx, cfg.MongoURI)
	if err != nil {
		logging.Fatal("connect to MongoDB", "error", err)
	}

	defer func() {
		if err := db.Disconnect(context.Background()); err != nil {
			slog.Error("disconnect MongoDB", "error", err)
		}
	}()

	userRepo := user.NewRepository(db.Database(cfg.MongoDatabase))
	if err := userRepo.EnsureIndexes(ctx); err != nil {
		logging.Fatal("ensure user indexes", "error", err)
	}

	userHandler := user.NewHandler(user.NewService(userRepo, cfg))

	newsRepo := news.NewRepository(db.Database(cfg.MongoDatabase))
	if err := newsRepo.EnsureIndexes(ctx); err != nil {
		logging.Fatal("ensure news indexes", "error", err)
	}

	newsHandler := news.NewHandler(news.NewService(newsRepo))

	app := server.New(cfg, db, metrics.Meter("github.com/fmolinar/arium/backend/internal/server"), userHandler, newsHandler)

	go func() {
		slog.Info("API listening", "address", cfg.Address)

		if err := app.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Fatal("serve API", "error", err)
		}
	}()

	<-ctx.Done()
	stop()

	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "error", err)
	}
}
