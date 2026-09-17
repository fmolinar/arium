package config

import (
	"log"
	"os"
)

type Config struct {
	Address       string
	FrontendURL   string
	MongoURI      string
	MongoDatabase string
	JWTSecret     string
}

func Load() Config {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	return Config{
		Address:       getEnv("ADDRESS", ":8080"),
		FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:3000"),
		MongoURI:      getEnv("MONGO_URI", "mongodb://localhost:27017/arium"),
		MongoDatabase: getEnv("MONGO_DB", "arium"),
		JWTSecret:     jwtSecret,
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
