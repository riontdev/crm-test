package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contact struct {
	ID        uuid.UUID       `db:"id"`
	Name      *string         `db:"name"`
	AvatarURL *string         `db:"avatar_url"`
	Phone     *string         `db:"phone"`
	Email     *string         `db:"email"`
	Company   *string         `db:"company"`
	Tags      []string        `db:"tags"`
	Notes     *string         `db:"notes"`
	Metadata  json.RawMessage `db:"metadata"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

type ContactRepository struct {
	pool *pgxpool.Pool
}

func NewContactRepository(pool *pgxpool.Pool) *ContactRepository {
	return &ContactRepository{pool: pool}
}

// FindOrCreateByPhone finds an existing contact by phone or creates a new one.
func (r *ContactRepository) FindOrCreateByPhone(ctx context.Context, phone string, name, avatarURL *string) (*Contact, error) {
	// Try to find existing contact via contact_identities
	var contact Contact
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.name, c.avatar_url, c.phone, c.email, c.company, c.tags, c.notes, c.metadata, c.created_at, c.updated_at
		 FROM contacts c
		 JOIN contact_identities ci ON ci.contact_id = c.id
		 WHERE ci.channel = 'whatsapp' AND ci.external_id = $1
		 LIMIT 1`, phone,
	).Scan(&contact.ID, &contact.Name, &contact.AvatarURL, &contact.Phone, &contact.Email,
		&contact.Company, &contact.Tags, &contact.Notes, &contact.Metadata,
		&contact.CreatedAt, &contact.UpdatedAt)

	if err == nil {
		return &contact, nil
	}

	// Create new contact
	err = r.pool.QueryRow(ctx,
		`INSERT INTO contacts (name, avatar_url, phone, metadata)
		 VALUES ($1, $2, $3, '{}')
		 RETURNING id, name, avatar_url, phone, email, company, tags, notes, metadata, created_at, updated_at`,
		name, avatarURL, phone,
	).Scan(&contact.ID, &contact.Name, &contact.AvatarURL, &contact.Phone, &contact.Email,
		&contact.Company, &contact.Tags, &contact.Notes, &contact.Metadata,
		&contact.CreatedAt, &contact.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	return &contact, nil
}

// FindOrCreateByExternalID finds a contact by platform external_id or creates one.
func (r *ContactRepository) FindOrCreateByExternalID(ctx context.Context, channel, externalID string, name, avatarURL *string) (*Contact, error) {
	// Try to find via contact_identities
	var contact Contact
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.name, c.avatar_url, c.phone, c.email, c.company, c.tags, c.notes, c.metadata, c.created_at, c.updated_at
		 FROM contacts c
		 JOIN contact_identities ci ON ci.contact_id = c.id
		 WHERE ci.channel = $1 AND ci.external_id = $2
		 LIMIT 1`, channel, externalID,
	).Scan(&contact.ID, &contact.Name, &contact.AvatarURL, &contact.Phone, &contact.Email,
		&contact.Company, &contact.Tags, &contact.Notes, &contact.Metadata,
		&contact.CreatedAt, &contact.UpdatedAt)

	if err == nil {
		return &contact, nil
	}

	// Create new contact
	err = r.pool.QueryRow(ctx,
		`INSERT INTO contacts (name, avatar_url, metadata)
		 VALUES ($1, $2, '{}')
		 RETURNING id, name, avatar_url, phone, email, company, tags, notes, metadata, created_at, updated_at`,
		name, avatarURL,
	).Scan(&contact.ID, &contact.Name, &contact.AvatarURL, &contact.Phone, &contact.Email,
		&contact.Company, &contact.Tags, &contact.Notes, &contact.Metadata,
		&contact.CreatedAt, &contact.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	return &contact, nil
}

// GetByID returns a contact by its UUID.
func (r *ContactRepository) GetByID(ctx context.Context, id uuid.UUID) (*Contact, error) {
	var contact Contact
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, avatar_url, phone, email, company, tags, notes, metadata, created_at, updated_at
		 FROM contacts WHERE id = $1`, id,
	).Scan(&contact.ID, &contact.Name, &contact.AvatarURL, &contact.Phone, &contact.Email,
		&contact.Company, &contact.Tags, &contact.Notes, &contact.Metadata,
		&contact.CreatedAt, &contact.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("contact not found: %w", err)
	}

	return &contact, nil
}

// UpdateNotes sets the notes for a contact (nil or empty clears them).
func (r *ContactRepository) UpdateNotes(ctx context.Context, contactID uuid.UUID, notes *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE contacts SET notes = $2, updated_at = now() WHERE id = $1`,
		contactID, notes,
	)
	return err
}
