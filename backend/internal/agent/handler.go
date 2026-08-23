package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/riont/crm/backend/internal/repository"
	"github.com/riont/crm/backend/internal/zernio"
)

// WebhookHandler processes incoming webhooks and invokes the agent after persistence.
type WebhookHandler struct {
	agent         *Agent
	messages      *repository.MessageRepository
	conversations *repository.ConversationRepository
	contacts      *repository.ContactRepository
	zernioClient  *zernio.Client
	pool          *pgxpool.Pool
}

func NewWebhookHandler(
	agent *Agent,
	messages *repository.MessageRepository,
	conversations *repository.ConversationRepository,
	contacts *repository.ContactRepository,
	zernioClient *zernio.Client,
	pool *pgxpool.Pool,
) *WebhookHandler {
	return &WebhookHandler{
		agent:         agent,
		messages:      messages,
		conversations: conversations,
		contacts:      contacts,
		zernioClient:  zernioClient,
		pool:          pool,
	}
}

// AfterMessageReceived is called AFTER a message.received webhook has been persisted.
// It invokes the AI agent if enabled for the channel, and sends the reply.
// This runs asynchronously — the webhook already returned 200.
func (h *WebhookHandler) AfterMessageReceived(ctx context.Context, conversationID, channel, contactName, messageText string) {
	// 1. Get conversation history. The incoming message was just persisted,
	// so drop its duplicate from history before passing it as the current turn.
	history, err := h.agent.GetHistory(ctx, conversationID, 21)
	if err != nil {
		fmt.Printf("agent: failed to get history: %v\n", err)
		return
	}
	if n := len(history); n > 0 {
		last := history[n-1]
		if last.Role == "user" && last.Content == messageText {
			history = history[:n-1]
		}
	}

	// 2. Call the agent
	resp, err := h.agent.ProcessMessage(ctx, AgentRequest{
		Channel:         channel,
		ContactName:     contactName,
		IncomingMessage: messageText,
		ConversationID:  conversationID,
		History:         history,
	})

	if err != nil {
		if strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
			fmt.Printf("agente IA sin configurar: falta OPENROUTER_API_KEY\n")
		} else {
			fmt.Printf("agente: error procesando mensaje: %v\n", err)
		}
		return
	}

	if resp == nil {
		// Agent disabled for this channel
		return
	}

	if resp.Reply == "" {
		return
	}

	// 3. Send reply via Zernio
	conv, err := h.conversations.FindByZernioID(ctx, channel, conversationID)
	if err != nil {
		fmt.Printf("agent: conversation not found for reply: %v\n", err)
		return
	}

	text := resp.Reply
	accountID := ""
	if conv.ZernioAccountID != nil {
		accountID = *conv.ZernioAccountID
	} else {
		fmt.Printf("agente: conversación sin zernio_account_id\n")
	}
	_, err = h.zernioClient.SendMessage(conversationID, zernio.SendMessageRequest{
		AccountID: accountID,
		Message:   &text,
	})
	if err != nil {
		fmt.Printf("agent: failed to send reply: %v\n", err)
		return
	}

	// 4. Persist the outgoing agent message
	now := time.Now()
	agentMsg := &repository.Message{
		ConversationID: conv.ID,
		ExternalID:     fmt.Sprintf("agent-%d", now.UnixNano()),
		Direction:      "outgoing",
		Text:           &resp.Reply,
		Attachments:    json.RawMessage("[]"),
		SenderType:     "system",
		Status:         "sent",
		Metadata:       json.RawMessage("{}"),
		SentAt:         &now,
	}

	if _, err := h.messages.InsertMessage(ctx, agentMsg); err != nil {
		fmt.Printf("agent: failed to persist reply: %v\n", err)
	}

	fmt.Printf("agent: replied on %s with %d tokens (model: %s)\n", channel, resp.TokensUsed, resp.UsedModel)
}
