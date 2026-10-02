package config

import (
	"log/slog"
	"os"
)

type Config struct {
	Address       string
	FrontendURL   string
	MongoURI      string
	MongoDatabase string
	JWTSecret     string

	// MetricsAddress is where Prometheus metrics are served at /metrics,
	// separately from the API. Empty disables the listener.
	MetricsAddress string
}

func Load() Config {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}

	return Config{
		Address:       getEnv("ADDRESS", ":8080"),
		FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:3000"),
		MongoURI:      getEnv("MONGO_URI", "mongodb://localhost:27017/arium"),
		MongoDatabase: getEnv("MONGO_DB", "arium"),
		JWTSecret:     jwtSecret,

		MetricsAddress: os.Getenv("METRICS_ADDRESS"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
