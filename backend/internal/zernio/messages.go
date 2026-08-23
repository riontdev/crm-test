package zernio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// SendMessage sends a message in a conversation via the Zernio inbox API.
// POST /v1/inbox/conversations/{conversationId}/messages
func (c *Client) SendMessage(conversationID string, req SendMessageRequest) (*SendMessageResponse, error) {
	u := fmt.Sprintf("%s/inbox/conversations/%s/messages", c.baseURL, url.PathEscape(conversationID))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var resp SendMessageResponse
	if err := c.Do(httpReq, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ListMessages fetches messages for a specific conversation.
// GET /v1/inbox/conversations/{conversationId}/messages
func (c *Client) ListMessages(conversationID string, limit int, cursor string) (*ListMessagesResponse, error) {
	u := fmt.Sprintf("%s/inbox/conversations/%s/messages", c.baseURL, url.PathEscape(conversationID))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	req.URL.RawQuery = q.Encode()

	var resp ListMessagesResponse
	if err := c.Do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
