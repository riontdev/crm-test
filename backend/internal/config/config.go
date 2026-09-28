package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL         string
	ZernioAPIKey        string
	ZernioWebhookSecret string
	OpenRouterAPIKey    string
	Port                string

	// Analisis de pedidos con IA local (Fase 19).
	// Ollama corre en otro contenedor de compose: el backend no lo expone a
	// internet, solo le habla por la red interna de docker.
	OllamaBaseURL  string
	OllamaModel    string
	OllamaTimeout  time.Duration
	TextWorkers    int
	InsightEnabled bool

	// Transcripcion de notas de voz (Fase 20).
	ASRBaseURL      string
	WhisperModel    string
	ASRTimeout      time.Duration
	ASRDownloadTime time.Duration
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

		OllamaBaseURL:  getEnv("OLLAMA_BASE_URL", "http://ollama:11434"),
		OllamaModel:    getEnv("OLLAMA_MODEL", "granite3.3:2b"),
		OllamaTimeout:  getEnvDuration("OLLAMA_TIMEOUT", 90*time.Second),
		TextWorkers:    getEnvInt("INSIGHT_TEXT_WORKERS", 1),
		InsightEnabled: getEnvBool("INSIGHT_ENABLED", true),

		ASRBaseURL:      getEnv("ASR_BASE_URL", "http://whisper:9000"),
		WhisperModel:    getEnv("WHISPER_MODEL", "small"),
		ASRTimeout:      getEnvDuration("ASR_TIMEOUT", 120*time.Second),
		ASRDownloadTime: getEnvDuration("ASR_DOWNLOAD_TIMEOUT", 60*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
