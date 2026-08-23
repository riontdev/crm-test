package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/riont/crm/backend/internal/repository"
	"github.com/riont/crm/backend/internal/sse"
	"github.com/riont/crm/backend/internal/zernio"
)

// AfterMessageFunc is called after a message.received is persisted.
type AfterMessageFunc func(ctx context.Context, conversationID, channel, contactName, messageText string)

type WebhookHandler struct {
	webhookEvents *repository.WebhookEventRepository
	contacts      *repository.ContactRepository
	identities    *repository.ContactIdentityRepository
	conversations *repository.ConversationRepository
	messages      *repository.MessageRepository
	zernioClient  *zernio.Client
	webhookSecret string
	afterMessage  AfterMessageFunc
	sseHub        *sse.Hub
}

func NewWebhookHandler(
	webhookEvents *repository.WebhookEventRepository,
	contacts *repository.ContactRepository,
	identities *repository.ContactIdentityRepository,
	conversations *repository.ConversationRepository,
	messages *repository.MessageRepository,
	zernioClient *zernio.Client,
	webhookSecret string,
	sseHub *sse.Hub,
) *WebhookHandler {
	return &WebhookHandler{
		webhookEvents: webhookEvents,
		contacts:      contacts,
		identities:    identities,
		conversations: conversations,
		messages:      messages,
		zernioClient:  zernioClient,
		webhookSecret: webhookSecret,
		sseHub:        sseHub,
	}
}

// SetAfterMessage sets the callback to run after a message is persisted.
func (h *WebhookHandler) SetAfterMessage(fn AfterMessageFunc) {
	h.afterMessage = fn
}

func (h *WebhookHandler) HandleWebhook(c echo.Context) error {
	rawBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read body"})
	}
	defer c.Request().Body.Close()

	signature := c.Request().Header.Get("X-Zernio-Signature")
	if h.webhookSecret != "" {
		if err := zernio.VerifyWebhookSignature(h.webhookSecret, rawBody, signature); err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
		}
	}

	var envelope zernio.WebhookPayload
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
	}

	claimed, err := h.webhookEvents.ClaimEvent(c.Request().Context(), envelope.ID, envelope.Event, rawBody)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to claim event"})
	}
	if !claimed {
		return c.JSON(http.StatusOK, map[string]string{"status": "already processed"})
	}

	ctx := c.Request().Context()

	switch envelope.Event {
	case "message.received":
		if err := h.handleMessageReceived(ctx, rawBody); err != nil {
			fmt.Printf("error processing message.received: %v\n", err)
		}
	case "conversation.started":
		if err := h.handleConversationStarted(ctx, rawBody); err != nil {
			fmt.Printf("error processing conversation.started: %v\n", err)
		}
	case "message.delivered", "message.read", "message.failed":
		if err := h.handleMessageStatus(ctx, rawBody, envelope.Event); err != nil {
			fmt.Printf("error processing %s: %v\n", envelope.Event, err)
		}
	}

	h.webhookEvents.MarkProcessed(ctx, envelope.ID)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *WebhookHandler) handleMessageReceived(ctx context.Context, rawBody []byte) error {
	var payload zernio.WebhookMessageReceived
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return fmt.Errorf("failed to parse message.received payload: %w", err)
	}

	msg := payload.Message
	acct := payload.Account
	channel := acct.Platform

	externalID := resolveExternalID(channel, &msg.Sender)

	contact, err := h.contacts.FindOrCreateByExternalID(ctx, channel, externalID, msg.Sender.Name, msg.Sender.Picture)
	if err != nil {
		return fmt.Errorf("failed to find/create contact: %w", err)
	}

	_, err = h.identities.FindOrCreate(ctx, contact.ID, channel, "zernio", externalID,
		msg.Sender.Username, msg.Sender.Name, msg.Sender.Picture)
	if err != nil {
		fmt.Printf("warning: failed to create identity: %v\n", err)
	}

	conv, err := h.conversations.FindOrCreate(ctx, contact.ID, channel, "zernio",
		payload.Conversation.ID, &acct.AccountID, &payload.Conversation.PlatformConversationID)
	if err != nil {
		return fmt.Errorf("failed to find/create conversation: %w", err)
	}

	attachmentsJSON, _ := json.Marshal(msg.Attachments)
	senderType := "contact"

	platformMsgID := msg.PlatformMessageID

	newMsg := &repository.Message{
		ConversationID:    conv.ID,
		ExternalID:        msg.ID,
		Direction:         msg.Direction,
		Text:              msg.Text,
		Attachments:       attachmentsJSON,
		SenderType:        senderType,
		SenderContactID:   &contact.ID,
		PlatformMessageID: &platformMsgID,
		Status:            "sent",
		Metadata:          json.RawMessage("{}"),
		SentAt:            &msg.SentAt,
	}

	if _, err := h.messages.InsertMessage(ctx, newMsg); err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	if err := h.conversations.UpdateLastInboundAt(ctx, conv.ID, msg.SentAt); err != nil {
		fmt.Printf("warning: failed to update last_inbound_at: %v\n", err)
	}

	h.conversations.IncrementUnread(ctx, conv.ID)

	// Broadcast new message via SSE
	if h.sseHub != nil {
		contactName := ""
		if msg.Sender.Name != nil {
			contactName = *msg.Sender.Name
		}
		msgText := ""
		if msg.Text != nil {
			msgText = *msg.Text
		}
		h.sseHub.Broadcast(sse.Event{
			Type: "message.received",
			Data: map[string]interface{}{
				"conversation_id": payload.Conversation.ID,
				"channel":         channel,
				"contact_name":    contactName,
				"text":            msgText,
				"direction":       msg.Direction,
				"sent_at":         msg.SentAt,
				"unread_count":    1,
			},
		})
	}

	// Invoke after-message hook (agent, notifications, etc.)
	if h.afterMessage != nil {
		contactName := ""
		if msg.Sender.Name != nil {
			contactName = *msg.Sender.Name
		}
		msgText := ""
		if msg.Text != nil {
			msgText = *msg.Text
		}
		go h.afterMessage(ctx, payload.Conversation.ID, channel, contactName, msgText)
	}

	return nil
}

