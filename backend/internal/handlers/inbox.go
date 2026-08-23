package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/riont/crm/backend/internal/repository"
)

type InboxHandler struct {
	conversations *repository.ConversationRepository
	messages      *repository.MessageRepository
	contacts      *repository.ContactRepository
}

func NewInboxHandler(
	conversations *repository.ConversationRepository,
	messages *repository.MessageRepository,
	contacts *repository.ContactRepository,
) *InboxHandler {
	return &InboxHandler{
		conversations: conversations,
		messages:      messages,
		contacts:      contacts,
	}
}

// ListConversations returns all conversations for the authenticated user.
// GET /api/inbox/conversations?channel=whatsapp&status=active
func (h *InboxHandler) ListConversations(c echo.Context) error {
	channel := c.QueryParam("channel")
	status := c.QueryParam("status")

	conversations, err := h.conversations.ListActive(c.Request().Context(), channel, status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list conversations"})
	}

	type ContactSummary struct {
		ID   uuid.UUID `json:"id"`
		Name *string   `json:"name,omitempty"`
	}

	type MessageSummary struct {
		Text      *string `json:"text,omitempty"`
		Direction string  `json:"direction"`
		SentAt    *string `json:"sent_at,omitempty"`
	}

	type ConversationResponse struct {
		ID                   uuid.UUID       `json:"id"`
		Channel              string          `json:"channel"`
		Provider             string          `json:"provider"`
		ZernioConversationID string          `json:"zernio_conversation_id"`
		ZernioAccountID      *string         `json:"zernio_account_id,omitempty"`
		Status               string          `json:"status"`
		LastInboundAt        *string         `json:"last_inbound_at,omitempty"`
		UnreadCount          int             `json:"unread_count"`
		CreatedAt            string          `json:"created_at"`
		UpdatedAt            string          `json:"updated_at"`
		Contact              *ContactSummary `json:"contact,omitempty"`
		LastMessage          *MessageSummary `json:"last_message,omitempty"`
	}

	var results []ConversationResponse
	for _, conv := range conversations {
		resp := ConversationResponse{
			ID:                   conv.ID,
			Channel:              conv.Channel,
			Provider:             conv.Provider,
			ZernioConversationID: conv.ZernioConversationID,
			ZernioAccountID:      conv.ZernioAccountID,
			Status:               conv.Status,
			UnreadCount:          conv.UnreadCount,
			CreatedAt:            conv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:            conv.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if conv.LastInboundAt != nil {
			s := conv.LastInboundAt.Format("2006-01-02T15:04:05Z07:00")
			resp.LastInboundAt = &s
		}

		// Fetch contact name
		contact, err := h.contacts.GetByID(c.Request().Context(), conv.ContactID)
		if err == nil {
			resp.Contact = &ContactSummary{
				ID:   contact.ID,
				Name: contact.Name,
			}
		}

		// Fetch last message
		msgs, err := h.messages.ListByConversation(c.Request().Context(), conv.ID, 1, 0)
		if err == nil && len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			resp.LastMessage = &MessageSummary{
				Text:      lastMsg.Text,
				Direction: lastMsg.Direction,
			}
			if lastMsg.SentAt != nil {
				s := lastMsg.SentAt.Format("2006-01-02T15:04:05Z07:00")
				resp.LastMessage.SentAt = &s
			}
		}

		results = append(results, resp)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":  results,
		"count": len(results),
	})
}

