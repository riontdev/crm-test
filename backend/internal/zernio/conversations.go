package zernio

import (
	"fmt"
	"net/http"
	"net/url"
)

// ListConversations fetches conversations from all connected messaging accounts.
// GET /v1/inbox/conversations
func (c *Client) ListConversations(platform string, limit int, cursor string) (*ListConversationsResponse, error) {
	u := fmt.Sprintf("%s/inbox/conversations", c.baseURL)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	if platform != "" {
		q.Set("platform", platform)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	req.URL.RawQuery = q.Encode()

	var resp ListConversationsResponse
	if err := c.Do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetConversation fetches a single conversation by ID.
// GET /v1/inbox/conversations/{conversationId}
func (c *Client) GetConversation(conversationID, accountID string) (*ConversationData, error) {
	u := fmt.Sprintf("%s/inbox/conversations/%s", c.baseURL, url.PathEscape(conversationID))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if accountID != "" {
		q := req.URL.Query()
		q.Set("accountId", accountID)
		req.URL.RawQuery = q.Encode()
	}

	var resp struct {
		Data ConversationData `json:"data"`
	}
	if err := c.Do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

// MarkConversationRead marks a conversation as read.
// POST /v1/inbox/conversations/{conversationId}/read
func (c *Client) MarkConversationRead(conversationID string) error {
	u := fmt.Sprintf("%s/inbox/conversations/%s/read", c.baseURL, url.PathEscape(conversationID))
	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	return c.Do(req, nil)
}
