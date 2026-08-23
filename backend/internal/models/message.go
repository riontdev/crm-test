package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID                uuid.UUID       `db:"id" json:"id"`
	ConversationID    uuid.UUID       `db:"conversation_id" json:"conversation_id"`
	ExternalID        string          `db:"external_id" json:"external_id"`
	Direction         string          `db:"direction" json:"direction"`
	Text              *string         `db:"text" json:"text,omitempty"`
	Attachments       json.RawMessage `db:"attachments" json:"attachments"`
	SenderType        string          `db:"sender_type" json:"sender_type"`
	SenderContactID   *uuid.UUID      `db:"sender_contact_id" json:"sender_contact_id,omitempty"`
	PlatformMessageID *string         `db:"platform_message_id" json:"platform_message_id,omitempty"`
	Status            string          `db:"status" json:"status"`
	Metadata          json.RawMessage `db:"metadata" json:"metadata"`
	SentAt            *time.Time      `db:"sent_at" json:"sent_at"`
	CreatedAt         time.Time       `db:"created_at" json:"created_at"`
}
