package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
	AccountID        string   `json:"account_id"`
	Message          string   `json:"message"`
	AttachmentURL    *string  `json:"attachment_url,omitempty"`
	AttachmentType   *string  `json:"attachment_type,omitempty"`
	ReplyTo          string   `json:"reply_to,omitempty"`
	TemplateName     string   `json:"template_name,omitempty"`
	TemplateLanguage string   `json:"template_language,omitempty"`
	TemplateParams   []string `json:"template_params,omitempty"`
}

// whatsappServiceWindow is the duration a freeform WhatsApp message stays
// allowed after the last inbound message (Meta's 24h customer-service window).
const whatsappServiceWindow = 24 * time.Hour

func windowClosed(conv *repository.Conversation) bool {
	if conv.LastInboundAt == nil {
		return true
	}
	return time.Since(*conv.LastInboundAt) > whatsappServiceWindow
}

// ListWhatsAppTemplates returns the approved WABA templates for an account.
// GET /api/whatsapp/templates?account_id=
func (h *SendHandler) ListWhatsAppTemplates(c echo.Context) error {
	accountID := c.QueryParam("account_id")
	if accountID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "account_id is required"})
	}
	resp, err := h.zernioClient.ListWhatsAppTemplates(accountID)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("failed to list templates: %v", err)})
	}
	templates := make([]map[string]string, 0, len(resp.Templates))
	for _, t := range resp.Templates {
		templates = append(templates, map[string]string{
			"name":     t.Name,
			"language": t.Language,
			"status":   t.Status,
			"category": t.Category,
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"templates": templates})
}

// CreateWhatsAppTemplate creates a new WhatsApp message template in WABA.
// POST /api/whatsapp/templates
func (h *SendHandler) CreateWhatsAppTemplate(c echo.Context) error {
	var req struct {
		AccountID string `json:"account_id"`
		Name      string `json:"name"`
		Category  string `json:"category"`
		Language  string `json:"language"`
		Content   string `json:"content"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.AccountID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "account_id is required"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "El nombre es obligatorio"})
	}
	if req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "El contenido es obligatorio"})
	}
	category := req.Category
	if category == "" {
		category = "UTILITY"
	}
	category = normalizeTemplateCategory(category)
	language := req.Language
	if language == "" {
		language = "es"
	}

	// Enviar a Zernio con un solo componente BODY (plantilla POSITIONAL).
	// Nota: Zernio espera el tipo del componente en minúsculas como discriminator.
	tplReq := zernio.CreateWhatsAppTemplateRequest{
		AccountID:       req.AccountID,
		Name:            req.Name,
		Category:        category,
		Language:        language,
		ParameterFormat: "POSITIONAL",
		Components: []zernio.WhatsAppTemplateComponent{
			{Type: "body", Text: req.Content},
		},
	}
	resp, err := h.zernioClient.CreateWhatsAppTemplate(tplReq)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("failed to create template: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  resp.Success,
		"template": resp.Template,
	})
}

// normalizeTemplateCategory maps UI-friendly categories to the WABA enum.
func normalizeTemplateCategory(cat string) string {
	switch cat {
	case "marketing":
		return "MARKETING"
	case "authentication", "auth":
		return "AUTHENTICATION"
	default:
		return "UTILITY"
	}
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

	if req.AccountID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "account_id is required"})
	}
	isTemplate := req.TemplateName != ""
	if !isTemplate && req.Message == "" && req.AttachmentURL == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message, attachment o plantilla es requerido"})
	}

	// Get the conversation to find the Zernio conversation ID
	conv, err := h.conversations.GetByID(c.Request().Context(), convID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "conversation not found"})
	}

	// Fuera de la ventana de 24h de WhatsApp, los mensajes libres NO se entregan
	// (dan wamid pero el destinatario nunca los recibe). Sólo se aceptan plantillas.
	if conv.Channel == "whatsapp" && windowClosed(conv) && !isTemplate {
		closed := ""
		if conv.LastInboundAt != nil {
			closed = conv.LastInboundAt.Format(time.RFC3339)
		}
		return c.JSON(http.StatusBadRequest, map[string]map[string]interface{}{
			"error": {
				"code":          "WINDOW_CLOSED",
				"message":       "La ventana de 24h de WhatsApp está vencida. Solo podés responder con una plantilla.",
				"lastInboundAt": closed,
			},
		})
	}

	if isTemplate {
		// Enviar plantilla aprobada fuera de la ventana = POST /v1/inbox/conversations,
		// que requiere el teléfono del destinatario como participantId. Resolvemos el
		// teléfono desde la conversación de Zernio (su participantId es el número).
		participantID := ""
		if zernioConv, err := h.zernioClient.GetConversation(conv.ZernioConversationID, req.AccountID); err == nil {
			participantID = zernioConv.ParticipantID
		}
		if participantID == "" {
			if contact, err := h.contacts.GetByID(c.Request().Context(), conv.ContactID); err == nil && contact.Phone != nil {
				participantID = *contact.Phone
			}
		}
		if participantID == "" {
			return c.JSON(http.StatusBadRequest, map[string]map[string]interface{}{
				"error": {
					"code":    "PHONE_REQUIRED",
					"message": "No se pudo resolver el teléfono del contacto para enviar la plantilla.",
				},
			})
		}
		tplReq := zernio.SendConversationTemplateRequest{
			AccountID:        req.AccountID,
			ParticipantID:    participantID,
			TemplateName:     req.TemplateName,
			TemplateLanguage: req.TemplateLanguage,
			TemplateParams:   req.TemplateParams,
		}
		tplResp, err := h.zernioClient.SendConversationTemplate(tplReq)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to send template: %v", err)})
		}

		// Persistir el mensaje saliente de plantilla.
		externalID := tplResp.Data.MessageID
		if externalID == "" {
			externalID = uuid.NewString()
		}
		text := fmt.Sprintf("[Plantilla %s]", req.TemplateName)
		metadata, _ := json.Marshal(map[string]interface{}{
			"template":      true,
			"template_name": req.TemplateName,
		})
		newMsg := &repository.Message{
			ConversationID:    convID,
			ExternalID:        externalID,
			Direction:         "outgoing",
			Text:              &text,
			SenderType:        "agent",
			PlatformMessageID: &externalID,
			Status:            "sent",
			Metadata:          metadata,
		}
		savedMsg, err := h.messages.InsertMessage(c.Request().Context(), newMsg)
		if err != nil {
			fmt.Printf("warning: template sent but failed to persist: %v\n", err)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success":    true,
				"message_id": externalID,
				"persisted":  false,
				"template":   true,
			})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    true,
			"message_id": externalID,
			"message":    savedMsg,
			"template":   true,
		})
	}

	// Mensaje libre (dentro de la ventana de 24h)
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

	zernioRespMsg, err := h.zernioClient.SendMessage(conv.ZernioConversationID, zernioReq)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to send message: %v", err)})
	}

	// Persist the outgoing message
	attachmentsJSON, _ := json.Marshal(zernioRespMsg.Data.Attachments)
	platformMsgID := zernioRespMsg.Data.MessageID

	newMsg := &repository.Message{
		ConversationID:    convID,
		ExternalID:        zernioRespMsg.Data.MessageID,
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
			"success":    true,
			"message_id": zernioRespMsg.Data.MessageID,
			"persisted":  false,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"message_id": zernioRespMsg.Data.MessageID,
		"message":    savedMsg,
	})
}
