package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/riont/crm/backend/internal/repository"
)

const timeLayout = "2006-01-02T15:04:05Z07:00"

type ContactSummary struct {
	ID        uuid.UUID `json:"id"`
	Name      *string   `json:"name,omitempty"`
	Phone     *string   `json:"phone,omitempty"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

type MessageSummary struct {
	Text      *string `json:"text,omitempty"`
	Direction string  `json:"direction"`
	SentAt    *string `json:"sent_at,omitempty"`
}

type AssigneeSummary struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ConversationResponse struct {
	ID                   uuid.UUID        `json:"id"`
	Channel              string           `json:"channel"`
	Provider             string           `json:"provider"`
	ZernioConversationID string           `json:"zernio_conversation_id"`
	ZernioAccountID      *string          `json:"zernio_account_id,omitempty"`
	Status               string           `json:"status"`
	LastInboundAt        *string          `json:"last_inbound_at,omitempty"`
	UnreadCount          int              `json:"unread_count"`
	CreatedAt            string           `json:"created_at"`
	UpdatedAt            string           `json:"updated_at"`
	Contact              *ContactSummary  `json:"contact,omitempty"`
	LastMessage          *MessageSummary  `json:"last_message,omitempty"`
	AssignedTo           *AssigneeSummary `json:"assigned_to"`
}

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

func formatTime(t interface{ Format(string) string }) *string {
	s := t.Format(timeLayout)
	return &s
}

// enrichConversationResponses builds the response page for a set of conversations:
// contact info, last message snippet and assignee names (single batched query).
func (h *InboxHandler) enrichConversationResponses(ctx context.Context, conversations []repository.Conversation) ([]ConversationResponse, error) {
	idSet := make(map[uuid.UUID]struct{})
	for _, conv := range conversations {
		if conv.AssignedTo.Valid {
			idSet[conv.AssignedTo.UUID] = struct{}{}
		}
	}
	ids := make([]uuid.UUID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	names, err := h.conversations.AssignedNames(ctx, ids)
	if err != nil {
		return nil, err
	}

	results := make([]ConversationResponse, 0, len(conversations))
	for _, conv := range conversations {
		resp := ConversationResponse{
			ID:                   conv.ID,
			Channel:              conv.Channel,
			Provider:             conv.Provider,
			ZernioConversationID: conv.ZernioConversationID,
			ZernioAccountID:      conv.ZernioAccountID,
			Status:               conv.Status,
			UnreadCount:          conv.UnreadCount,
			CreatedAt:            conv.CreatedAt.Format(timeLayout),
			UpdatedAt:            conv.UpdatedAt.Format(timeLayout),
		}

		if conv.LastInboundAt != nil {
			resp.LastInboundAt = formatTime(conv.LastInboundAt)
		}

		if contact, err := h.contacts.GetByID(ctx, conv.ContactID); err == nil {
			resp.Contact = &ContactSummary{
				ID:        contact.ID,
				Name:      contact.Name,
				Phone:     contact.Phone,
				AvatarURL: contact.AvatarURL,
			}
		}

		if msgs, err := h.messages.ListByConversation(ctx, conv.ID, 1, 0); err == nil && len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			resp.LastMessage = &MessageSummary{
				Text:      lastMsg.Text,
				Direction: lastMsg.Direction,
			}
			if lastMsg.SentAt != nil {
				resp.LastMessage.SentAt = formatTime(lastMsg.SentAt)
			}
		}

		if conv.AssignedTo.Valid {
			if name, ok := names[conv.AssignedTo.UUID]; ok {
				resp.AssignedTo = &AssigneeSummary{ID: conv.AssignedTo.UUID, Name: name}
			}
		}

		results = append(results, resp)
	}

	return results, nil
}

// ListConversations returns all conversations for the authenticated user.
// GET /api/inbox/conversations?channel=whatsapp&status=active&limit=30&offset=0
func (h *InboxHandler) ListConversations(c echo.Context) error {
	channel := c.QueryParam("channel")
	status := c.QueryParam("status")

	limit := 30
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	conversations, total, err := h.conversations.ListActive(c.Request().Context(), channel, status, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list conversations"})
	}

	results, err := h.enrichConversationResponses(c.Request().Context(), conversations)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list conversations"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": results,
		"meta": map[string]interface{}{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
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
		ID                   uuid.UUID        `json:"id"`
		Channel              string           `json:"channel"`
		Provider             string           `json:"provider"`
		ZernioConversationID string           `json:"zernio_conversation_id"`
		ZernioAccountID      *string          `json:"zernio_account_id,omitempty"`
		Status               string           `json:"status"`
		LastInboundAt        *string          `json:"last_inbound_at,omitempty"`
		UnreadCount          int              `json:"unread_count"`
		AssignedTo           *AssigneeSummary `json:"assigned_to"`
		CreatedAt            string           `json:"created_at"`
		UpdatedAt            string           `json:"updated_at"`
		Contact              ContactResponse  `json:"contact"`
		Messages             []MsgResponse    `json:"messages"`
	}

	resp := ConversationDetail{
		ID:                   conv.ID,
		Channel:              conv.Channel,
		Provider:             conv.Provider,
		ZernioConversationID: conv.ZernioConversationID,
		ZernioAccountID:      conv.ZernioAccountID,
		Status:               conv.Status,
		UnreadCount:          conv.UnreadCount,
		CreatedAt:            conv.CreatedAt.Format(timeLayout),
		UpdatedAt:            conv.UpdatedAt.Format(timeLayout),
		Contact: ContactResponse{
			ID:    contact.ID,
			Name:  contact.Name,
			Phone: contact.Phone,
			Email: contact.Email,
		},
	}

	if conv.AssignedTo.Valid {
		names, err := h.conversations.AssignedNames(c.Request().Context(), []uuid.UUID{conv.AssignedTo.UUID})
		if name, ok := names[conv.AssignedTo.UUID]; err == nil && ok {
			resp.AssignedTo = &AssigneeSummary{ID: conv.AssignedTo.UUID, Name: name}
		}
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
		resp.LastInboundAt = formatTime(conv.LastInboundAt)
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
			mr.SentAt = formatTime(msg.SentAt)
		}
		mr.CreatedAt = msg.CreatedAt.Format(timeLayout)
		resp.Messages = append(resp.Messages, mr)
	}

	return c.JSON(http.StatusOK, resp)
}

// UpdateConversation modifies conversation fields (status and/or assigned_to).
// PATCH /api/inbox/conversations/:id  body: {"status"?: "active"|"archived", "assigned_to"?: "<uuid>"|null}
func (h *InboxHandler) UpdateConversation(c echo.Context) error {
	ctx := c.Request().Context()
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid conversation id"})
	}

	var req struct {
		Status     *string          `json:"status"`
		AssignedTo *json.RawMessage `json:"assigned_to"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if _, err := h.conversations.GetByID(ctx, id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "conversación no encontrada"})
	}

	statusChanged := false
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "archived" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "status must be 'active' or 'archived'"})
		}
		if err := h.conversations.UpdateStatus(ctx, id, *req.Status); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update conversation"})
		}
		statusChanged = true
	}

	if req.AssignedTo != nil {
		raw := strings.TrimSpace(string(*req.AssignedTo))
		if raw == "null" {
			if err := h.conversations.UpdateAssignedTo(ctx, id, nil); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update conversation"})
			}
		} else {
			var assignedStr string
			if err := json.Unmarshal(*req.AssignedTo, &assignedStr); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "assigned_to inválido"})
			}
			userID, err := uuid.Parse(strings.TrimSpace(assignedStr))
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "assigned_to inválido"})
			}
			names, err := h.conversations.AssignedNames(ctx, []uuid.UUID{userID})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update conversation"})
			}
			if _, ok := names[userID]; !ok {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "el usuario asignado no existe"})
			}
			if err := h.conversations.UpdateAssignedTo(ctx, id, &userID); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update conversation"})
			}
		}
	}

	if !statusChanged && req.AssignedTo == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "nada que actualizar: enviá status y/o assigned_to"})
	}

	conv, err := h.conversations.GetByID(ctx, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update conversation"})
	}

	response := map[string]interface{}{
		"id":          conv.ID,
		"status":      conv.Status,
		"assigned_to": nil,
	}
	if conv.AssignedTo.Valid {
		names, err := h.conversations.AssignedNames(ctx, []uuid.UUID{conv.AssignedTo.UUID})
		if name, ok := names[conv.AssignedTo.UUID]; err == nil && ok {
			response["assigned_to"] = map[string]string{"id": conv.AssignedTo.UUID.String(), "name": name}
		}
	}

	return c.JSON(http.StatusOK, response)
}

