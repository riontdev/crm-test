package insight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository es la unica capa que habla con las tablas del analisis. Vive en
// el paquete insight (y no en repository/) porque TODAS estas tablas existen
// para esta funcionalidad: asi queda obvio que leerlas significa hacer
// analisis.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ---------------------------------------------------------------------------
// Tipos
// ---------------------------------------------------------------------------

// AnalysisRow es una fila de message_analyses (el historial por mensaje).
type AnalysisRow struct {
	ID             uuid.UUID       `db:"id" json:"id"`
	MessageID      uuid.UUID       `db:"message_id" json:"message_id"`
	ConversationID uuid.UUID       `db:"conversation_id" json:"conversation_id"`
	Scope          string          `db:"scope" json:"scope"`
	Intent         *string         `db:"intent" json:"intent"`
	Resumen        *string         `db:"resumen" json:"resumen"`
	Productos      []string        `db:"productos" json:"productos"`
	Cantidades     []int           `db:"cantidades" json:"cantidades"`
	TipoCantidad   *string         `db:"tipo_cantidad" json:"tipo_cantidad"`
	Detalles       map[string]any  `db:"detalles" json:"detalles"`
	Confianza      *float64        `db:"confianza" json:"confianza"`
	NeedsReview    bool            `db:"needs_review" json:"needs_review"`
	ASRText        *string         `db:"asr_text" json:"asr_text"`
	ASRMS          *int            `db:"asr_ms" json:"asr_ms"`
	ASRModel       *string         `db:"asr_model" json:"asr_model"`
	Model          *string         `db:"model" json:"model"`
	LatencyMS      *int            `db:"latency_ms" json:"latency_ms"`
	Raw            json.RawMessage `db:"raw" json:"raw"`
	Status         string          `db:"status" json:"status"`
	Error          *string         `db:"error" json:"error"`
	SkipReason     *string         `db:"skip_reason" json:"skip_reason"`
	CreatedAt      time.Time       `db:"created_at" json:"created_at"`
}

const analysisColumns = `id, message_id, conversation_id, scope, intent, resumen, productos,
	cantidades, tipo_cantidad, detalles, confianza, needs_review, asr_text, asr_ms, asr_model,
	model, latency_ms, raw, status, error, skip_reason, created_at`

func scanAnalysis(s pgx.Row) (*AnalysisRow, error) {
	var a AnalysisRow
	err := s.Scan(&a.ID, &a.MessageID, &a.ConversationID, &a.Scope, &a.Intent,
		&a.Resumen, &a.Productos, &a.Cantidades, &a.TipoCantidad, &a.Detalles, &a.Confianza,
		&a.NeedsReview, &a.ASRText, &a.ASRMS, &a.ASRModel, &a.Model, &a.LatencyMS,
		&a.Raw, &a.Status, &a.Error, &a.SkipReason, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// AnalysisResult agrupa todo lo que se persiste al cerrar un analisis.
type AnalysisResult struct {
	Analysis    *Analysis
	Detalles    map[string]string
	NeedsReview bool
	Model       string
	LatencyMS   int
	Raw         json.RawMessage
	ASRText     *string
	ASRMS       *int
	ASRModel    *string
}

// InsightConfig es insight_configs: la configuracion por canal.
type InsightConfig struct {
	Channel         string    `db:"channel" json:"channel"`
	Enabled         bool      `db:"enabled" json:"enabled"`
	ASREnabled      bool      `db:"asr_enabled" json:"asr_enabled"`
	Model           string    `db:"model" json:"model"`
	SystemPrompt    *string   `db:"system_prompt" json:"system_prompt"`
	Temperature     float64   `db:"temperature" json:"temperature"`
	ContextMessages int       `db:"context_messages" json:"context_messages"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// InsightConfigPatch son los campos editables desde la UI (parcial).
type InsightConfigPatch struct {
	Enabled         *bool
	ASREnabled      *bool
	Model           *string
	SystemPrompt    *string
	Temperature     *float64
	ContextMessages *int
}

// AnalysisCounts resume el estado de la cola para la UI y /status.
type AnalysisCounts struct {
	Total       int `db:"total" json:"total"`
	Pending     int `db:"pending" json:"pending"`
	Processing  int `db:"processing" json:"processing"`
	OK          int `db:"ok" json:"ok"`
	Error       int `db:"error" json:"error"`
	Skipped     int `db:"skipped" json:"skipped"`
	NeedsReview int `db:"needs_review" json:"needs_review"`
}

// nullableUUID convierte el uuid.Nil en un NULL de verdad para las queries que
// lo usan como filtro opcional.
func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

// RecoverItem es un mensaje que hay que (re)analizar. Text/Attachments vienen
// del join con messages para que el worker no tenga que volver a consultar.
type RecoverItem struct {
	MessageID      uuid.UUID       `db:"message_id"`
	ConversationID uuid.UUID       `db:"conversation_id"`
	Channel        string          `db:"channel"`
	Text           *string         `db:"text"`
	Attachments    json.RawMessage `db:"attachments"`
	SentAt         *time.Time      `db:"sent_at"`
	AnalysisID     *uuid.UUID      `db:"analysis_id"`
	Status         string          `db:"status"`
	Attempts       int             `db:"attempts"`
}

// ---------------------------------------------------------------------------
// Ciclo de vida del analisis por mensaje
// ---------------------------------------------------------------------------

// Claim reserva el derecho a analizar un mensaje.
//
// Es el mismo patron que webhook_events: INSERT ... ON CONFLICT DO NOTHING
// RETURNING. Si no devuelve fila, otro worker (o un reintento del webhook) ya
// lo tomo. El UNIQUE(message_id) de la tabla es el que hace de idempotencia.
func (r *Repository) Claim(ctx context.Context, messageID, conversationID uuid.UUID) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO message_analyses (message_id, conversation_id, status)
		 VALUES ($1, $2, 'processing')
		 ON CONFLICT (message_id) DO NOTHING
		 RETURNING id`,
		messageID, conversationID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("failed to claim analysis: %w", err)
	}
	return id, true, nil
}

// ClaimExisting verifica que una fila siga EN PROCESAMIENTO y la devuelve para
// seguir trabajando. Devuelve false si ya termino (ok/error/skipped) o si la
// fila cambio de mensaje: en ese caso el trabajo ya lo hizo otro y no se toca.
//
// Es el camino del audio -> texto: processAudio reserva la fila para dejar
// trazabilidad del audio, transcribe, y despues processText debe ANALIZAR esa
// misma fila. Un Claim normal no serviria (ON CONFLICT DO NOTHING).
func (r *Repository) ClaimExisting(ctx context.Context, id uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`UPDATE message_analyses SET attempts = attempts + 1, updated_at = now()
		 WHERE id = $1 AND status = 'processing'
		 RETURNING true`,
		id,
	).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to reuse analysis: %w", err)
	}
	return ok, nil
}

