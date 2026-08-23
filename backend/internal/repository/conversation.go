package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Conversation struct {
	ID                     uuid.UUID  `db:"id"`
	ContactID              uuid.UUID  `db:"contact_id"`
	Channel                string     `db:"channel"`
	Provider               string     `db:"provider"`
	ZernioConversationID   string     `db:"zernio_conversation_id"`
	ZernioAccountID        *string    `db:"zernio_account_id"`
	PlatformConversationID *string    `db:"platform_conversation_id"`
	Status                 string     `db:"status"`
	LastInboundAt          *time.Time `db:"last_inbound_at"`
	UnreadCount            int        `db:"unread_count"`
	CreatedAt              time.Time  `db:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at"`
}

type ConversationRepository struct {
	pool *pgxpool.Pool
}

func NewConversationRepository(pool *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{pool: pool}
}

// FindByZernioID finds a conversation by its Zernio conversation ID and channel.
func (r *ConversationRepository) FindByZernioID(ctx context.Context, channel, zernioConversationID string) (*Conversation, error) {
	var conv Conversation
	err := r.pool.QueryRow(ctx,
		`SELECT id, contact_id, channel, provider, zernio_conversation_id, zernio_account_id,
		        platform_conversation_id, status, last_inbound_at, unread_count, created_at, updated_at
		 FROM conversations
		 WHERE channel = $1 AND zernio_conversation_id = $2`,
		channel, zernioConversationID,
	).Scan(&conv.ID, &conv.ContactID, &conv.Channel, &conv.Provider,
		&conv.ZernioConversationID, &conv.ZernioAccountID, &conv.PlatformConversationID,
		&conv.Status, &conv.LastInboundAt, &conv.UnreadCount,
		&conv.CreatedAt, &conv.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	return &conv, nil
}

// FindOrCreate finds an existing conversation or creates a new one.
func (r *ConversationRepository) FindOrCreate(ctx context.Context, contactID uuid.UUID, channel, provider, zernioConversationID string, zernioAccountID, platformConversationID *string) (*Conversation, error) {
	// Try to find existing
	conv, err := r.FindByZernioID(ctx, channel, zernioConversationID)
	if err == nil {
		return conv, nil
	}

	// Create new
	var newConv Conversation
	err = r.pool.QueryRow(ctx,
		`INSERT INTO conversations (contact_id, channel, provider, zernio_conversation_id, zernio_account_id, platform_conversation_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, contact_id, channel, provider, zernio_conversation_id, zernio_account_id,
		           platform_conversation_id, status, last_inbound_at, unread_count, created_at, updated_at`,
		contactID, channel, provider, zernioConversationID, zernioAccountID, platformConversationID,
	).Scan(&newConv.ID, &newConv.ContactID, &newConv.Channel, &newConv.Provider,
		&newConv.ZernioConversationID, &newConv.ZernioAccountID, &newConv.PlatformConversationID,
		&newConv.Status, &newConv.LastInboundAt, &newConv.UnreadCount,
		&newConv.CreatedAt, &newConv.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return &newConv, nil
}

// UpdateLastInboundAt updates the last_inbound_at timestamp for a conversation.
// This is critical for the Meta 24h window rule.
func (r *ConversationRepository) UpdateLastInboundAt(ctx context.Context, conversationID uuid.UUID, lastInboundAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET last_inbound_at = $1, updated_at = now() WHERE id = $2`,
		lastInboundAt, conversationID,
	)
	return err
}

// IncrementUnread increments the unread counter for a conversation.
func (r *ConversationRepository) IncrementUnread(ctx context.Context, conversationID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET unread_count = unread_count + 1, updated_at = now() WHERE id = $1`,
		conversationID,
	)
	return err
}

// ResetUnread resets the unread counter to 0.
func (r *ConversationRepository) ResetUnread(ctx context.Context, conversationID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET unread_count = 0, updated_at = now() WHERE id = $1`,
		conversationID,
	)
	return err
}

// UpdateStatus sets the conversation status (active | archived).
func (r *ConversationRepository) UpdateStatus(ctx context.Context, conversationID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET status = $2, updated_at = now() WHERE id = $1`,
		conversationID, status,
	)
	return err
}

// ListByContact returns all conversations for a contact, ordered by most recent.
func (r *ConversationRepository) ListByContact(ctx context.Context, contactID uuid.UUID) ([]Conversation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, contact_id, channel, provider, zernio_conversation_id, zernio_account_id,
		        platform_conversation_id, status, last_inbound_at, unread_count, created_at, updated_at
		 FROM conversations
		 WHERE contact_id = $1
		 ORDER BY updated_at DESC`, contactID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.ContactID, &conv.Channel, &conv.Provider,
			&conv.ZernioConversationID, &conv.ZernioAccountID, &conv.PlatformConversationID,
			&conv.Status, &conv.LastInboundAt, &conv.UnreadCount,
			&conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// ListActive returns conversations filtered by channel and status, ordered by most recent activity.
func (r *ConversationRepository) ListActive(ctx context.Context, channel, status string) ([]Conversation, error) {
	query := `SELECT id, contact_id, channel, provider, zernio_conversation_id, zernio_account_id,
	                 platform_conversation_id, status, last_inbound_at, unread_count, created_at, updated_at
	          FROM conversations WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if channel != "" {
		query += ` AND channel = $` + strconv.Itoa(argIdx)
		args = append(args, channel)
		argIdx++
	}
	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY updated_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.ContactID, &conv.Channel, &conv.Provider,
			&conv.ZernioConversationID, &conv.ZernioAccountID, &conv.PlatformConversationID,
			&conv.Status, &conv.LastInboundAt, &conv.UnreadCount,
			&conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// GetByID returns a conversation by its UUID.
func (r *ConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	var conv Conversation
	err := r.pool.QueryRow(ctx,
		`SELECT id, contact_id, channel, provider, zernio_conversation_id, zernio_account_id,
		        platform_conversation_id, status, last_inbound_at, unread_count, created_at, updated_at
		 FROM conversations WHERE id = $1`, id,
	).Scan(&conv.ID, &conv.ContactID, &conv.Channel, &conv.Provider,
		&conv.ZernioConversationID, &conv.ZernioAccountID, &conv.PlatformConversationID,
		&conv.Status, &conv.LastInboundAt, &conv.UnreadCount,
		&conv.CreatedAt, &conv.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	return &conv, nil
}