func (h *WebhookHandler) handleConversationStarted(ctx context.Context, rawBody []byte) error {
	var payload zernio.WebhookConversationStarted
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return fmt.Errorf("failed to parse conversation.started payload: %w", err)
	}

	acct := payload.Account
	conv := payload.Conversation
	channel := acct.Platform

	var externalID string
	if conv.ParticipantID != nil {
		externalID = *conv.ParticipantID
	} else {
		return fmt.Errorf("no participant_id in conversation.started")
	}

	contact, err := h.contacts.FindOrCreateByExternalID(ctx, channel, externalID, conv.ParticipantName, conv.ParticipantPicture)
	if err != nil {
		return fmt.Errorf("failed to find/create contact: %w", err)
	}

	_, err = h.identities.FindOrCreate(ctx, contact.ID, channel, "zernio", externalID,
		conv.ParticipantUsername, conv.ParticipantName, conv.ParticipantPicture)
	if err != nil {
		fmt.Printf("warning: failed to create identity: %v\n", err)
	}

	_, err = h.conversations.FindOrCreate(ctx, contact.ID, channel, "zernio",
		conv.ID, &acct.AccountID, &conv.PlatformConversationID)
	if err != nil {
		return fmt.Errorf("failed to find/create conversation: %w", err)
	}

	return nil
}

func (h *WebhookHandler) handleMessageStatus(ctx context.Context, rawBody []byte, eventType string) error {
	var payload struct {
		ID        string                    `json:"id"`
		Event     string                    `json:"event"`
		Message   zernio.InboxWebhookMessage `json:"message"`
		StatusAt  time.Time                 `json:"statusAt"`
	}

	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return fmt.Errorf("failed to parse %s payload: %w", eventType, err)
	}

	var status string
	switch eventType {
	case "message.delivered":
		status = "delivered"
	case "message.read":
		status = "read"
	case "message.failed":
		status = "failed"
	}

	msg, err := h.messages.FindByExternalID(ctx, payload.Message.ID)
	if err != nil {
		return fmt.Errorf("message not found for status update: %w", err)
	}

	_, err = h.messages.UpdateStatus(ctx, msg.ID, status)
	if err != nil {
		return fmt.Errorf("failed to update message status: %w", err)
	}

	return nil
}

func resolveExternalID(channel string, sender *zernio.MessageSender) string {
	switch channel {
	case "whatsapp":
		if sender.BusinessScopedUserID != nil && *sender.BusinessScopedUserID != "" {
			return *sender.BusinessScopedUserID
		}
		if sender.PhoneNumber != nil && *sender.PhoneNumber != "" {
			return *sender.PhoneNumber
		}
		return sender.ID
	default:
		return sender.ID
	}
}
