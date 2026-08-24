package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

var channelOrder = []string{"whatsapp", "instagram", "facebook"}

type ChannelsHandler struct {
	pool *pgxpool.Pool
}

func NewChannelsHandler(pool *pgxpool.Pool) *ChannelsHandler {
	return &ChannelsHandler{pool: pool}
}

type channelStatus struct {
	Channel            string     `json:"channel"`
	Connected          bool       `json:"connected"`
	ConversationsCount int64      `json:"conversations_count"`
	MessagesCount      int64      `json:"messages_count"`
	LastActivityAt     *time.Time `json:"last_activity_at"`
	AgentEnabled       bool       `json:"agent_enabled"`
}

type channelsStatusResponse struct {
	Channels   []channelStatus `json:"channels"`
	WebhookURL string          `json:"webhook_url"`
}

// Status devuelve el estado de cada canal: actividad registrada y agente IA.
// GET /api/channels/status
func (h *ChannelsHandler) Status(c echo.Context) error {
	if h.pool == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	ctx := c.Request().Context()

	convsByChannel := map[string]int64{}
	rows, err := h.pool.Query(ctx,
		`SELECT channel, COUNT(*) FROM conversations GROUP BY channel`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al consultar conversaciones"})
	}
	for rows.Next() {
		var ch string
		var cnt int64
		if err := rows.Scan(&ch, &cnt); err != nil {
			rows.Close()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer conversaciones"})
		}
		convsByChannel[ch] = cnt
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer conversaciones"})
	}
	rows.Close()

	msgsByChannel := map[string]int64{}
	lastActByChannel := map[string]time.Time{}
	rows, err = h.pool.Query(ctx,
		`SELECT cv.channel, COUNT(*), MAX(COALESCE(m.sent_at, m.created_at))
		 FROM messages m
		 JOIN conversations cv ON cv.id = m.conversation_id
		 GROUP BY cv.channel`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al consultar mensajes"})
	}
	for rows.Next() {
		var ch string
		var cnt int64
		var lastAct time.Time
		if err := rows.Scan(&ch, &cnt, &lastAct); err != nil {
			rows.Close()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer mensajes"})
		}
		msgsByChannel[ch] = cnt
		lastActByChannel[ch] = lastAct
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer mensajes"})
	}
	rows.Close()

	agentsByChannel := map[string]bool{}
	rows, err = h.pool.Query(ctx,
		`SELECT channel, enabled FROM agent_configs`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al consultar agentes"})
	}
	defer rows.Close()
	for rows.Next() {
		var ch string
		var enabled bool
		if err := rows.Scan(&ch, &enabled); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer agentes"})
		}
		agentsByChannel[ch] = enabled
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al leer agentes"})
	}

	resp := channelsStatusResponse{
		Channels:   make([]channelStatus, 0, len(channelOrder)),
		WebhookURL: webhookURL(c),
	}
	for _, ch := range channelOrder {
		lastAct := lastActByChannel[ch]
		cs := channelStatus{
			Channel:            ch,
			ConversationsCount: convsByChannel[ch],
			MessagesCount:      msgsByChannel[ch],
			LastActivityAt:     &lastAct,
			AgentEnabled:       agentsByChannel[ch],
		}
		cs.Connected = cs.ConversationsCount > 0
		resp.Channels = append(resp.Channels, cs)
	}

	return c.JSON(http.StatusOK, resp)
}

func webhookURL(c echo.Context) string {
	if public := os.Getenv("PUBLIC_WEBHOOK_URL"); public != "" {
		return public
	}
	// Detrás de proxies (Railway/Vercel) el Host del request es el host público.
	return "https://" + c.Request().Host + "/webhook/zernio"
}
