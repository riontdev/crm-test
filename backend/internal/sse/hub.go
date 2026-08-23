package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
)

// Event represents an SSE event sent to clients.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Client represents a connected SSE client.
type Client struct {
	ID     string
	Ch     chan Event
	Filter map[string]string // optional filter, e.g. conversation_id
}

// Hub manages SSE client connections and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
	nextID  int
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

// Subscribe adds a new client and returns it.
func (h *Hub) Subscribe(filter map[string]string) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextID++
	c := &Client{
		ID:     fmt.Sprintf("client-%d", h.nextID),
		Ch:     make(chan Event, 16),
		Filter: filter,
	}
	h.clients[c.ID] = c
	return c
}

// Unsubscribe removes a client.
func (h *Hub) Unsubscribe(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c.ID]; ok {
		delete(h.clients, c.ID)
		close(c.Ch)
	}
}

// Broadcast sends an event to all matching clients.
func (h *Hub) Broadcast(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		// Check filter match
		if len(c.Filter) > 0 {
			match := true
			eventData, _ := json.Marshal(event.Data)
			var dataMap map[string]interface{}
			json.Unmarshal(eventData, &dataMap)

			for k, v := range c.Filter {
				if fmt.Sprintf("%v", dataMap[k]) != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// Non-blocking send
		select {
		case c.Ch <- event:
		default:
			// Client too slow, drop event
		}
	}
}

// ServeHTTP handles the SSE endpoint for a single client.
func (h *Hub) ServeHTTP(c echo.Context) error {
	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
	}

	// Parse optional query filters
	filter := make(map[string]string)
	if convID := c.QueryParam("conversation_id"); convID != "" {
		filter["conversation_id"] = convID
	}

	client := h.Subscribe(filter)
	defer h.Unsubscribe(client)

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Accel-Buffering", "no")

	// Send initial connected event
	fmt.Fprintf(c.Response().Writer, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-client.Ch:
			if !ok {
				return nil
			}
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(c.Response().Writer, "event: %s\ndata: %s\n\n", event.Type, string(data))
			flusher.Flush()
		}
	}
}
