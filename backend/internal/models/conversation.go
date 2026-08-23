package models

import (
	"time"

	"github.com/google/uuid"
)

type Conversation struct {
	ID                     uuid.UUID  `db:"id" json:"id"`
	ContactID              uuid.UUID  `db:"contact_id" json:"contact_id"`
	Channel                string     `db:"channel" json:"channel"`
	Provider               string     `db:"provider" json:"provider"`
	ZernioConversationID   string     `db:"zernio_conversation_id" json:"zernio_conversation_id"`
	ZernioAccountID        *string    `db:"zernio_account_id" json:"zernio_account_id,omitempty"`
	PlatformConversationID *string    `db:"platform_conversation_id" json:"platform_conversation_id,omitempty"`
	Status                 string     `db:"status" json:"status"`
	LastInboundAt          *time.Time `db:"last_inbound_at" json:"last_inbound_at,omitempty"`
	UnreadCount            int        `db:"unread_count" json:"unread_count"`
	CreatedAt              time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at" json:"updated_at"`
}
