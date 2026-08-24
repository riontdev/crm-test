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
	AssignedTo             uuid.NullUUID `db:"assigned_to"`
	CreatedAt              time.Time  `db:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at"`
}

const conversationColumns = `id, contact_id, channel, provider, zernio_conversation_id, zernio_account_id,
	platform_conversation_id, status, last_inbound_at, unread_count, assigned_to, created_at, updated_at`

const conversationColumnsQualified = `c.id, c.contact_id, c.channel, c.provider, c.zernio_conversation_id, c.zernio_account_id,
	c.platform_conversation_id, c.status, c.last_inbound_at, c.unread_count, c.assigned_to, c.created_at, c.updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanConversation(s scanner) (*Conversation, error) {
	var conv Conversation
	err := s.Scan(&conv.ID, &conv.ContactID, &conv.Channel, &conv.Provider,
		&conv.ZernioConversationID, &conv.ZernioAccountID, &conv.PlatformConversationID,
		&conv.Status, &conv.LastInboundAt, &conv.UnreadCount, &conv.AssignedTo,
		&conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

type ConversationRepository struct {
	pool *pgxpool.Pool
}

func NewConversationRepository(pool *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{pool: pool}
}

// FindByZernioID finds a conversation by its Zernio conversation ID and channel.
func (r *ConversationRepository) FindByZernioID(ctx context.Context, channel, zernioConversationID string) (*Conversation, error) {
	conv, err := scanConversation(r.pool.QueryRow(ctx,
		`SELECT `+conversationColumns+`
		 FROM conversations
		 WHERE channel = $1 AND zernio_conversation_id = $2`,
		channel, zernioConversationID,
	))
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	return conv, nil
}

// FindOrCreate finds an existing conversation or creates a new one.
func (r *ConversationRepository) FindOrCreate(ctx context.Context, contactID uuid.UUID, channel, provider, zernioConversationID string, zernioAccountID, platformConversationID *string) (*Conversation, error) {
	// Try to find existing
	conv, err := r.FindByZernioID(ctx, channel, zernioConversationID)
	if err == nil {
		return conv, nil
	}

	// Create new
	newConv, err := scanConversation(r.pool.QueryRow(ctx,
		`INSERT INTO conversations (contact_id, channel, provider, zernio_conversation_id, zernio_account_id, platform_conversation_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+conversationColumns,
		contactID, channel, provider, zernioConversationID, zernioAccountID, platformConversationID,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return newConv, nil
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

// UpdateAssignedTo assigns (or unassigns with nil) a user to a conversation.
// User existence is validated by the FK plus handler pre-check.
func (r *ConversationRepository) UpdateAssignedTo(ctx context.Context, conversationID uuid.UUID, userID *uuid.UUID) error {
	param := uuid.NullUUID{}
	if userID != nil {
		param = uuid.NullUUID{UUID: *userID, Valid: true}
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET assigned_to = $1, updated_at = now() WHERE id = $2`,
		param, conversationID,
	)
	return err
}

// ListByContact returns all conversations for a contact, ordered by most recent.
func (r *ConversationRepository) ListByContact(ctx context.Context, contactID uuid.UUID) ([]Conversation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+conversationColumns+`
		 FROM conversations
		 WHERE contact_id = $1
		 ORDER BY updated_at DESC`, contactID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		conv, err := scanConversation(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, *conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate conversations: %w", err)
	}

	return conversations, nil
}

// ListActive returns conversations filtered by channel and status (empty filters match all),
// ordered by most recent activity, with pagination. Returns the page plus total matching count.
func (r *ConversationRepository) ListActive(ctx context.Context, channel, status string, limit, offset int) ([]Conversation, int64, error) {
	if limit < 1 {
		limit = 1
	}
	if offset < 0 {
		offset = 0
	}

	filters := ""
	args := []interface{}{}
	argIdx := 1

	if channel != "" {
		filters += ` AND channel = $` + strconv.Itoa(argIdx)
		args = append(args, channel)
		argIdx++
	}
	if status != "" {
		filters += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query := `SELECT ` + conversationColumns + ` FROM conversations WHERE 1=1` + filters +
		` ORDER BY updated_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	listArgs := append(append([]interface{}{}, args...), limit, offset)

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		conv, err := scanConversation(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, *conv)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate conversations: %w", err)
	}

	var total int64
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM conversations WHERE 1=1`+filters, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count conversations: %w", err)
	}

	return conversations, total, nil
}

// SearchConversations returns conversations whose contact name/phone or any message
// text matches the query, ordered by most recent activity.
func (r *ConversationRepository) SearchConversations(ctx context.Context, q string, limit int) ([]Conversation, error) {
	if limit < 1 {
		limit = 10
	}

	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT `+conversationColumnsQualified+`
		 FROM conversations c
		 JOIN contacts ct ON ct.id = c.contact_id
		 WHERE (
		   ct.name ILIKE '%'||$1||'%' OR ct.phone ILIKE '%'||$1||'%'
		   OR EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id AND m.text ILIKE '%'||$1||'%')
		 )
		 ORDER BY c.updated_at DESC
		 LIMIT $2`, q, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search conversations: %w", err)
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		conv, err := scanConversation(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, *conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate conversations: %w", err)
	}

	return conversations, nil
}

// UnreadConversations returns active conversations with pending messages,
// ordered by most recent inbound activity.
func (r *ConversationRepository) UnreadConversations(ctx context.Context, limit int) ([]Conversation, error) {
	if limit < 1 {
		limit = 8
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+conversationColumns+`
		 FROM conversations
		 WHERE unread_count > 0 AND status = 'active'
		 ORDER BY last_inbound_at DESC NULLS LAST
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list unread conversations: %w", err)
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		conv, err := scanConversation(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, *conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate conversations: %w", err)
	}

	return conversations, nil
}

// UnreadSummary returns the sum of unread_count across all active conversations.
func (r *ConversationRepository) UnreadSummary(ctx context.Context) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(unread_count), 0) FROM conversations WHERE status = 'active'`,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum unread counts: %w", err)
	}
	return total, nil
}

// AssignedNames resolves user names for the given IDs (missing IDs are simply absent from the map).
func (r *ConversationRepository) AssignedNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}

	// Sin ANY($1): el array como parámetro no es soportado en protocolo simple.
	// El workspace es pequeño: traemos todos y filtramos en memoria.
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM users`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch assigned users: %w", err)
	}
	defer rows.Close()

	all := make(map[uuid.UUID]string)
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("failed to scan assigned user: %w", err)
		}
		all[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate assigned users: %w", err)
	}

	for _, id := range ids {
		if name, ok := all[id]; ok {
			names[id] = name
		}
	}
	return names, nil
}

// GetByID returns a conversation by its UUID.
func (r *ConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	conv, err := scanConversation(r.pool.QueryRow(ctx,
		`SELECT `+conversationColumns+`
		 FROM conversations WHERE id = $1`, id,
	))
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	return conv, nil
}
