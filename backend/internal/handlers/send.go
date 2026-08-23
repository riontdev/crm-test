package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/riont/crm/backend/internal/repository"
	"github.com/riont/crm/backend/internal/zernio"
)

type SendHandler struct {
	messages      *repository.MessageRepository
	conversations *repository.ConversationRepository
	contacts      *repository.ContactRepository
	zernioClient  *zernio.Client
}

func NewSendHandler(
	messages *repository.MessageRepository,
	conversations *repository.ConversationRepository,
	contacts *repository.ContactRepository,
	zernioClient *zernio.Client,
) *SendHandler {
	return &SendHandler{
		messages:      messages,
		conversations: conversations,
		contacts:      contacts,
		zernioClient:  zernioClient,
	}
}

// SendMessageRequest is the body for POST /api/inbox/conversations/:id/messages
type SendMessageRequest struct {
	AccountID      string  `json:"account_id"`
	Message        string  `json:"message"`
	AttachmentURL  *string `json:"attachment_url,omitempty"`
	AttachmentType *string `json:"attachment_type,omitempty"`
	ReplyTo        string  `json:"reply_to,omitempty"`
}

// SendMessage sends a reply to a conversation.
// POST /api/inbox/conversations/:id/messages
func (h *SendHandler) SendMessage(c echo.Context) error {
	convIDStr := c.Param("id")
	convID, err := uuid.Parse(convIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid conversation id"})
	}

	var req SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message is required"})
	}
	if req.AccountID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "account_id is required"})
	}

	// Get the conversation to find the Zernio conversation ID
	conv, err := h.conversations.GetByID(c.Request().Context(), convID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "conversation not found"})
	}

	// Send via Zernio API
	text := req.Message
	zernioReq := zernio.SendMessageRequest{
		AccountID:      req.AccountID,
		Message:        &text,
		AttachmentURL:  req.AttachmentURL,
		AttachmentType: req.AttachmentType,
	}
	if req.ReplyTo != "" {
		zernioReq.ReplyTo = &req.ReplyTo
	}

	zernioResp, err := h.zernioClient.SendMessage(conv.ZernioConversationID, zernioReq)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to send message: %v", err)})
	}

	// Persist the outgoing message
	attachmentsJSON, _ := json.Marshal(zernioResp.Data.Attachments)
	platformMsgID := zernioResp.Data.MessageID

	newMsg := &repository.Message{
		ConversationID:    convID,
		ExternalID:        zernioResp.Data.MessageID,
		Direction:         "outgoing",
		Text:              &text,
		Attachments:       attachmentsJSON,
		SenderType:        "agent",
		PlatformMessageID: &platformMsgID,
		Status:            "sent",
		Metadata:          json.RawMessage("{}"),
	}

	savedMsg, err := h.messages.InsertMessage(c.Request().Context(), newMsg)
	if err != nil {
		// Message was sent to Zernio but failed to persist
		// Log error but still return success to frontend
		fmt.Printf("warning: message sent to Zernio but failed to persist: %v\n", err)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":     true,
			"message_id":  zernioResp.Data.MessageID,
			"persisted":   false,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"message_id": zernioResp.Data.MessageID,
		"message":    savedMsg,
	})
}
