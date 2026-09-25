package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/database"
	"github.com/fmolinar/arium/backend/internal/news"
	"github.com/fmolinar/arium/backend/internal/server"
	"github.com/fmolinar/arium/backend/internal/user"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	db, err := database.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := db.Disconnect(context.Background()); err != nil {
			log.Printf("disconnect MongoDB: %v", err)
		}
	}()

	userRepo := user.NewRepository(db.Database(cfg.MongoDatabase))
	if err := userRepo.EnsureIndexes(ctx); err != nil {
		log.Fatal(err)
	}

	userHandler := user.NewHandler(user.NewService(userRepo, cfg))

	newsRepo := news.NewRepository(db.Database(cfg.MongoDatabase))
	if err := newsRepo.EnsureIndexes(ctx); err != nil {
		log.Fatal(err)
	}

	newsHandler := news.NewHandler(news.NewService(newsRepo))

	app := server.New(cfg, db, userHandler, newsHandler)

	go func() {
		log.Printf("API listening on %s", cfg.Address)

		if err := app.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	stop()

	log.Print("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