// GetConversation returns a single conversation with its messages.
// GET /api/inbox/conversations/:id
func (h *InboxHandler) GetConversation(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid conversation id"})
	}

	conv, err := h.conversations.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "conversation not found"})
	}

	contact, err := h.contacts.GetByID(c.Request().Context(), conv.ContactID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch contact"})
	}

	msgLimit := 50
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			msgLimit = v
		}
	}

	msgOffset := 0
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			msgOffset = v
		}
	}

	msgs, err := h.messages.ListByConversation(c.Request().Context(), conv.ID, msgLimit, msgOffset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list messages"})
	}

	// Reset unread counter when conversation is opened
	h.conversations.ResetUnread(c.Request().Context(), conv.ID)

	type ContactResponse struct {
		ID        uuid.UUID `json:"id"`
		Name      *string   `json:"name,omitempty"`
		Phone     *string   `json:"phone,omitempty"`
		Email     *string   `json:"email,omitempty"`
		AvatarURL *string   `json:"avatar_url,omitempty"`
		Company   *string   `json:"company,omitempty"`
		Tags      []string  `json:"tags,omitempty"`
	}

	type MsgResponse struct {
		ID                uuid.UUID       `json:"id"`
		ExternalID        string          `json:"external_id"`
		Direction         string          `json:"direction"`
		Text              *string         `json:"text,omitempty"`
		Attachments       json.RawMessage `json:"attachments"`
		SenderType        string          `json:"sender_type"`
		Status            string          `json:"status"`
		PlatformMessageID *string         `json:"platform_message_id,omitempty"`
		SentAt            *string         `json:"sent_at,omitempty"`
		CreatedAt         string          `json:"created_at"`
	}

	type ConversationDetail struct {
		ID                   uuid.UUID       `json:"id"`
		Channel              string          `json:"channel"`
		Provider             string          `json:"provider"`
		ZernioConversationID string          `json:"zernio_conversation_id"`
		ZernioAccountID      *string         `json:"zernio_account_id,omitempty"`
		Status               string          `json:"status"`
		LastInboundAt        *string         `json:"last_inbound_at,omitempty"`
		UnreadCount          int             `json:"unread_count"`
		CreatedAt            string          `json:"created_at"`
		UpdatedAt            string          `json:"updated_at"`
		Contact              ContactResponse `json:"contact"`
		Messages             []MsgResponse   `json:"messages"`
	}

	resp := ConversationDetail{
		ID:                   conv.ID,
		Channel:              conv.Channel,
		Provider:             conv.Provider,
		ZernioConversationID: conv.ZernioConversationID,
		ZernioAccountID:      conv.ZernioAccountID,
		Status:               conv.Status,
		UnreadCount:          conv.UnreadCount,
		CreatedAt:            conv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:            conv.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Contact: ContactResponse{
			ID:    contact.ID,
			Name:  contact.Name,
			Phone: contact.Phone,
			Email: contact.Email,
		},
	}

	if contact.AvatarURL != nil {
		resp.Contact.AvatarURL = contact.AvatarURL
	}

	if contact.Company != nil {
		resp.Contact.Company = contact.Company
	}

	if len(contact.Tags) > 0 {
		resp.Contact.Tags = contact.Tags
	}

	if conv.LastInboundAt != nil {
		s := conv.LastInboundAt.Format("2006-01-02T15:04:05Z07:00")
		resp.LastInboundAt = &s
	}

	for _, msg := range msgs {
		mr := MsgResponse{
			ID:          msg.ID,
			ExternalID:  msg.ExternalID,
			Direction:   msg.Direction,
			Text:        msg.Text,
			Attachments: msg.Attachments,
			SenderType:  msg.SenderType,
			Status:      msg.Status,
		}
		if msg.PlatformMessageID != nil {
			mr.PlatformMessageID = msg.PlatformMessageID
		}
		if msg.SentAt != nil {
			s := msg.SentAt.Format("2006-01-02T15:04:05Z07:00")
			mr.SentAt = &s
		}
		mr.CreatedAt = msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.Messages = append(resp.Messages, mr)
	}

	return c.JSON(http.StatusOK, resp)
}

// UpdateConversation modifies conversation fields (currently: status).
// PATCH /api/inbox/conversations/:id
func (h *InboxHandler) UpdateConversation(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid conversation id"})
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.Status != "active" && req.Status != "archived" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "status must be 'active' or 'archived'"})
	}

	if err := h.conversations.UpdateStatus(c.Request().Context(), id, req.Status); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update conversation"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":     id,
		"status": req.Status,
	})
}