// MarkOK cierra el analisis con exito. Guarda tambien la salida cruda del
// modelo (columna raw): sin eso no hay forma de auditar por que dijo eso.
func (r *Repository) MarkOK(ctx context.Context, id uuid.UUID, res AnalysisResult) error {
	detalles := res.Detalles
	if detalles == nil {
		detalles = map[string]string{}
	}
	detallesJSON, err := json.Marshal(detalles)
	if err != nil {
		return fmt.Errorf("failed to marshal detalles: %w", err)
	}
	raw := res.Raw
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}

	_, err = r.pool.Exec(ctx,
		`UPDATE message_analyses
		 SET status = 'ok', intent = $2, resumen = $3, productos = $4, cantidades = $5,
		     tipo_cantidad = $6, detalles = $7, confianza = $8, needs_review = $9,
		     model = $10, latency_ms = $11,
		     raw = $12, asr_text = COALESCE($13, asr_text), asr_ms = COALESCE($14, asr_ms),
		     asr_model = COALESCE($15, asr_model), error = NULL
		 WHERE id = $1`,
		id, res.Analysis.Intent, res.Analysis.Resumen, res.Analysis.Productos,
		res.Analysis.Cantidades, res.Analysis.TipoCantidad, string(detallesJSON),
		res.Analysis.Confianza, res.NeedsReview,
		res.Model, res.LatencyMS, string(raw), res.ASRText, res.ASRMS, res.ASRModel,
	)
	if err != nil {
		return fmt.Errorf("failed to save analysis: %w", err)
	}
	return nil
}

// MarkError deja el error registrado. Devuelve si la fila quedo en 'pending'
// (reintentable) o en 'error' (terminal).
//
// retryable=false NO es "no se pudo ahora": es "esto no va a mejorar solo".
// Un prompt mal Formationado o un 400 no se arreglan reintentando, y reintentar
// quema CPU y esconde el bug.
func (r *Repository) MarkError(ctx context.Context, id uuid.UUID, cause error, retryable bool, attempt int, maxAttempts int) (bool, error) {
	status := "error"
	if retryable && attempt < maxAttempts {
		status = "pending"
	}
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}

	_, err := r.pool.Exec(ctx,
		`UPDATE message_analyses
		 SET status = $2, error = $3
		 WHERE id = $1`,
		id, status, fmt.Sprintf("intento %d/%d: %s", attempt, maxAttempts, msg),
	)
	if err != nil {
		return false, fmt.Errorf("failed to mark error: %w", err)
	}
	return status == "pending", nil
}

// MarkSkipped deja el mensaje sin analizar a proposito, con el motivo. Es un
// estado de exito: no se reintenta nunca.
func (r *Repository) MarkSkipped(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE message_analyses SET status = 'skipped', skip_reason = $2 WHERE id = $1`,
		id, reason,
	)
	if err != nil {
		return fmt.Errorf("failed to mark skipped: %w", err)
	}
	return nil
}

// SetTranscript guarda la transcripcion de una nota de voz y la marca como
// utilizable. Se llama antes del analisis para que el operador vea el texto
// aunque Whisper este caido.
func (r *Repository) SetTranscript(ctx context.Context, id uuid.UUID, text string, ms int, model string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE message_analyses
		 SET asr_text = $2, asr_ms = $3, asr_model = $4
		 WHERE id = $1`,
		id, text, ms, model,
	)
	if err != nil {
		return fmt.Errorf("failed to set transcript: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Recuperacion / backfill
// ---------------------------------------------------------------------------

// ListRecoverable devuelve los mensajes con analisis en 'pending' o
// 'processing'. Sirve para dos cosas:
//   - al arrancar el backend (crash a mitad de un trabajo)
//   - reintento diferido de lo que quedo en 'pending' por un error transitorio
func (r *Repository) ListRecoverable(ctx context.Context, limit int) ([]RecoverItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.message_id, a.conversation_id, c.channel, m.text, m.attachments,
		        m.sent_at, a.id, a.status,
		        COALESCE(substring(a.error from 'intento ([0-9]+)/'), '0')::int AS attempts
		 FROM message_analyses a
		 JOIN messages m ON m.id = a.message_id
		 JOIN conversations c ON c.id = a.conversation_id
		 WHERE a.status IN ('pending', 'processing')
		 ORDER BY m.sent_at
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list recoverable: %w", err)
	}
	return scanRecoverItems(rows)
}

// ListUnanalyzed devuelve mensajes entrados que todavia no tienen fila de
// analisis. Es el backfill: NO se encola solo, lo dispara el operador desde la
// UI, porque analizar todo el historico de golpe quemaria la CPU del equipo.
func (r *Repository) ListUnanalyzed(ctx context.Context, limit int) ([]RecoverItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT m.id, m.conversation_id, c.channel, m.text, m.attachments, m.sent_at,
		        NULL::uuid, 'pending', 0
		 FROM messages m
		 JOIN conversations c ON c.id = m.conversation_id
		 WHERE m.direction = 'incoming'
		   AND NOT EXISTS (SELECT 1 FROM message_analyses a WHERE a.message_id = m.id)
		 ORDER BY m.sent_at
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list unanalyzed: %w", err)
	}
	return scanRecoverItems(rows)
}

