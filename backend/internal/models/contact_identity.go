package models

import (
	"time"

	"github.com/google/uuid"
)

type ContactIdentity struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	ContactID        uuid.UUID  `db:"contact_id" json:"contact_id"`
	Channel          string     `db:"channel" json:"channel"`
	Provider         string     `db:"provider" json:"provider"`
	ExternalID       string     `db:"external_id" json:"external_id"`
	ProviderUsername *string    `db:"provider_username" json:"provider_username,omitempty"`
	ProviderName     *string    `db:"provider_name" json:"provider_name,omitempty"`
	ProviderAvatar   *string    `db:"provider_avatar" json:"provider_avatar,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}
