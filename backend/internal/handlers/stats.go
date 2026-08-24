package handlers

import (
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Ventanas soportadas por el parámetro ?period=
var statsPeriodIntervals = map[string]string{
	"24h": "24 hours",
	"7d":  "7 days",
	"30d": "30 days",
}

// Cantidad de buckets diarios por período (24h cubre los últimos 2 días).
var statsPeriodDays = map[string]int{
	"24h": 2,
	"7d":  7,
	"30d": 30,
}

type StatsHandler struct {
	pool *pgxpool.Pool
}

func NewStatsHandler(pool *pgxpool.Pool) *StatsHandler {
	return &StatsHandler{pool: pool}
}

type statsCountDelta struct {
	Count    int64    `json:"count"`
	DeltaPct *float64 `json:"delta_pct"`
}

type statsConversationsTotal struct {
	Active      int64 `json:"active"`
	NewInPeriod int64 `json:"new_in_period"`
}

type statsUnreadTotal struct {
	Total int64 `json:"total"`
}

type statsAIReplies struct {
	Count      int64 `json:"count"`
	HumanCount int64 `json:"human_count"`
}

type statsFirstResponse struct {
	AvgSeconds *int64 `json:"avg_seconds"`
}

type statsTotals struct {
	Messages      statsCountDelta         `json:"messages"`
	Conversations statsConversationsTotal `json:"conversations"`
	Unread        statsUnreadTotal        `json:"unread"`
	AIReplies     statsAIReplies          `json:"ai_replies"`
	FirstResponse statsFirstResponse      `json:"first_response"`
}

type statsByChannel struct {
	Channel       string `json:"channel"`
	Messages      int64  `json:"messages"`
	Conversations int64  `json:"conversations"`
}

type statsDailyPoint struct {
	Date     string `json:"date"`
	Incoming int64  `json:"incoming"`
	Outgoing int64  `json:"outgoing"`
}

type statsOverviewResponse struct {
	Period      string            `json:"period"`
	Totals      statsTotals       `json:"totals"`
	ByChannel   []statsByChannel  `json:"by_channel"`
	DailySeries []statsDailyPoint `json:"daily_series"`
}

// Overview devuelve los KPIs del dashboard para la ventana elegida.
// GET /api/stats/overview?period=24h|7d|30d
func (h *StatsHandler) Overview(c echo.Context) error {
	if h.pool == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	ctx := c.Request().Context()

	period := c.QueryParam("period")
	if period == "" {
		period = "24h"
	}
	intervalText, ok := statsPeriodIntervals[period]
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "período inválido: debe ser '24h', '7d' o '30d'"})
	}
	windowDays := statsPeriodDays[period]

	resp := statsOverviewResponse{
		Period:      period,
		ByChannel:   []statsByChannel{},
		DailySeries: []statsDailyPoint{},
	}

	// Mensajes ventana actual vs anterior (para el delta %)
	var curMessages, prevMessages int64
	if err := h.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE created_at >= now() - $1::interval AND created_at < now()),
			COUNT(*) FILTER (WHERE created_at >= now() - $2::interval AND created_at < now() - $1::interval)
		 FROM messages`, intervalText, intervalText,
	).Scan(&curMessages, &prevMessages); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	if prevMessages > 0 {
		v := math.Round(((float64(curMessages)-float64(prevMessages))/float64(prevMessages))*1000) / 10
		resp.Totals.Messages = statsCountDelta{Count: curMessages, DeltaPct: &v}
	} else {
		resp.Totals.Messages = statsCountDelta{Count: curMessages}
	}

	// Conversaciones activas ahora y creadas en la ventana
	var activeConvos, newConvos int64
	if err := h.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE status = 'active'),
			COUNT(*) FILTER (WHERE created_at >= now() - $1::interval)
		 FROM conversations`, intervalText,
	).Scan(&activeConvos, &newConvos); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	resp.Totals.Conversations = statsConversationsTotal{Active: activeConvos, NewInPeriod: newConvos}

	// Unread acumulado de conversaciones activas
	var unreadTotal int64
	if err := h.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(unread_count), 0) FROM conversations WHERE status = 'active'`,
	).Scan(&unreadTotal); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	resp.Totals.Unread = statsUnreadTotal{Total: unreadTotal}

	// Respuestas IA vs humanas (salientes en la ventana)
	var aiCount, humanCount int64
	if err := h.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE sender_type = 'system'),
			COUNT(*) FILTER (WHERE sender_type = 'agent')
		 FROM messages
		 WHERE direction = 'outgoing' AND created_at >= now() - $1::interval`, intervalText,
	).Scan(&aiCount, &humanCount); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	resp.Totals.AIReplies = statsAIReplies{Count: aiCount, HumanCount: humanCount}

	// Primera respuesta promedio: primer incoming de cada conversación en la
	// ventana vs el siguiente outgoing más cercano en esa misma conversación.
	var avgSeconds *int64
	if err := h.pool.QueryRow(ctx,
		`WITH fi AS (
			SELECT conversation_id, MIN(sent_at) AS first_in
			FROM messages
			WHERE direction = 'incoming' AND sent_at IS NOT NULL AND sent_at >= now() - $1::interval
			GROUP BY conversation_id
		 )
		 SELECT AVG(EXTRACT(EPOCH FROM (fo.first_out - fi.first_in)))::bigint
		 FROM fi
		 CROSS JOIN LATERAL (
			SELECT MIN(m.sent_at) AS first_out
			FROM messages m
			WHERE m.conversation_id = fi.conversation_id AND m.direction = 'outgoing' AND m.sent_at > fi.first_in
		 ) fo`, intervalText,
	).Scan(&avgSeconds); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	resp.Totals.FirstResponse = statsFirstResponse{AvgSeconds: avgSeconds}

	// Mensajes por canal en la ventana
	msgByChannel := map[string]int64{}
	rows, err := h.pool.Query(ctx,
		`SELECT cv.channel, COUNT(m.id)
		 FROM conversations cv
		 JOIN messages m ON m.conversation_id = cv.id
		 WHERE m.created_at >= now() - $1::interval AND m.created_at < now()
		 GROUP BY cv.channel`, intervalText)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	for rows.Next() {
		var channel string
		var cnt int64
		if err := rows.Scan(&channel, &cnt); err != nil {
			rows.Close()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
		}
		msgByChannel[channel] = cnt
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	rows.Close()

	// Conversaciones nuevas por canal en la ventana
	convByChannel := map[string]int64{}
	rows, err = h.pool.Query(ctx,
		`SELECT channel, COUNT(*)
		 FROM conversations
		 WHERE created_at >= now() - $1::interval
		 GROUP BY channel`, intervalText)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	defer rows.Close()
	for rows.Next() {
		var channel string
		var cnt int64
		if err := rows.Scan(&channel, &cnt); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
		}
		convByChannel[channel] = cnt
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}

	// Merge de ambos mapas en un slice ordenado por mensajes desc
	channels := make([]string, 0, len(msgByChannel)+len(convByChannel))
	seen := map[string]bool{}
	for ch := range msgByChannel {
		if !seen[ch] {
			seen[ch] = true
			channels = append(channels, ch)
		}
	}
	for ch := range convByChannel {
		if !seen[ch] {
			seen[ch] = true
			channels = append(channels, ch)
		}
	}
	sort.Strings(channels)
	for _, ch := range channels {
		resp.ByChannel = append(resp.ByChannel, statsByChannel{
			Channel:       ch,
			Messages:      msgByChannel[ch],
			Conversations: convByChannel[ch],
		})
	}
	sort.SliceStable(resp.ByChannel, func(i, j int) bool {
		if resp.ByChannel[i].Messages != resp.ByChannel[j].Messages {
			return resp.ByChannel[i].Messages > resp.ByChannel[j].Messages
		}
		return resp.ByChannel[i].Channel < resp.ByChannel[j].Channel
	})

	// Serie diaria: buckets de 1 día desde el inicio truncado hasta ahora.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(windowDays - 1))
	rows, err = h.pool.Query(ctx,
		`SELECT to_char(gs::date, 'YYYY-MM-DD'),
			COUNT(m.id) FILTER (WHERE m.direction = 'incoming'),
			COUNT(m.id) FILTER (WHERE m.direction = 'outgoing')
		 FROM generate_series($1::timestamptz, now(), '1 day') gs
		 LEFT JOIN messages m ON m.created_at >= gs AND m.created_at < gs + interval '1 day'
		 GROUP BY gs ORDER BY gs`, start)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}
	defer rows.Close()
	for rows.Next() {
		var point statsDailyPoint
		if err := rows.Scan(&point.Date, &point.Incoming, &point.Outgoing); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
		}
		resp.DailySeries = append(resp.DailySeries, point)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular las estadísticas"})
	}

	return c.JSON(http.StatusOK, resp)
}
