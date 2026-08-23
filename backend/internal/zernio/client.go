package zernio

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://zernio.com/api/v1"
	defaultTimeout = 10 * time.Second
)

// Client is the Zernio API HTTP client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a new Zernio API client.
func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
	}
}

// Do sends an authenticated HTTP request and decodes the JSON response into v.
func (c *Client) Do(req *http.Request, v interface{}) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("zernio request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("zernio API error (status %d): %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// VerifyWebhookSignature verifies the HMAC-SHA256 signature of a webhook payload.
// It compares the raw body against the X-Zernio-Signature header.
// If no secret is configured, it rejects everything (rule: without secret, reject all).
func VerifyWebhookSignature(secret string, body []byte, signature string) error {
	if secret == "" {
		return fmt.Errorf("no webhook secret configured: rejecting all unsigned webhooks")
	}

	if signature == "" {
		return fmt.Errorf("no signature provided in X-Zernio-Signature header")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("invalid webhook signature: expected %s, got %s", expected, signature)
	}

	return nil
}

// ParseWebhookPayload parses the raw body into a WebhookPayload envelope.
// It extracts the event type and timestamp without losing the raw bytes.
func ParseWebhookPayload(body []byte) (*WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}
	return &payload, nil
}