func scanRecoverItems(rows pgx.Rows) ([]RecoverItem, error) {
	defer rows.Close()
	out := []RecoverItem{}
	for rows.Next() {
		var it RecoverItem
		if err := rows.Scan(&it.MessageID, &it.ConversationID, &it.Channel, &it.Text,
			&it.Attachments, &it.SentAt, &it.AnalysisID, &it.Status, &it.Attempts); err != nil {
			return nil, fmt.Errorf("failed to scan recoverable: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate recoverable: %w", err)
	}
	return out, nil
}

// ResetStale devuelve a 'pending' lo que quedo en 'processing'. Si el proceso
// murio con un trabajo a medias, ese mensaje llevaba horas esperando en una
// cola que ya no existe.
func (r *Repository) ResetStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE message_analyses
		 SET status = 'pending', error = 'recuperado al reiniciar el backend'
		 WHERE status = 'processing' AND created_at < now() - make_interval(secs => $1)`,
		int(olderThan.Seconds()),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to reset stale: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Counts devuelve el estado de la cola.
func (r *Repository) Counts(ctx context.Context) (*AnalysisCounts, error) {
	var c AnalysisCounts
	err := r.pool.QueryRow(ctx,
		`SELECT count(*)::int,
		        count(*) FILTER (WHERE status = 'pending')::int,
		        count(*) FILTER (WHERE status = 'processing')::int,
		        count(*) FILTER (WHERE status = 'ok')::int,
		        count(*) FILTER (WHERE status = 'error')::int,
		        count(*) FILTER (WHERE status = 'skipped')::int,
		        count(*) FILTER (WHERE needs_review AND status = 'ok')::int
		 FROM message_analyses`,
	).Scan(&c.Total, &c.Pending, &c.Processing, &c.OK, &c.Error, &c.Skipped, &c.NeedsReview)
	if err != nil {
		return nil, fmt.Errorf("failed to count analyses: %w", err)
	}
	return &c, nil
}

// ListByConversation devuelve el historial de analisis del hilo, para que el
// operador pueda ver por que el pedido quedo como quedo.
func (r *Repository) ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]AnalysisRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+analysisColumns+`
		 FROM message_analyses
		 WHERE conversation_id = $1
		 ORDER BY created_at DESC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list analyses: %w", err)
	}
	defer rows.Close()

	out := []AnalysisRow{}
	for rows.Next() {
		a, err := scanAnalysis(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan analysis: %w", err)
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate analyses: %w", err)
	}
	return out, nil
}

// ListErrors lista los fallos para la pantalla de observabilidad.
func (r *Repository) ListErrors(ctx context.Context, limit int) ([]AnalysisRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+analysisColumns+`
		 FROM message_analyses
		 WHERE status = 'error'
		 ORDER BY created_at DESC
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list errors: %w", err)
	}
	defer rows.Close()

	out := []AnalysisRow{}
	for rows.Next() {
		a, err := scanAnalysis(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan error row: %w", err)
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate errors: %w", err)
	}
	return out, nil
}

// Transcript arma el hilo en el formato que espera el prompt: "Cliente: ...",
// "Agente: ...". Solo entra lo que el cliente o el operador escribieron de
// verdad: los mensajes de sistema y los automaticos meterian ruido y el modelo
// los leeria como pedidos.
//
// Los N mensajes mas RECIENTES: el hilo importa por lo que acaba de pasar, y
// de hace tres semanas no.
// Transcript arma el hilo de los ultimos N mensajes para darle contexto al
// modelo. excludeMessageID saca el mensaje que se esta analizando: sin eso el
// modelo ve el mensaje actual dos veces (una en el hilo, otra como "MENSAJE
// ACTUAL") y los benchmarks de copia de few-shot lo cogieron haciendolo.
func (r *Repository) Transcript(ctx context.Context, conversationID, excludeMessageID uuid.UUID, limit int) (string, error) {
	if limit < 0 {
		limit = 0
	}
	if limit > 50 {
		limit = 50
	}
	if limit == 0 {
		return "", nil
	}

	// COALESCE(sent_at, created_at): sent_at es nullable y Postgres ordena los
	// NULLS AL FINAL en DESC, lo que dejaria los mensajes sin sent_at fuera del
	// contexto sin avisar.
	rows, err := r.pool.Query(ctx,
		`SELECT direction, text
		 FROM messages
		 WHERE conversation_id = $1
		   AND direction IN ('incoming', 'outgoing')
		   AND text IS NOT NULL AND btrim(text) <> ''
		   AND ($3::uuid IS NULL OR id <> $3)
		 ORDER BY COALESCE(sent_at, created_at) DESC, created_at DESC
		 LIMIT $2`,
		conversationID, limit, nullableUUID(excludeMessageID),
	)
	if err != nil {
		return "", fmt.Errorf("failed to read transcript: %w", err)
	}
	defer rows.Close()

	type line struct {
		who  string
		what string
	}
	lines := make([]line, 0, limit)
	for rows.Next() {
		var direction, text string
		if err := rows.Scan(&direction, &text); err != nil {
			return "", fmt.Errorf("failed to scan transcript row: %w", err)
		}
		who := "Agente"
		if direction == "incoming" {
			who = "Cliente"
		}
		lines = append(lines, line{who: who, what: text})
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate transcript: %w", err)
	}

	// Venimos de DESC (lo mas nuevo primero): hay que dar vuelta para que el
	// modelo lea el hilo en orden cronologico.
	var b strings.Builder
	for i := len(lines) - 1; i >= 0; i-- {
		b.WriteString(lines[i].who)
		b.WriteString(": ")
		b.WriteString(lines[i].what)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// ---------------------------------------------------------------------------
// Pedidos consolidados
// ---------------------------------------------------------------------------

// OrderView es un pedido con los datos de contacto: lo que necesita la tabla
// de /pedidos para no pedir una peticion por fila.
type OrderView struct {
	Order
	ID            uuid.UUID  `db:"id" json:"id"`
	Status        string     `db:"status" json:"status"`
	ContactID     uuid.UUID  `db:"contact_id" json:"contact_id"`
	ContactName   *string    `db:"contact_name" json:"contact_name"`
	ContactAvatar *string    `db:"contact_avatar" json:"contact_avatar"`
	Channel       string     `db:"channel" json:"channel"`
	Pending       bool       `db:"pending" json:"pending"`
	UnreadCount   int        `db:"unread_count" json:"unread_count"`
	LastInboundAt *time.Time `db:"last_inbound_at" json:"last_inbound_at"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}

const orderColumns = `o.id, o.conversation_id, o.intent, o.resumen, o.productos, o.cantidades,
	o.detalles, o.confianza, o.needs_review, o.source_message_id, o.model, o.edited, o.status,
	o.created_at, o.updated_at, o.revision, o.is_current, o.superseded_at, o.superseded_by`

// orderColumnCount es cuantas columnas trae orderColumns: las 15 columnas
// crudas de conversation_orders mas las 4 de historial que agrego la 000015.
// Los scanners de abajo comparan contra este numero para fallar con un mensaje
// propio en vez del "number of field descriptions must equal number of
// destinations" de pgx, que no dice que columna sobra ni cual falta.
const orderColumnCount = 19

// viewExtraColumnCount son las 7 columnas que agregan los SELECT con JOIN por
// encima de orderColumns: 6 de contacto y `pending`.
const viewExtraColumnCount = 7

