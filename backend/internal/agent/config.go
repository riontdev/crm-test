package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentConfig struct {
	ID           string          `db:"id"`
	Channel      string          `db:"channel"`
	Enabled      bool            `db:"enabled"`
	Model        string          `db:"model"`
	SystemPrompt *string         `db:"system_prompt"`
	Temperature  float64         `db:"temperature"`
	MaxTokens    int             `db:"max_tokens"`
	Tools        json.RawMessage `db:"tools"`
}

type ConfigRepository struct {
	pool *pgxpool.Pool
}

func NewConfigRepository(pool *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{pool: pool}
}

// GetByChannel returns the agent config for a specific channel.
func (r *ConfigRepository) GetByChannel(ctx context.Context, channel string) (*AgentConfig, error) {
	var cfg AgentConfig
	err := r.pool.QueryRow(ctx,
		`SELECT id, channel, enabled, model, system_prompt, temperature, max_tokens, tools
		 FROM agent_configs WHERE channel = $1`, channel,
	).Scan(&cfg.ID, &cfg.Channel, &cfg.Enabled, &cfg.Model,
		&cfg.SystemPrompt, &cfg.Temperature, &cfg.MaxTokens, &cfg.Tools)

	if err != nil {
		return nil, fmt.Errorf("agent config not found for channel %s: %w", channel, err)
	}

	return &cfg, nil
}

// GetAll returns all agent configs.
func (r *ConfigRepository) GetAll(ctx context.Context) ([]AgentConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, channel, enabled, model, system_prompt, temperature, max_tokens, tools
		 FROM agent_configs ORDER BY channel`)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent configs: %w", err)
	}
	defer rows.Close()

	var configs []AgentConfig
	for rows.Next() {
		var cfg AgentConfig
		if err := rows.Scan(&cfg.ID, &cfg.Channel, &cfg.Enabled, &cfg.Model,
			&cfg.SystemPrompt, &cfg.Temperature, &cfg.MaxTokens, &cfg.Tools); err != nil {
			return nil, fmt.Errorf("failed to scan agent config: %w", err)
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

// UpdateEnabled toggles the enabled flag for a channel.
func (r *ConfigRepository) UpdateEnabled(ctx context.Context, channel string, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET enabled = $1, updated_at = now() WHERE channel = $2`,
		enabled, channel,
	)
	return err
}

// UpdatePrompt updates the system prompt for a channel.
func (r *ConfigRepository) UpdatePrompt(ctx context.Context, channel, systemPrompt string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET system_prompt = $1, updated_at = now() WHERE channel = $2`,
		systemPrompt, channel,
	)
	return err
}
