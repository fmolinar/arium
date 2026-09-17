//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/database"
	"github.com/fmolinar/arium/backend/internal/server"
	"github.com/fmolinar/arium/backend/internal/user"
)

var testServer *httptest.Server

// TestMain spins up the real HTTP server against a MongoDB instance reachable
// at TEST_MONGO_URI (default: mongodb://localhost:27017), using a throwaway
// database that is dropped once the suite finishes.
func TestMain(m *testing.M) {
	mongoURI := os.Getenv("TEST_MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	cfg := config.Config{
		Address:       ":0",
		FrontendURL:   "http://localhost:3000",
		MongoURI:      mongoURI,
		MongoDatabase: fmt.Sprintf("arium_integration_%d", time.Now().UnixNano()),
		JWTSecret:     "integration-test-secret",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := database.Connect(ctx, cfg.MongoURI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests require a reachable MongoDB (set TEST_MONGO_URI): %v\n", err)
		os.Exit(1)
	}

	userRepo := user.NewRepository(client.Database(cfg.MongoDatabase))
	if err := userRepo.EnsureIndexes(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ensure indexes: %v\n", err)
		os.Exit(1)
	}

	userHandler := user.NewHandler(user.NewService(userRepo, cfg))
	app := server.New(cfg, client, userHandler)
	testServer = httptest.NewServer(app.Handler())

	code := m.Run()

	testServer.Close()

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = client.Database(cfg.MongoDatabase).Drop(cleanupCtx)
	_ = client.Disconnect(cleanupCtx)
	cleanupCancel()

	os.Exit(code)
}
