package insight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Claves de app_settings. Son constantes de Go justamente para que ningun
// string escrito a mano en un handler pueda crear una fila basura.
const (
	SettingMasterEnabled = "insight.master_enabled"
	SettingASREnabled    = "insight.asr_enabled"
)

// Los tres interruptores que deciden si un mensaje se analiza:
//
//	app_settings['insight.master_enabled']   -> interruptor global
//	insight_configs[channel].enabled         -> interruptor del canal
//	conversations.insight_enabled            -> interruptor del hilo
//
// EffectiveEnabled los combina. Vive aca (y no en el handler) porque es la
// regla que hay que respeectar en TODOS los caminos: webhook, backfill,
// reintento y replay manual.
func (r *Repository) EffectiveEnabled(ctx context.Context, conversationID uuid.UUID) (bool, string, error) {
	master, err := r.GetBoolSetting(ctx, SettingMasterEnabled, true)
	if err != nil {
		return false, "", err
	}
	if !master {
		return false, SettingMasterEnabled, nil
	}

	var channel string
	var convOn, cfgOn bool
	err = r.pool.QueryRow(ctx,
		`SELECT c.channel, c.insight_enabled, COALESCE(cfg.enabled, true)
		 FROM conversations c
		 LEFT JOIN insight_configs cfg ON cfg.channel = c.channel
		 WHERE c.id = $1`, conversationID,
	).Scan(&channel, &convOn, &cfgOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "conversation_not_found", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("failed to resolve conversation flags: %w", err)
	}

	switch {
	case !convOn:
		return false, "conversation", nil
	case !cfgOn:
		return false, "channel:" + channel, nil
	}
	return true, "", nil
}

// GetBoolSetting lee un interruptor de app_settings. missing = fallback para
// que un borrado accidental no deje el sistema sin analizar nada.
func (r *Repository) GetBoolSetting(ctx context.Context, key string, fallback bool) (bool, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return fallback, fmt.Errorf("failed to read setting %s: %w", key, err)
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		// Un valor no-boolean no debe voltear la funcion: se avisa y se sigue
		// con el default, que en estos dos casos es "encendido".
		return fallback, nil
	}
	return v, nil
}

