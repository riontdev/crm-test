package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

var validAgentChannels = map[string]bool{
	"whatsapp":  true,
	"instagram": true,
	"facebook":  true,
}

type AgentsHandler struct {
	pool *pgxpool.Pool
}

func NewAgentsHandler(pool *pgxpool.Pool) *AgentsHandler {
	return &AgentsHandler{pool: pool}
}

type AgentConfigResponse struct {
	ID           uuid.UUID `json:"id"`
	Channel      string    `json:"channel"`
	Enabled      bool      `json:"enabled"`
	Model        string    `json:"model"`
	SystemPrompt *string   `json:"system_prompt,omitempty"`
	Temperature  float64   `json:"temperature"`
	MaxTokens    int       `json:"max_tokens"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

const agentConfigColumns = `id, channel, enabled, model, system_prompt, temperature, max_tokens, created_at, updated_at`

// scanAgentConfig scans one agent_configs row and maps it to the JSON response shape.
func scanAgentConfig(row pgx.Row) (*AgentConfigResponse, error) {
	var id uuid.UUID
	var channel, model string
	var enabled bool
	var systemPrompt *string
	var temperature float64
	var maxTokens int
	var createdAt, updatedAt time.Time

	if err := row.Scan(&id, &channel, &enabled, &model, &systemPrompt,
		&temperature, &maxTokens, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	return &AgentConfigResponse{
		ID:           id,
		Channel:      channel,
		Enabled:      enabled,
		Model:        model,
		SystemPrompt: systemPrompt,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
		CreatedAt:    createdAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    updatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// ListAgents returns all agent configs.
// GET /api/agents
func (h *AgentsHandler) ListAgents(c echo.Context) error {
	if h.pool == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	rows, err := h.pool.Query(c.Request().Context(),
		`SELECT `+agentConfigColumns+` FROM agent_configs ORDER BY channel`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al listar las configuraciones de agente"})
	}
	defer rows.Close()

	results := []AgentConfigResponse{}
	for rows.Next() {
		cfg, err := scanAgentConfig(rows)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer las configuraciones de agente"})
		}
		results = append(results, *cfg)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer las configuraciones de agente"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":  results,
		"count": len(results),
	})
}

// UpdateAgent modifies the agent config for a channel (enabled, model, system_prompt, temperature, max_tokens).
// PATCH /api/agents/:channel
func (h *AgentsHandler) UpdateAgent(c echo.Context) error {
	if h.pool == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	channel := c.Param("channel")
	if !validAgentChannels[channel] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "canal inválido: debe ser 'whatsapp', 'instagram' o 'facebook'"})
	}

	// Decode as raw map to distinguish absent fields from explicit null (null clears system_prompt)
	payload := map[string]json.RawMessage{}
	if err := json.NewDecoder(c.Request().Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo de la petición inválido"})
	}

	setClauses := []string{}
	args := []interface{}{}

	if raw, ok := payload["enabled"]; ok {
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'enabled' debe ser un booleano"})
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("enabled = $%d", len(args)))
	}

	if raw, ok := payload["model"]; ok {
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'model' debe ser una cadena de texto"})
		}
		if strings.TrimSpace(v) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'model' no puede estar vacío"})
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("model = $%d", len(args)))
	}

	if raw, ok := payload["system_prompt"]; ok {
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'system_prompt' debe ser una cadena de texto o null"})
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("system_prompt = $%d", len(args)))
	}

	if raw, ok := payload["temperature"]; ok {
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'temperature' debe ser un número"})
		}
		if v < 0 || v > 2 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'temperature' debe estar entre 0 y 2"})
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("temperature = $%d", len(args)))
	}

	if raw, ok := payload["max_tokens"]; ok {
		var v int
		if err := json.Unmarshal(raw, &v); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'max_tokens' debe ser un entero"})
		}
		if v < 1 || v > 8192 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "'max_tokens' debe estar entre 1 y 8192"})
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("max_tokens = $%d", len(args)))
	}

	query := `UPDATE agent_configs SET updated_at = now()`
	for _, clause := range setClauses {
		query += `, ` + clause
	}
	args = append(args, channel)
	query += fmt.Sprintf(` WHERE channel = $%d RETURNING %s`, len(args), agentConfigColumns)

	cfg, err := scanAgentConfig(h.pool.QueryRow(c.Request().Context(), query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("configuración de agente no encontrada para el canal '%s'", channel)})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al actualizar la configuración del agente"})
	}

	return c.JSON(http.StatusOK, cfg)
}
