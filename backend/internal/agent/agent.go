package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConversationMessage represents a message in the conversation history for the LLM.
type ConversationMessage struct {
	Role      string  // "user" or "assistant"
	Content   string
	SentAt    string
	SenderName *string
}

// AgentRequest is the input to the agent.
type AgentRequest struct {
	Channel           string
	ContactName       string
	IncomingMessage   string
	ConversationID    string
	History           []ConversationMessage
}

// AgentResponse is the output from the agent.
type AgentResponse struct {
	Reply   string
	UsedModel string
	TokensUsed int
}

// Agent orchestrates the LLM call for a specific channel.
type Agent struct {
	configRepo   *ConfigRepository
	openrouter   *OpenRouterClient
	pool         *pgxpool.Pool
}

func NewAgent(configRepo *ConfigRepository, openrouter *OpenRouterClient, pool *pgxpool.Pool) *Agent {
	return &Agent{
		configRepo: configRepo,
		openrouter: openrouter,
		pool:       pool,
	}
}

// ProcessMessage handles an incoming message and generates an AI response.
// Returns nil response if agent is disabled for this channel.
func (a *Agent) ProcessMessage(ctx context.Context, req AgentRequest) (*AgentResponse, error) {
	// 1. Get agent config for this channel
	cfg, err := a.configRepo.GetByChannel(ctx, req.Channel)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}

	// 2. Check if agent is enabled for this channel
	if !cfg.Enabled {
		return nil, nil // agent disabled, no response
	}

	// 3. Build messages for LLM
	messages := a.buildMessages(cfg, req)

	// 4. Call OpenRouter
	completion, err := a.openrouter.ChatCompletion(ChatCompletionRequest{
		Model:       cfg.Model,
		Messages:    messages,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		Stream:      false,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	reply := completion.Choices[0].Message.Content
	reply = strings.TrimSpace(reply)

	return &AgentResponse{
		Reply:      reply,
		UsedModel:  cfg.Model,
		TokensUsed: completion.Usage.TotalTokens,
	}, nil
}

// buildMessages constructs the message array for the LLM.
func (a *Agent) buildMessages(cfg *AgentConfig, req AgentRequest) []ChatMessage {
	var messages []ChatMessage

	// System prompt
	systemPrompt := a.getSystemPrompt(cfg, req)
	messages = append(messages, ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Conversation history (last N messages for context)
	historyLimit := 20
	if len(req.History) > historyLimit {
		req.History = req.History[len(req.History)-historyLimit:]
	}

	for _, h := range req.History {
		role := "user"
		if h.Role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, ChatMessage{
			Role:    role,
			Content: h.Content,
		})
	}

	// Current incoming message
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: req.IncomingMessage,
	})

	return messages
}

// getSystemPrompt builds the system prompt with context variables.
func (a *Agent) getSystemPrompt(cfg *AgentConfig, req AgentRequest) string {
	basePrompt := "Eres un asistente de servicio al cliente."

	if cfg.SystemPrompt != nil && *cfg.SystemPrompt != "" {
		basePrompt = *cfg.SystemPrompt
	}

	// Replace context variables
	basePrompt = strings.ReplaceAll(basePrompt, "{{channel}}", req.Channel)
	basePrompt = strings.ReplaceAll(basePrompt, "{{contact_name}}", req.ContactName)
	basePrompt = strings.ReplaceAll(basePrompt, "{{conversation_id}}", req.ConversationID)

	return basePrompt
}

// GetHistory retrieves conversation history from the database.
func (a *Agent) GetHistory(ctx context.Context, conversationID string, limit int) ([]ConversationMessage, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT direction, text, sent_at
		 FROM messages
		 WHERE conversation_id = $1 AND text IS NOT NULL
		 ORDER BY created_at DESC
		 LIMIT $2`,
		conversationID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var history []ConversationMessage
	for rows.Next() {
		var msg ConversationMessage
		var sentAt string
		if err := rows.Scan(&msg.Role, &msg.Content, &sentAt); err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}
		// Convert direction to LLM role
		if msg.Role == "incoming" {
			msg.Role = "user"
		} else {
			msg.Role = "assistant"
		}
		msg.SentAt = sentAt
		history = append(history, msg)
	}

	// Reverse to chronological order
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	return history, nil
}