// SetBoolSetting escribe un interruptor global.
func (r *Repository) SetBoolSetting(ctx context.Context, key string, value bool, userID uuid.UUID) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// string(raw) y no raw: json.Marshal devuelve []byte, que pgx manda como
	// bytea y Postgres no puede castear a jsonb ("invalid input syntax for
	// type json"). El mismo error que en MarkOK, mismo motivo.
	_, err = r.pool.Exec(ctx,
		`INSERT INTO app_settings (key, value, updated_by, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (key) DO UPDATE
		   SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		key, string(raw), uuid.NullUUID{UUID: userID, Valid: true},
	)
	if err != nil {
		return fmt.Errorf("failed to write setting %s: %w", key, err)
	}
	return nil
}

// ConfigForChannel devuelve la config del canal. Si no existe fila (un canal
// nuevo agregado despues), devuelve los defaults en vez de fallar: un canal
// recien conectado tiene que funcionar sin una migracion extra.
func (r *Repository) ConfigForChannel(ctx context.Context, channel string) (*InsightConfig, error) {
	var c InsightConfig
	err := r.pool.QueryRow(ctx,
		`SELECT channel, enabled, asr_enabled, model, system_prompt, temperature,
		        context_messages, updated_at
		 FROM insight_configs WHERE channel = $1`, channel,
	).Scan(&c.Channel, &c.Enabled, &c.ASREnabled, &c.Model, &c.SystemPrompt,
		&c.Temperature, &c.ContextMessages, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &InsightConfig{
			Channel:         channel,
			Enabled:         true,
			ASREnabled:      true,
			Temperature:     0,
			ContextMessages: 10,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read insight config: %w", err)
	}
	return &c, nil
}

// ListConfigs devuelve la config de los tres canales known, creando las filas
// que falten. La UI siempre muestra los 3, no solo los ya configurados.
func (r *Repository) ListConfigs(ctx context.Context, channels []string) ([]InsightConfig, error) {
	out := make([]InsightConfig, 0, len(channels))
	for _, ch := range channels {
		c, err := r.ConfigForChannel(ctx, ch)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, nil
}

// UpsertConfigForChannel es el alta de un canal nuevo. Los defaults son
// enabled=true: es una excepcion JUSTIFICADA a la regla dura 6 del AGENTS.md
// porque el analizador es read-only (los agentes que RESPONDEN siguen
// arrancando apagados en agent_configs).
func (r *Repository) UpsertConfigForChannel(ctx context.Context, channel string) (*InsightConfig, error) {
	var c InsightConfig
	err := r.pool.QueryRow(ctx,
		`INSERT INTO insight_configs (channel, enabled, asr_enabled)
		 VALUES ($1, true, true)
		 ON CONFLICT (channel) DO UPDATE SET updated_at = now()
		 RETURNING channel, enabled, asr_enabled, model, system_prompt, temperature,
		           context_messages, updated_at`, channel,
	).Scan(&c.Channel, &c.Enabled, &c.ASREnabled, &c.Model, &c.SystemPrompt,
		&c.Temperature, &c.ContextMessages, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert insight config: %w", err)
	}
	return &c, nil
}

// UpdateConfig aplica un PATCH parcial. Los punteros nil = "no lo toques", que
// es lo que permite que la UI mande solo lo que cambio.
func (r *Repository) UpdateConfig(ctx context.Context, channel string, p InsightConfigPatch) (*InsightConfig, error) {
	// upsert primero: si el canal no tiene fila, el PATCH la crea con los
	// defaults y encima aplica lo pedido.
	if _, err := r.UpsertConfigForChannel(ctx, channel); err != nil {
		return nil, err
	}

	var c InsightConfig
	err := r.pool.QueryRow(ctx,
		`UPDATE insight_configs
		 SET enabled         = COALESCE($2, enabled),
		     asr_enabled     = COALESCE($3, asr_enabled),
		     model           = COALESCE(NULLIF($4, ''), model),
		     system_prompt   = COALESCE($5, system_prompt),
		     temperature     = COALESCE($6, temperature),
		     context_messages = COALESCE($7, context_messages),
		     updated_at = now()
		 WHERE channel = $1
		 RETURNING channel, enabled, asr_enabled, model, system_prompt, temperature,
		           context_messages, updated_at`,
		channel, p.Enabled, p.ASREnabled, p.Model, p.SystemPrompt, p.Temperature,
		p.ContextMessages,
	).Scan(&c.Channel, &c.Enabled, &c.ASREnabled, &c.Model, &c.SystemPrompt,
		&c.Temperature, &c.ContextMessages, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update insight config: %w", err)
	}
	return &c, nil
}

// SetConversationEnabled prende/apaga el analisis de UN hilo.
func (r *Repository) SetConversationEnabled(ctx context.Context, conversationID uuid.UUID, enabled bool) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE conversations SET insight_enabled = $2, updated_at = now() WHERE id = $1`,
		conversationID, enabled,
	)
	if err != nil {
		return false, fmt.Errorf("failed to set conversation insight: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, ErrNotFound
	}
	return true, nil
}

// QueueDepth da la foto de las colas en memoria para /status.
type QueueDepth struct {
	Text  int `json:"text"`
	Audio int `json:"audio"`
}

// StatsForStatus junta la foto de la cola y la de la DB para el endpoint de
// estado. Un unico lugar donde se responde "como viene la cosa".
type StatusReport struct {
	MasterEnabled bool           `json:"master_enabled"`
	ASREnabled    bool           `json:"asr_enabled"`
	Model         string         `json:"model"`
	ModelReady    bool           `json:"model_ready"`
	OllamaError   string         `json:"ollama_error,omitempty"`
	ASRModel      string         `json:"asr_model"`
	Counts        AnalysisCounts `json:"counts"`
	Queues        QueueDepth     `json:"queues"`
	RecentErrors  []AnalysisRow  `json:"recent_errors"`
}

// Status arma el reporte. ollamaReady y ollamaErr los mete el worker porque es
// el unico que sabe si el ultimo ping funciono.
func (r *Repository) Status(ctx context.Context, model, asrModel string, ready bool, ollamaErr error, q QueueDepth) (*StatusReport, error) {
	master, err := r.GetBoolSetting(ctx, SettingMasterEnabled, true)
	if err != nil {
		return nil, err
	}
	asr, err := r.GetBoolSetting(ctx, SettingASREnabled, true)
	if err != nil {
		return nil, err
	}
	counts, err := r.Counts(ctx)
	if err != nil {
		return nil, err
	}
	errs, err := r.ListErrors(ctx, 20)
	if err != nil {
		return nil, err
	}
	rep := &StatusReport{
		MasterEnabled: master,
		ASREnabled:    asr,
		Model:         model,
		ModelReady:    ready,
		ASRModel:      asrModel,
		Counts:        *counts,
		Queues:        q,
		RecentErrors:  errs,
	}
	if ollamaErr != nil {
		rep.OllamaError = ollamaErr.Error()
	}
	return rep, nil
}

// StaleThreshold es cuanto tiempo un 'processing' se considera muerto. Un
// analisis de texto tarda segundos; 5 minutos es holgado sin ser infinito.
const StaleThreshold = 5 * time.Minute
