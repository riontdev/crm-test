package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

const reportsMaxRangeDays = 92

type ReportsHandler struct {
	pool *pgxpool.Pool
}

func NewReportsHandler(pool *pgxpool.Pool) *ReportsHandler {
	return &ReportsHandler{pool: pool}
}

type reportDailyRow struct {
	Date     string `json:"date"`
	Channel  string `json:"channel"`
	Incoming int64  `json:"incoming"`
	Outgoing int64  `json:"outgoing"`
}

type reportChannelTotal struct {
	Channel       string `json:"channel"`
	Incoming      int64  `json:"incoming"`
	Outgoing      int64  `json:"outgoing"`
	Conversations int64  `json:"conversations"`
}

type reportResponseTimes struct {
	AvgSeconds *int64 `json:"avg_seconds"`
	MinSeconds *int64 `json:"min_seconds"`
	MaxSeconds *int64 `json:"max_seconds"`
}

type reportsResponse struct {
	From            string               `json:"from"`
	To              string               `json:"to"`
	Daily           []reportDailyRow     `json:"daily"`
	TotalsByChannel []reportChannelTotal `json:"totals_by_channel"`
	ResponseTimes   reportResponseTimes  `json:"response_times"`
}

// Report devuelve métricas agregadas por día/canal para el rango pedido.
// GET /api/stats/reports?from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *ReportsHandler) Report(c echo.Context) error {
	if h.pool == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}
	ctx := c.Request().Context()

	today := time.Now().Truncate(24 * time.Hour)
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")
	if fromStr == "" && toStr == "" {
		fromStr = today.AddDate(0, 0, -29).Format("2006-01-02")
		toStr = today.Format("2006-01-02")
	}

	var from, to time.Time
	var err error
	if from, err = time.Parse("2006-01-02", fromStr); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "fechas inválidas"})
	}
	if to, err = time.Parse("2006-01-02", toStr); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "fechas inválidas"})
	}
	if from.After(to) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "fechas inválidas"})
	}
	days := int(to.Sub(from).Hours()/24) + 1
	if days > reportsMaxRangeDays {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "rango máximo 92 días"})
	}

	resp := reportsResponse{
		From:            fromStr,
		To:              toStr,
		Daily:           []reportDailyRow{},
		TotalsByChannel: []reportChannelTotal{},
	}

	// Serie diaria por canal (JOIN con conversations: el canal vive ahí).
	msgByDateChannel := map[string]map[string][2]int64{}
	rows, err := h.pool.Query(ctx,
		`SELECT to_char(date_trunc('day', m.created_at), 'YYYY-MM-DD') AS d,
			cv.channel,
			COUNT(m.id) FILTER (WHERE m.direction = 'incoming'),
			COUNT(m.id) FILTER (WHERE m.direction = 'outgoing')
		 FROM messages m
		 JOIN conversations cv ON cv.id = m.conversation_id
		 WHERE m.created_at >= $1::timestamptz AND m.created_at < ($2::date + interval '1 day')
		 GROUP BY 1, 2
		 ORDER BY 1, 2`, fromStr, toStr)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
	}
	for rows.Next() {
		var d, ch string
		var inCnt, outCnt int64
		if err := rows.Scan(&d, &ch, &inCnt, &outCnt); err != nil {
			rows.Close()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
		}
		if msgByDateChannel[d] == nil {
			msgByDateChannel[d] = map[string][2]int64{}
		}
		msgByDateChannel[d][ch] = [2]int64{inCnt, outCnt}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
	}
	rows.Close()

	// Zero-fill: todas las combinaciones día×canal para que los gráficos alineen.
	for i := 0; i < days; i++ {
		day := from.AddDate(0, 0, i).Format("2006-01-02")
		for _, ch := range channelOrder {
			counts := msgByDateChannel[day][ch]
			resp.Daily = append(resp.Daily, reportDailyRow{
				Date:     day,
				Channel:  ch,
				Incoming: counts[0],
				Outgoing: counts[1],
			})
		}
	}

	// Totales por canal en el rango (+ conversaciones creadas en el rango).
	inByChannel := map[string]int64{}
	outByChannel := map[string]int64{}
	rows, err = h.pool.Query(ctx,
		`SELECT cv.channel,
			COUNT(m.id) FILTER (WHERE m.direction = 'incoming'),
			COUNT(m.id) FILTER (WHERE m.direction = 'outgoing')
		 FROM conversations cv
		 LEFT JOIN messages m ON m.conversation_id = cv.id
		 	AND m.created_at >= $1::timestamptz AND m.created_at < ($2::date + interval '1 day')
		 GROUP BY cv.channel`, fromStr, toStr)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
	}
	for rows.Next() {
		var ch string
		var inCnt, outCnt int64
		if err := rows.Scan(&ch, &inCnt, &outCnt); err != nil {
			rows.Close()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
		}
		inByChannel[ch] = inCnt
		outByChannel[ch] = outCnt
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
	}
	rows.Close()

	convsByChannel := map[string]int64{}
	rows, err = h.pool.Query(ctx,
		`SELECT channel, COUNT(*)
		 FROM conversations
		 WHERE created_at >= $1::timestamptz AND created_at < ($2::date + interval '1 day')
		 GROUP BY channel`, fromStr, toStr)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
	}
	defer rows.Close()
	for rows.Next() {
		var ch string
		var cnt int64
		if err := rows.Scan(&ch, &cnt); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
		}
		convsByChannel[ch] = cnt
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
	}

	emitted := map[string]bool{}
	appendTotals := func(ch string) {
		if emitted[ch] {
			return
		}
		emitted[ch] = true
		resp.TotalsByChannel = append(resp.TotalsByChannel, reportChannelTotal{
			Channel:       ch,
			Incoming:      inByChannel[ch],
			Outgoing:      outByChannel[ch],
			Conversations: convsByChannel[ch],
		})
	}
	for _, ch := range channelOrder {
		appendTotals(ch)
	}
	extras := make([]string, 0)
	for ch := range inByChannel {
		if !emitted[ch] {
			extras = append(extras, ch)
		}
	}
	for ch := range convsByChannel {
		if !emitted[ch] {
			extras = append(extras, ch)
		}
	}
	sort.Strings(extras)
	for _, ch := range extras {
		appendTotals(ch)
	}

	// Tiempos de primera respuesta: primer incoming vs siguiente outgoing
	// más cercano en la misma conversación (MIN/AVG/MAX del rango).
	var avgSecs, minSecs, maxSecs *int64
	if err := h.pool.QueryRow(ctx,
		`WITH fi AS (
			SELECT conversation_id, MIN(sent_at) AS first_in
			FROM messages
			WHERE direction = 'incoming'
				AND sent_at IS NOT NULL
				AND sent_at >= $1::timestamptz AND sent_at < ($2::date + interval '1 day')
			GROUP BY conversation_id
		 ),
		 deltas AS (
			SELECT EXTRACT(EPOCH FROM (fo.first_out - fi.first_in))::bigint AS secs
			FROM fi
			CROSS JOIN LATERAL (
				SELECT MIN(m.sent_at) AS first_out
				FROM messages m
				WHERE m.conversation_id = fi.conversation_id AND m.direction = 'outgoing' AND m.sent_at > fi.first_in
			) fo
			WHERE fo.first_out IS NOT NULL
		 )
		 SELECT AVG(secs)::bigint, MIN(secs), MAX(secs) FROM deltas`, fromStr, toStr,
	).Scan(&avgSecs, &minSecs, &maxSecs); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al calcular los reportes"})
	}
	resp.ResponseTimes = reportResponseTimes{
		AvgSeconds: avgSecs,
		MinSeconds: minSecs,
		MaxSeconds: maxSecs,
	}

	return c.JSON(http.StatusOK, resp)
}
