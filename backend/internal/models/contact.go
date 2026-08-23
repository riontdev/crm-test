package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Contact struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	Name      *string         `db:"name" json:"name,omitempty"`
	AvatarURL *string         `db:"avatar_url" json:"avatar_url,omitempty"`
	Phone     *string         `db:"phone" json:"phone,omitempty"`
	Email     *string         `db:"email" json:"email,omitempty"`
	Company   *string         `db:"company" json:"company,omitempty"`
	Tags      []string        `db:"tags" json:"tags"`
	Notes     *string         `db:"notes" json:"notes,omitempty"`
	Metadata  json.RawMessage `db:"metadata" json:"metadata"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}
