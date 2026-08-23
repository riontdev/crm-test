package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL         string
	ZernioAPIKey        string
	ZernioWebhookSecret string
	OpenRouterAPIKey    string
	Port                string
}

func Load() *Config {
	// Load .env file if it exists (ignore error if not found)
	godotenv.Load(".env")
	godotenv.Load("../.env")
	godotenv.Load("../../.env")

	return &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		ZernioAPIKey:        os.Getenv("ZERNIO_API_KEY"),
		ZernioWebhookSecret: os.Getenv("ZERNIO_WEBHOOK_SECRET"),
		OpenRouterAPIKey:    os.Getenv("OPENROUTER_API_KEY"),
		Port:                getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
