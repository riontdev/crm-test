package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type WebhookEvent struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	EventID   string          `db:"event_id" json:"event_id"`
	EventType string          `db:"event_type" json:"event_type"`
	Payload   json.RawMessage `db:"payload" json:"payload"`
	Processed bool            `db:"processed" json:"processed"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}