// UnreadFeed returns the most recent active conversations with unread messages.
// GET /api/inbox/unread?limit=8
func (h *InboxHandler) UnreadFeed(c echo.Context) error {
	ctx := c.Request().Context()

	limit := 8
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 20 {
		limit = 20
	}

	conversations, err := h.conversations.UnreadConversations(ctx, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list unread conversations"})
	}

	total, err := h.conversations.UnreadSummary(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to sum unread conversations"})
	}

	items := make([]map[string]interface{}, 0, len(conversations))
	for _, conv := range conversations {
		name := "Desconocido"
		if ct, err := h.contacts.GetByID(ctx, conv.ContactID); err == nil {
			switch {
			case ct.Name != nil && strings.TrimSpace(*ct.Name) != "":
				name = *ct.Name
			case ct.Phone != nil && strings.TrimSpace(*ct.Phone) != "":
				name = *ct.Phone
			}
		}

		var preview *string
		if msgs, err := h.messages.ListByConversation(ctx, conv.ID, 1, 0); err == nil && len(msgs) > 0 {
			preview = msgs[len(msgs)-1].Text
		}

		item := map[string]interface{}{
			"id":              conv.ID,
			"channel":         conv.Channel,
			"contact_name":    name,
			"preview_text":    preview,
			"unread_count":    conv.UnreadCount,
			"last_inbound_at": nil,
		}
		if conv.LastInboundAt != nil {
			item["last_inbound_at"] = formatTime(conv.LastInboundAt)
		}

		items = append(items, item)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":  items,
		"total": total,
	})
}

// Search returns conversations matching a text query (contact name/phone or message content).
// GET /api/inbox/search?q=texto
func (h *InboxHandler) Search(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	if q == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "falta el parámetro q"})
	}

	limit := 10
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 20 {
		limit = 20
	}

	conversations, err := h.conversations.SearchConversations(c.Request().Context(), q, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to search conversations"})
	}

	results, err := h.enrichConversationResponses(c.Request().Context(), conversations)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to search conversations"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":  results,
		"count": len(results),
	})
}

// UpdateContactNotes actualiza las notas de un contacto.
// PATCH /api/inbox/contacts/:id
func (h *InboxHandler) UpdateContactNotes(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de contacto inválido"})
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo de la petición inválido"})
	}

	if _, err := h.contacts.GetByID(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "contacto no encontrado"})
	}

	// Empty string allowed = clear notes (store NULL)
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}

	if err := h.contacts.UpdateNotes(c.Request().Context(), id, notes); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al actualizar el contacto"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":    id,
		"notes": req.Notes,
	})
}
