package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Message struct {
	ID                uuid.UUID       `db:"id"`
	ConversationID    uuid.UUID       `db:"conversation_id"`
	ExternalID        string          `db:"external_id"`
	Direction         string          `db:"direction"`
	Text              *string         `db:"text"`
	Attachments       json.RawMessage `db:"attachments"`
	SenderType        string          `db:"sender_type"`
	SenderContactID   *uuid.UUID      `db:"sender_contact_id"`
	PlatformMessageID *string         `db:"platform_message_id"`
	Status            string          `db:"status"`
	Metadata          json.RawMessage `db:"metadata"`
	SentAt            *time.Time      `db:"sent_at"`
	CreatedAt         time.Time       `db:"created_at"`
}

type MessageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

// InsertMessage inserts a new message. Uses external_id as the idempotency key.
// Returns the message if newly created, or finds the existing one on conflict.
func (r *MessageRepository) InsertMessage(ctx context.Context, msg *Message) (*Message, error) {
	var result Message
	err := r.pool.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, external_id, direction, text, attachments, sender_type, sender_contact_id, platform_message_id, status, metadata, sent_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (external_id) DO NOTHING
		 RETURNING id, conversation_id, external_id, direction, text, attachments, sender_type, sender_contact_id, platform_message_id, status, metadata, sent_at, created_at`,
		msg.ConversationID, msg.ExternalID, msg.Direction, msg.Text, msg.Attachments,
		msg.SenderType, msg.SenderContactID, msg.PlatformMessageID, msg.Status,
		msg.Metadata, msg.SentAt,
	).Scan(&result.ID, &result.ConversationID, &result.ExternalID, &result.Direction,
		&result.Text, &result.Attachments, &result.SenderType, &result.SenderContactID,
		&result.PlatformMessageID, &result.Status, &result.Metadata, &result.SentAt, &result.CreatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			// Message already exists (duplicate), fetch it
			return r.FindByExternalID(ctx, msg.ExternalID)
		}
		return nil, fmt.Errorf("failed to insert message: %w", err)
	}

	return &result, nil
}

// FindByExternalID returns a message by its external_id.
func (r *MessageRepository) FindByExternalID(ctx context.Context, externalID string) (*Message, error) {
	var msg Message
	err := r.pool.QueryRow(ctx,
		`SELECT id, conversation_id, external_id, direction, text, attachments, sender_type, sender_contact_id, platform_message_id, status, metadata, sent_at, created_at
		 FROM messages WHERE external_id = $1`, externalID,
	).Scan(&msg.ID, &msg.ConversationID, &msg.ExternalID, &msg.Direction,
		&msg.Text, &msg.Attachments, &msg.SenderType, &msg.SenderContactID,
		&msg.PlatformMessageID, &msg.Status, &msg.Metadata, &msg.SentAt, &msg.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}

	return &msg, nil
}

// ListByConversation returns messages for a conversation, ordered by created_at.
func (r *MessageRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int, offset int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, conversation_id, external_id, direction, text, attachments, sender_type, sender_contact_id, platform_message_id, status, metadata, sent_at, created_at
		 FROM messages
		 WHERE conversation_id = $1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`, conversationID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.ExternalID, &msg.Direction,
			&msg.Text, &msg.Attachments, &msg.SenderType, &msg.SenderContactID,
			&msg.PlatformMessageID, &msg.Status, &msg.Metadata, &msg.SentAt, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// UpdateStatus updates the status of a message.
func (r *MessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*Message, error) {
	var msg Message
	err := r.pool.QueryRow(ctx,
		`UPDATE messages SET status = $1
		 WHERE id = $2
		 RETURNING id, conversation_id, external_id, direction, text, attachments, sender_type, sender_contact_id, platform_message_id, status, metadata, sent_at, created_at`,
		status, id,
	).Scan(&msg.ID, &msg.ConversationID, &msg.ExternalID, &msg.Direction,
		&msg.Text, &msg.Attachments, &msg.SenderType, &msg.SenderContactID,
		&msg.PlatformMessageID, &msg.Status, &msg.Metadata, &msg.SentAt, &msg.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to update message status: %w", err)
	}

	return &msg, nil
}
