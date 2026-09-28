// Package logging configura el logger estructurado del backend.
//
// Motivo concreto: el analizador de pedidos falla en background. Con log.Printf
// no hay forma de filtrar "fallo de Ollama" de "fallo de Postgres" ni de
// encontrar los 5 mensajes que fallaron de los 10.000. slog en JSON lo resuelve
// y, de paso, `docker compose logs backend | jq` empieza a servir.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup devuelve el logger raiz. El formato es JSON salvo que LOG_FORMAT=human,
// para poder leerlo crudo cuando se debuggea en una terminal.
func Setup() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "human") {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}

	log := slog.New(h)
	slog.SetDefault(log)
	return log
}
