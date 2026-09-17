package config

import "os"

type Config struct {
	Address     string
	FrontendURL string
	MongoURI    string
}

func Load() Config {
	return Config{
		Address:     getEnv("ADDRESS", ":8080"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		MongoURI:    getEnv("MONGO_URI", "mongodb://localhost:27017/arium"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
