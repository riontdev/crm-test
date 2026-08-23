package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookEvent struct {
	ID        string          `db:"id"`
	EventID   string          `db:"event_id"`
	EventType string          `db:"event_type"`
	Payload   json.RawMessage `db:"payload"`
	Processed bool            `db:"processed"`
}

type WebhookEventRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookEventRepository(pool *pgxpool.Pool) *WebhookEventRepository {
	return &WebhookEventRepository{pool: pool}
}

// ClaimEvent attempts to claim a webhook event by inserting its ID.
// Returns (true, nil) if this is a new event that should be processed.
// Returns (false, nil) if this is a duplicate/retry — caller should return 200 immediately.
// Returns (false, err) on actual errors.
func (r *WebhookEventRepository) ClaimEvent(ctx context.Context, eventID, eventType string, payload json.RawMessage) (bool, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO webhook_events (event_id, event_type, payload)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (event_id) DO NOTHING
		 RETURNING id`,
		eventID, eventType, payload,
	).Scan(&id)

	if err != nil {
		// pgx returns pgx.ErrNoRows when ON CONFLICT DO NOTHING fires (no row to return)
		if err.Error() == "no rows in result set" {
			return false, nil // duplicate, already processed
		}
		return false, fmt.Errorf("failed to claim webhook event: %w", err)
	}

	return true, nil // new event claimed
}

// MarkProcessed marks a webhook event as processed.
func (r *WebhookEventRepository) MarkProcessed(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE webhook_events SET processed = true WHERE event_id = $1`,
		eventID,
	)
	return err
}
