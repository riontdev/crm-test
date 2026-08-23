package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AgentConfig struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	Channel      string          `db:"channel" json:"channel"`
	Enabled      bool            `db:"enabled" json:"enabled"`
	Model        string          `db:"model" json:"model"`
	SystemPrompt *string         `db:"system_prompt" json:"system_prompt,omitempty"`
	Temperature  float64         `db:"temperature" json:"temperature"`
	MaxTokens    int             `db:"max_tokens" json:"max_tokens"`
	Tools        json.RawMessage `db:"tools" json:"tools"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}