// orderRow son las columnas crudas de conversation_orders: los campos de
// negocio de Order mas id/status/created_at/updated_at y el historial.
//
// Hace falta porque los caminos de lectura son de dos formas: los SELECT con
// JOIN agregan 6 columnas de contacto (21 en total, OrderView), y los UPDATE
// con RETURNING devuelven solo las 15 de la tabla. Escanear con el struct
// equivocado da el error de pgx "got 15 and 11", que no dice nada de por sí.
type orderRow struct {
	Order
	ID        uuid.UUID
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func scanOrderRow(row pgx.Row) (*orderRow, error) {
	var r orderRow
	dest := []any{&r.ID, &r.ConversationID, &r.Intent, &r.Resumen, &r.Productos,
		&r.Cantidades, &r.Detalles, &r.Confianza, &r.NeedsReview, &r.SourceMessageID,
		&r.Model, &r.Edited, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		&r.Revision, &r.IsCurrent, &r.SupersededAt, &r.SupersededBy}
	if len(dest) != orderColumnCount {
		return nil, fmt.Errorf("scanOrderRow: %d destinos, orderColumns trae %d",
			len(dest), orderColumnCount)
	}
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	return &r, nil
}

// scanOrderView lee las 21 columnas de un OrderView. Son 15 del pedido
// (orderColumns) + 6 de contacto: el SELECT que arma la vista trae todas, asi
// que el Scan tiene que traerlas todas. Con 15 destinos pgx tira
// "number of field descriptions must equal number of destinations" y la
// consulta revienta con un 500 sin detalle.
func scanOrderView(row pgx.Row) (*OrderView, error) {
	var v OrderView
	dest := []any{&v.ID, &v.ConversationID, &v.Intent, &v.Resumen, &v.Productos,
		&v.Cantidades, &v.Detalles, &v.Confianza, &v.NeedsReview, &v.SourceMessageID,
		&v.Model, &v.Edited, &v.Status, &v.CreatedAt, &v.UpdatedAt,
		&v.Revision, &v.IsCurrent, &v.SupersededAt, &v.SupersededBy,
		&v.ContactID, &v.Channel, &v.UnreadCount, &v.LastInboundAt,
		&v.ContactName, &v.ContactAvatar, &v.Pending}
	if len(dest) != orderColumnCount+viewExtraColumnCount {
		return nil, fmt.Errorf("scanOrderView: %d destinos, la consulta trae %d",
			len(dest), orderColumnCount+viewExtraColumnCount)
	}
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	return &v, nil
}

// scanOrderViews es el mismo scan en un lote de filas, para no repetir la lista
// de destinos en cada pagina de /api/orders.
func scanOrderViews(rows pgx.Rows) ([]OrderView, error) {
	out := []OrderView{}
	for rows.Next() {
		v, err := scanOrderView(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		out = append(out, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate orders: %w", err)
	}
	return out, nil
}

// GetOrderByConversation devuelve el pedido consolidado vigente del hilo, o nil
// si todavia no hay ninguno. Solo mira is_current: las revisiones viejas son
// historial para la UI y no entran nunca en la consolidacion.
func (r *Repository) GetOrderByConversation(ctx context.Context, conversationID uuid.UUID) (*Order, error) {
	var o Order
	err := r.pool.QueryRow(ctx,
		`SELECT conversation_id, intent, resumen, productos, cantidades, detalles,
		        confianza, needs_review, source_message_id, model, edited
		 FROM conversation_orders WHERE conversation_id = $1 AND is_current`,
		conversationID,
	).Scan(&o.ConversationID, &o.Intent, &o.Resumen, &o.Productos, &o.Cantidades,
		&o.Detalles, &o.Confianza, &o.NeedsReview, &o.SourceMessageID, &o.Model, &o.Edited)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	return &o, nil
}

// UpsertOrder guarda el pedido consolidado. Devuelve false si no escribio
// nada, que es la senal de que el pedido esta editado por un humano (el worker
// no emite SSE al vacio).
//
// La 000015 saco el UNIQUE (conversation_id), asi que el "upsert" ya no puede
// ser un ON CONFLICT: ahora son tres casos distintos y se resuelven con un
// UPDATE ... WHERE is_current AND edited=false mas un INSERT de la fila nueva
// solo si no hay ninguna vigente:
//
//  1. hay vigente sin editar  -> UPDATE (la IA continua)
//  2. hay vigente editada     -> nada, false (regla dura #9)
//  3. no hay vigente          -> INSERT revision 1
//
// Va en UNA sentencia con CTE. Los data-modifying CTE de Postgres ejecutan sus
// sub-consultas con el mismo snapshot, asi que el INSERT ve la fila vigente
// aunque el UPDATE la haya tocado: en el caso 1 el NOT EXISTS da falso y no
// inserta. Sin el "UNION ALL" final no habria forma de devolver el id en el
// caso 1, porque el RETURNING del INSERT no devuelve nada ahi.
func (r *Repository) UpsertOrder(ctx context.Context, o *Order) (updated bool, err error) {
	detalles := o.Detalles
	if detalles == nil {
		detalles = map[string]string{}
	}
	detallesJSON, err := json.Marshal(detalles)
	if err != nil {
		return false, fmt.Errorf("failed to marshal detalles: %w", err)
	}
	productos := o.Productos
	if productos == nil {
		productos = []string{}
	}
	cantidades := o.Cantidades
	if cantidades == nil {
		cantidades = []int{}
	}

	var id uuid.UUID
	err = r.pool.QueryRow(ctx,
		`WITH upd AS (
		   UPDATE conversation_orders
		      SET intent = $2, resumen = $3, productos = $4, cantidades = $5,
		          detalles = $6, confianza = $7, needs_review = $8,
		          source_message_id = $9, model = $10,
		          applied_analysis_at = now(), updated_at = now()
		    WHERE conversation_id = $1 AND is_current AND edited = false
		    RETURNING id
		 ), ins AS (
		   INSERT INTO conversation_orders
		     (conversation_id, intent, resumen, productos, cantidades, detalles, confianza,
		      needs_review, source_message_id, model, status, revision, is_current, applied_analysis_at)
		   SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'nuevo', 1, true, now()
		    WHERE NOT EXISTS (SELECT 1 FROM upd)
		      AND NOT EXISTS (SELECT 1 FROM conversation_orders
		                       WHERE conversation_id = $1 AND is_current)
		    RETURNING id
		 )
		 SELECT id FROM upd UNION ALL SELECT id FROM ins`,
		o.ConversationID, o.Intent, o.Resumen, productos, cantidades, string(detallesJSON),
		o.Confianza, o.NeedsReview, o.SourceMessageID, o.Model,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to upsert order: %w", err)
	}
	return true, nil
}

// OrderFilter son los filtros de la vista /pedidos.
type OrderFilter struct {
	Status string
	Intent string
	Search string
	Limit  int
	Offset int
}

// ListOrders pagina los pedidos consolidados VIGENTES, del mas reciente al mas
// viejo. Las revisiones viejas salen por el historial del hilo, no en esta
// lista: /api/orders es la bandeja de trabajo, no un archivo.
func (r *Repository) ListOrders(ctx context.Context, f OrderFilter) ([]OrderView, int64, error) {
	if f.Limit < 1 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	where := ` WHERE 1=1`
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if f.Status != "" {
		where += ` AND o.status = ` + arg(f.Status)
	}
	if f.Intent != "" {
		where += ` AND o.intent = ` + arg(f.Intent)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		p := arg("%" + s + "%")
		where += ` AND (o.resumen ILIKE ` + p + `
		              OR EXISTS (SELECT 1 FROM unnest(o.productos) pr
		                         WHERE pr ILIKE ` + p + `)
		              OR c.name ILIKE ` + p + `)`
	}

	// Todo listado de pedidos operativo mira solo la vigente. Si esto se
	// olvida, /api/orders duplica hilos en vez de mostrar historial.
	where += ` AND o.is_current`

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM conversation_orders o
		 JOIN conversations c ON c.id = o.conversation_id`+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	limitArg := arg(f.Limit)
	offsetArg := arg(f.Offset)
	query := `SELECT ` + orderColumns + `,
	    c.contact_id, c.channel, c.unread_count, c.last_inbound_at,
	    ct.name, ct.avatar_url` + pendingColumn + `
	  FROM conversation_orders o
	  JOIN conversations c ON c.id = o.conversation_id
	  JOIN contacts ct ON ct.id = c.contact_id` + where + `
	  ORDER BY o.updated_at DESC
	  LIMIT ` + limitArg + ` OFFSET ` + offsetArg

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	out, err := scanOrderViews(rows)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// GetOrderByID devuelve un pedido puntual (para editarlo).
func (r *Repository) GetOrderByID(ctx context.Context, id uuid.UUID) (*OrderView, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+`,
		    c.contact_id, c.channel, c.unread_count, c.last_inbound_at, ct.name, ct.avatar_url`+pendingColumn+`
		 FROM conversation_orders o
		 JOIN conversations c ON c.id = o.conversation_id
		 JOIN contacts ct ON ct.id = c.contact_id
		 WHERE o.id = $1`, id)

	v, err := scanOrderView(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	return v, nil
}

// OperatorOrderPatch es lo que un humano puede corregir. Pisar cualquiera de
// estos campos marca edited=true, que es la forma de tellarle a la IA que
// este pedido ya no se toca.
type OperatorOrderPatch struct {
	Intent       *string
	Resumen      *string
	Productos    []string
	Cantidades   []int
	TipoCantidad *string
	Detalles     map[string]string
	Status       *string
	NeedsReview  *bool
}

// UpdateOrderByOperator aplica la correccion humana y sella edited=true.
//
// Solo la revision vigente es editable: una que ya fue reemplazada es historial
// y devolver 404 es lo correcto, no un error silencioso.
func (r *Repository) UpdateOrderByOperator(ctx context.Context, id uuid.UUID, p OperatorOrderPatch, userID uuid.UUID) (*OrderView, error) {
	// COALESCE por campo: un PATCH parcial no puede borrar lo que no se manda.
	detalles := p.Detalles
	if detalles == nil {
		detalles = map[string]string{}
	}
	detallesJSON, err := json.Marshal(detalles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal detalles: %w", err)
	}
	productos := p.Productos
	if productos == nil {
		productos = []string{}
	}
	cantidades := p.Cantidades
	if cantidades == nil {
		cantidades = []int{}
	}

	row := r.pool.QueryRow(ctx,
		// El alias "o" es obligatorio: orderColumns viene con el prefijo "o."
		// para poder reusarse en los SELECT con JOIN, y sin alias el RETURNING
		// no resuelve "o.id" (missing FROM-clause entry).
		`UPDATE conversation_orders AS o
		 SET intent = COALESCE($2, o.intent),
		     resumen = COALESCE($3, o.resumen),
		     productos = CASE WHEN $4::text[] = '{}' THEN o.productos ELSE $4 END,
		     cantidades = CASE WHEN $5::int[] = '{}' THEN o.cantidades ELSE $5 END,
		     detalles = CASE WHEN $6::jsonb = '{}'::jsonb THEN o.detalles
		                     ELSE o.detalles || $6::jsonb END,
		     status = COALESCE($7, o.status),
		     needs_review = COALESCE($8, o.needs_review),
		     edited = true, edited_by = $9, updated_at = now()
		 WHERE o.id = $1 AND o.is_current
		 RETURNING `+orderColumns,
		id, p.Intent, p.Resumen, productos, cantidades, string(detallesJSON), p.Status,
		p.NeedsReview, uuid.NullUUID{UUID: userID, Valid: true},
	)

	raw, err := scanOrderRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// O no existe, o ya no es la vigente. Los dos casos son 404 para el
		// operador: no hay un pedido editable con ese id.
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}
	// Los datos de contacto quedan vacios: el RETURNING no hace JOIN. El
	// handler vuelve a leer la vista completa si los necesita.
	return &OrderView{
		Order:     raw.Order,
		ID:        raw.ID,
		Status:    raw.Status,
		CreatedAt: raw.CreatedAt,
		UpdatedAt: raw.UpdatedAt,
	}, nil
}

// SetOrderStatus mueve el pedido en el embudo sin marcarlo como editado: el
// operador puede cambiar el estado sin "tomar posesion" del contenido.
func (r *Repository) SetOrderStatus(ctx context.Context, id uuid.UUID, status string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE conversation_orders SET status = $2, updated_at = now()
		 WHERE id = $1 AND is_current`,
		id, status,
	)
	if err != nil {
		return fmt.Errorf("failed to set order status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrNotFound lo usan los handlers para diferenciar 404 de 500.
var ErrNotFound = errors.New("not found")

// Badge es lo minimo que la bandeja necesita por conversacion: la intencion
// consolidada y si el operador tiene que mirarlo.
//
// Pending es el caso que antes no existia en ningun lado: un analysis "pedido"
// mas nuevo que el pedido vigente. Pasa seguido cuando el pedido esta editado
// (la IA no lo puede consolidar) y sin esto el hilo se ve tranquilo mientras el
// cliente ya mando otra cosa. El criterio es created_at > updated_at del pedido
// vigente, el MISMO que usa AcceptPending, para que la lista y la tarjeta no
// puedan discrepar.
type Badge struct {
	Intent      string `db:"intent"`
	NeedsReview bool   `db:"needs_review"`
	Pending     bool   `db:"pending"`
}

// BadgesByConversation resuelve los badges de toda una pagina en UNA query.
// El inbox lista 50 conversaciones: 50 queries por pantalla es exactamente el
// tipo de N+1 que hace que la bandeja "a veces" tarda medio segundo.
// BadgesByConversation devuelve intencion y needs_review de los pedidos de un
// lote de conversaciones, para pintar los badges del inbox en una sola query.
//
// Los ids van como []string y no como []uuid.UUID: el pool corre con
// DefaultQueryExecModeSimpleProtocol, que no tiene plan de encode para un
// array de uuid y devolvia "cannot find encode plan". Por eso el cast explicito.
func (r *Repository) BadgesByConversation(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Badge, error) {
	out := map[uuid.UUID]Badge{}
	if len(ids) == 0 {
		return out, nil
	}
	strIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		strIDs = append(strIDs, id.String())
	}

	rows, err := r.pool.Query(ctx,
		`SELECT o.conversation_id, COALESCE(o.intent, 'otro'), o.needs_review,
		        EXISTS (SELECT 1 FROM message_analyses a
		                 WHERE a.conversation_id = o.conversation_id
		                   AND a.status = 'ok' AND a.intent = 'pedido'
		                   AND `+pendingPredicate+`) AS pending
		 FROM conversation_orders o
		 WHERE o.conversation_id = ANY($1::uuid[]) AND o.is_current`, strIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load badges: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var b Badge
		if err := rows.Scan(&id, &b.Intent, &b.NeedsReview, &b.Pending); err != nil {
			return nil, fmt.Errorf("failed to scan badge: %w", err)
		}
		out[id] = b
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate badges: %w", err)
	}
	return out, nil
}

// GetAnalysisByMessage devuelve el analisis de un mensaje puntual, o nil si
// todavia no seidio.
func (r *Repository) GetAnalysisByMessage(ctx context.Context, messageID uuid.UUID) (*AnalysisRow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+analysisColumns+` FROM message_analyses WHERE message_id = $1`, messageID)

	a, err := scanAnalysis(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get analysis: %w", err)
	}
	return a, nil
}

// GetOrderByConversationView (vigente) es la version "con datos de contacto" del pedido
// consolidado: lo que pinta la tarjeta del panel de la conversacion.
func (r *Repository) GetOrderByConversationView(ctx context.Context, conversationID uuid.UUID) (*OrderView, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+`,
		    c.contact_id, c.channel, c.unread_count, c.last_inbound_at, ct.name, ct.avatar_url`+pendingColumn+`
		 FROM conversation_orders o
		 JOIN conversations c ON c.id = o.conversation_id
		 JOIN contacts ct ON ct.id = c.contact_id
		 WHERE o.conversation_id = $1 AND o.is_current
		 LIMIT 1`, conversationID)

	v, err := scanOrderView(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order view: %w", err)
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Historial de revisiones, pedido pendiente y reapertura
// ---------------------------------------------------------------------------

// ErrNoPending: el pedido vigente esta al dia y no hay nada posterior que
// aplicar. No es un error de sistema, es un 409 con sentido para el operador.
var ErrNoPending = errors.New("no hay pedido pendiente")

// ErrNotEdited: el pedido vigente no esta editado, entonces no hay nada que
// reabrir. Un 400, no un 404: el pedido existe.
var ErrNotEdited = errors.New("el pedido no esta editado")

// pendingColumn es el EXISTS que responde "el cliente pidio algo que todavia
// no se consolido". Va como columna de la fila (no como filtro) para que
// /api/orders devuelva el pedido junto con su pendiente en una sola ida: el
// listado se pagina en el server, asi que traer los pendientes aparte seria
// otra consulta con otra paginacion y los dos resultados podrian no calzar.
const pendingColumn = `,
    EXISTS (SELECT 1 FROM message_analyses a
             WHERE a.conversation_id = o.conversation_id
               AND a.status = 'ok' AND a.intent = 'pedido'
               AND ` + pendingPredicate + `) AS pending`

// prefixColumns le pone un alias a cada columna de una lista separada por
// comas. Hace falta cuando la misma lista de columnas se usa con y sin JOIN:
// sin prefijo, Postgres dice "column reference id is ambiguous" y no aclara
// entre cuales dos tablas.
func prefixColumns(cols, prefix string) string {
	parts := strings.Split(cols, ",")
	for i, c := range parts {
		// Con punto, no pegado: "a"+"id" es "aid", una columna que no existe
		// y un error que no dice nada de cual de las dos opciones estaba mal.
		parts[i] = prefix + "." + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}

// pendingPredicate es la definicion UNICA de "pedido pendiente", en SQL, para
// que el badge del inbox, el banner de la tarjeta y AcceptPending no puedan
// discrepar entre si.
//
// La segunda rama (applied_analysis_at IS NULL) es la que hace que un pedido
// viejo migrado de antes de la 000015 muestre todo lo posterior como pendiente
// en vez de asumir que ya esta todo consolidado.
const pendingPredicate = `(o.applied_analysis_at IS NULL OR a.created_at > o.applied_analysis_at)`

// querier es lo que pool y tx tienen en comun. Las funciones de este bloque
// corren dentro de una transaccion y fuera de ella con la misma logica.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// storedAnalysis es un analysis ya persistido (o sea, YA enriquecido: productos
// canonicalizados, cantidades saneadas, needs_review calculado por reglas).
//
// Volverlo a Analysis y pasar por OrderFromAnalysis es lo que hace que el
// guardarraíl siga valiendo al aplicar o reabrir: los valores salen de la fila
// ya saneada, no de una llamada nueva al modelo que puede volver a inventarse
// el 5 de "2 stickers de 5cm".
type storedAnalysis struct {
	MessageID    uuid.UUID
	Intent       string
	Resumen      *string
	Productos    []string
	Cantidades   []int
	TipoCantidad *string
	Detalles     map[string]string
	Confianza    *float64
	NeedsReview  bool
	Model        *string
	CreatedAt    time.Time
}

// toAnalysis deshace el jsonb de detalles para volver al struct del modelo.
func (s *storedAnalysis) toAnalysis() *Analysis {
	a := &Analysis{
		Intent:       s.Intent,
		Resumen:      derefString(s.Resumen),
		Productos:    s.Productos,
		Cantidades:   s.Cantidades,
		TipoCantidad: s.TipoCantidad,
		Confianza:    derefFloat(s.Confianza),
	}
	if v, ok := s.Detalles["material"]; ok {
		a.Material = &v
	}
	if v, ok := s.Detalles["medidas"]; ok {
		a.Medidas = &v
	}
	if v, ok := s.Detalles["personalizacion"]; ok {
		a.Personalizacion = &v
	}
	if v, ok := s.Detalles["fecha_entrega"]; ok {
		a.FechaEntrega = &v
	}
	return a
}

// currentOrder bloquea la fila vigente del pedido. El FOR UPDATE es lo que
// hace segura la carrera entre dos operadores o entre un operador y el worker:
// el segundo espera en vez de crear dos revisiones a la vez.
func (r *Repository) currentOrder(ctx context.Context, q querier, orderID uuid.UUID) (
	conversationID uuid.UUID, revision int, appliedAt *time.Time, edited bool, err error) {
	err = q.QueryRow(ctx,
		`SELECT conversation_id, revision, applied_analysis_at, edited
		 FROM conversation_orders WHERE id = $1 AND is_current FOR UPDATE`,
		orderID,
	).Scan(&conversationID, &revision, &appliedAt, &edited)
	return
}

// pendingAnalyses son los analysis "pedido" que todavia no llegaron al pedido
// vigente, del mas viejo al mas nuevo. Usa pendingPredicate contra
// applied_analysis_at.
//
// Devuelve TODOS, no solo el ultimo, y en orden cronologico. Antes tomaba el
// mas reciente y se:"-eso estaba bien cuando un pendiente era siempre "el pedido
// nuevo completo", pero en cuanto un pendiente puede ser un INCREMENTO la
// diferencia es visible: "sumale 2 mas" seguido de "y sumale 3 mas" deja dos
// pendientes, y quedarse con el ultimo consolida 30+3 en vez de 30+2+3. La
// peticion del cliente se pierde en silencio otra vez, que es el mismo fallo que
// la 000016 vino a arreglar.
//
// El orden importa: son deltas que se suman en cadena.
func (r *Repository) pendingAnalyses(ctx context.Context, q querier, conversationID uuid.UUID) ([]*storedAnalysis, error) {
	rows, err := q.Query(ctx,
		`SELECT a.message_id, a.intent, a.resumen, a.productos, a.cantidades, a.tipo_cantidad,
		        a.detalles, a.confianza, a.needs_review, a.model, a.created_at
		 FROM message_analyses a
		 JOIN conversation_orders o ON o.conversation_id = a.conversation_id AND o.is_current
		 WHERE a.conversation_id = $1 AND a.status = 'ok' AND a.intent = 'pedido'
		   AND `+pendingPredicate+`
		 ORDER BY a.created_at ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending analyses: %w", err)
	}
	defer rows.Close()

	var out []*storedAnalysis
	for rows.Next() {
		var s storedAnalysis
		if err := rows.Scan(&s.MessageID, &s.Intent, &s.Resumen, &s.Productos, &s.Cantidades,
			&s.TipoCantidad, &s.Detalles, &s.Confianza, &s.NeedsReview, &s.Model, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan pending analysis: %w", err)
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read pending analyses: %w", err)
	}
	return out, nil
}

// pendingAnalysis conserva la forma de "el pendiente mas reciente" para los
// lugares que solo necesitan saber SI hay algo pendiente.
func (r *Repository) pendingAnalysis(ctx context.Context, q querier, conversationID uuid.UUID) (*storedAnalysis, error) {
	all, err := r.pendingAnalyses(ctx, q, conversationID)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all[len(all)-1], nil
}

// ListRevisions devuelve el historial completo del hilo, de la revision mas
// nueva a la mas vieja. Es lo que pinta el timeline de la tarjeta.
func (r *Repository) ListRevisions(ctx context.Context, conversationID uuid.UUID) ([]OrderView, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+`,
		    c.contact_id, c.channel, c.unread_count, c.last_inbound_at, ct.name, ct.avatar_url`+pendingColumn+`
		 FROM conversation_orders o
		 JOIN conversations c ON c.id = o.conversation_id
		 JOIN contacts ct ON ct.id = c.contact_id
		 WHERE o.conversation_id = $1
		 ORDER BY o.revision DESC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list revisions: %w", err)
	}
	defer rows.Close()
	return scanOrderViews(rows)
}

// AcceptPending crea la revision siguiente a partir del analysis pendiente: el
// pedido nuevo del cliente pasa a ser el vigente y el anterior queda en el
// historial. Es la respuesta a "el cliente pidio algo nuevo y mi pedido ya no
// lo refleja".
//
// No relaja la regla dura #9: la fila vieja queda sellada para siempre, solo
// deja de ser la vigente. La nueva revision arranca sin editar, asi que la IA
// vuelve a consolidar sobre ella a partir de aca (el operador opto por esto
// explicitamente).
func (r *Repository) AcceptPending(ctx context.Context, orderID, userID uuid.UUID) (*OrderView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	convID, revision, _, _, err := r.currentOrder(ctx, tx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lock order: %w", err)
	}

	pends, err := r.pendingAnalyses(ctx, tx, convID)
	if err != nil {
		return nil, err
	}
	if len(pends) == 0 {
		return nil, ErrNoPending
	}

	// 1) Archivar la vigente. Con el indice unico parcial (is_current) el
	// INSERT del paso 2 no puede ocurrir antes de esto.
	tag, err := tx.Exec(ctx,
		`UPDATE conversation_orders SET is_current = false, superseded_at = now()
		 WHERE id = $1 AND is_current`, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to archive revision: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	// 2) La revision nueva.
	//
	// NO se arma con el analysis suelto: se CONSOLIDA sobre el pedido que
	// acaba de archivarse. La diferencia se ve en un caso que era facil de
	// perder: el operador editó su pedido (figura Kratos), el cliente escribe
	// "y sumale 2 stickers mas" y el operador toca "Aplicar". Tomando el analysis
	// tal cual, la revision nueva quedaba con 2 stickers y la figura
	// desaparecia. Consolidando, la nueva revision es figura + 2 stickers, que es
	// lo que el operador quiso decir con "aplicar".
	//
	// MergeOrder ignora el sello edited de la fila previa porque aca la escritura
	// la autorizo el operador de forma explicita (mismo criterio que Reopen).
	cur, err := r.getOrderTx(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	cur.Edited = false

	merged := cur
	needsReview := cur.NeedsReview
	for _, pend := range pends {
		a := pend.toAnalysis()
		next := OrderFromAnalysis(convID, pend.MessageID, a, derefString(pend.Model))
		next.NeedsReview = pend.NeedsReview
		merged = MergeOrder(merged, next, a.TipoCantidad)
		needsReview = needsReview || merged.NeedsReview
	}
	// NeedsReview del pedido consolidado: si CUALQUIER analysis se quedo con la
	// duda, el pedido entero se marca. MergeOrder reinicia el flag en cada
	// iteracion (copia del analysis nuevo), asi que se acumula aparte.
	merged.NeedsReview = needsReview

	detallesJSON, err := json.Marshal(merged.Detalles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal detalles: %w", err)
	}
	var newID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO conversation_orders
		   (conversation_id, intent, resumen, productos, cantidades, detalles, confianza,
		    needs_review, source_message_id, model, status, revision, is_current, applied_analysis_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'nuevo',$11,true,now())
		 RETURNING id`,
		convID, merged.Intent, merged.Resumen, merged.Productos, merged.Cantidades,
		string(detallesJSON), merged.Confianza, merged.NeedsReview, merged.SourceMessageID,
		merged.Model, revision+1,
	).Scan(&newID)
	if err != nil {
		return nil, fmt.Errorf("failed to create revision: %w", err)
	}

	// 3) Enlazar el historial con la revision que la reemplaza, para que el
	// timeline no tenga que deducirlo de timestamps.
	if _, err := tx.Exec(ctx,
		`UPDATE conversation_orders SET superseded_by = $2 WHERE id = $1`,
		orderID, newID); err != nil {
		return nil, fmt.Errorf("failed to link revision: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	return r.GetOrderByID(ctx, newID)
}

// ReopenOrder saca el sello edited de la revision vigente para que la IA vuelva
// a escribir sobre ella. A diferencia de AcceptPending NO crea revision: el
// operador dice "mi correccion estaba bien, solo dejo de actualizarse".
//
// Si hay un analysis pendiente se aplica sobre la MISMA fila con la misma
// MergeOrder que usa el worker, para que reabrir no pierda el pedido nuevo que
// el cliente ya mando.
func (r *Repository) ReopenOrder(ctx context.Context, orderID uuid.UUID) (*OrderView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	convID, _, _, edited, err := r.currentOrder(ctx, tx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lock order: %w", err)
	}
	if !edited {
		return nil, ErrNotEdited
	}

	prev, err := r.getOrderTx(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	prev.Edited = false

	if pends, err := r.pendingAnalyses(ctx, tx, convID); err != nil {
		return nil, err
	} else {
		// Todos los pendientes, en orden cronologico: si hay dos incrementos
		// seguidos se suman los dos. Ver pendingAnalyses.
		for _, pend := range pends {
			a := pend.toAnalysis()
			next := OrderFromAnalysis(convID, pend.MessageID, a, derefString(pend.Model))
			prev = MergeOrder(prev, next, a.TipoCantidad)
			prev.Edited = false
		}
	}

	detallesJSON, err := json.Marshal(prev.Detalles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal detalles: %w", err)
	}
	row := tx.QueryRow(ctx,
		// El alias "o" es obligatorio: orderColumns viene con el prefijo "o."
		// para poder reusarse en los SELECT con JOIN, y sin alias el
		// RETURNING no resuelve "o.id" (missing FROM-clause entry).
		`UPDATE conversation_orders AS o
		    SET intent=$2, resumen=$3, productos=$4, cantidades=$5, detalles=$6,
		        confianza=$7, needs_review=$8, source_message_id=$9, model=$10,
		        edited=false, edited_by=NULL, applied_analysis_at=now(), updated_at=now()
		  WHERE o.id=$1 AND o.is_current
		 RETURNING `+orderColumns,
		orderID, prev.Intent, prev.Resumen, prev.Productos, prev.Cantidades,
		string(detallesJSON), prev.Confianza, prev.NeedsReview, prev.SourceMessageID, prev.Model)
	raw, err := scanOrderRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to reopen order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	return &OrderView{Order: raw.Order, ID: raw.ID, Status: raw.Status,
		CreatedAt: raw.CreatedAt, UpdatedAt: raw.UpdatedAt}, nil
}

// getOrderTx lee el pedido dentro de la transaccion (sin datos de contacto: el
// caller ya armo la vista).
func (r *Repository) getOrderTx(ctx context.Context, q querier, id uuid.UUID) (*Order, error) {
	var o Order
	err := q.QueryRow(ctx,
		`SELECT conversation_id, intent, resumen, productos, cantidades, detalles,
		        confianza, needs_review, source_message_id, model, edited
		 FROM conversation_orders WHERE id = $1`, id,
	).Scan(&o.ConversationID, &o.Intent, &o.Resumen, &o.Productos, &o.Cantidades,
		&o.Detalles, &o.Confianza, &o.NeedsReview, &o.SourceMessageID, &o.Model, &o.Edited)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	return &o, nil
}

// PendingAnalysis devuelve, con forma completa para la UI, el analysis "pedido"
// mas nuevo que todavia no llego al pedido vigente. nil si no hay pedido o si el
// pedido ya esta al dia.
//
// Es lo que permite que el banner diga QUE pidio el cliente y no solo "hay algo
// pendiente": sin esto el operador tiene que ir a buscar el mensaje a mano.
func (r *Repository) PendingAnalysis(ctx context.Context, conversationID uuid.UUID) (*AnalysisRow, error) {
	// JOIN y no una lectura previa del tiempo del pedido: si no hay pedido
	// vigente no hay nada pendiente, y asi el caso "sin pedido" sale como 0
	// filas de la misma query.
	row := r.pool.QueryRow(ctx,
		`SELECT `+prefixColumns(analysisColumns, "a")+` FROM message_analyses a
		 JOIN conversation_orders o ON o.conversation_id = a.conversation_id AND o.is_current
		 WHERE a.conversation_id = $1 AND a.status = 'ok' AND a.intent = 'pedido'
		   AND `+pendingPredicate+`
		 ORDER BY a.created_at DESC LIMIT 1`, conversationID)

	a, err := scanAnalysis(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending analysis: %w", err)
	}
	return a, nil
}
