package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContactIdentity struct {
	ID               uuid.UUID  `db:"id"`
	ContactID        uuid.UUID  `db:"contact_id"`
	Channel          string     `db:"channel"`
	Provider         string     `db:"provider"`
	ExternalID       string     `db:"external_id"`
	ProviderUsername *string    `db:"provider_username"`
	ProviderName     *string    `db:"provider_name"`
	ProviderAvatar   *string    `db:"provider_avatar"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

type ContactIdentityRepository struct {
	pool *pgxpool.Pool
}

func NewContactIdentityRepository(pool *pgxpool.Pool) *ContactIdentityRepository {
	return &ContactIdentityRepository{pool: pool}
}

// FindOrCreate creates a contact identity if it doesn't exist.
// Returns the existing identity if (channel, external_id) already exists.
func (r *ContactIdentityRepository) FindOrCreate(ctx context.Context, contactID uuid.UUID, channel, provider, externalID string, username, name, avatar *string) (*ContactIdentity, error) {
	var identity ContactIdentity

	// Try to find existing
	err := r.pool.QueryRow(ctx,
		`SELECT id, contact_id, channel, provider, external_id, provider_username, provider_name, provider_avatar, created_at, updated_at
		 FROM contact_identities
		 WHERE channel = $1 AND external_id = $2
		 LIMIT 1`, channel, externalID,
	).Scan(&identity.ID, &identity.ContactID, &identity.Channel, &identity.Provider,
		&identity.ExternalID, &identity.ProviderUsername, &identity.ProviderName,
		&identity.ProviderAvatar, &identity.CreatedAt, &identity.UpdatedAt)

	if err == nil {
		return &identity, nil
	}

	// Create new
	err = r.pool.QueryRow(ctx,
		`INSERT INTO contact_identities (contact_id, channel, provider, external_id, provider_username, provider_name, provider_avatar)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, contact_id, channel, provider, external_id, provider_username, provider_name, provider_avatar, created_at, updated_at`,
		contactID, channel, provider, externalID, username, name, avatar,
	).Scan(&identity.ID, &identity.ContactID, &identity.Channel, &identity.Provider,
		&identity.ExternalID, &identity.ProviderUsername, &identity.ProviderName,
		&identity.ProviderAvatar, &identity.CreatedAt, &identity.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create contact identity: %w", err)
	}

	return &identity, nil
}

// GetByChannelAndExternalID returns an identity by channel and external_id.
func (r *ContactIdentityRepository) GetByChannelAndExternalID(ctx context.Context, channel, externalID string) (*ContactIdentity, error) {
	var identity ContactIdentity
	err := r.pool.QueryRow(ctx,
		`SELECT id, contact_id, channel, provider, external_id, provider_username, provider_name, provider_avatar, created_at, updated_at
		 FROM contact_identities
		 WHERE channel = $1 AND external_id = $2`,
		channel, externalID,
	).Scan(&identity.ID, &identity.ContactID, &identity.Channel, &identity.Provider,
		&identity.ExternalID, &identity.ProviderUsername, &identity.ProviderName,
		&identity.ProviderAvatar, &identity.CreatedAt, &identity.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("identity not found: %w", err)
	}

	return &identity, nil
}
