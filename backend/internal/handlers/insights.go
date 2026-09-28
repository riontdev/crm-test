package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/riont/crm/backend/internal/insight"
	"github.com/riont/crm/backend/internal/sse"
)

// InsightHandler expone el estado y el control del analizador de pedidos.
//
// Todos los endpoints son de la API protegida: el analisis contiene texto de
// clientes, o sea datos personales. Lo mismo que el resto del inbox.
type InsightHandler struct {
	svc     *insight.Service
	repo    *insight.Repository
	catalog *insight.CatalogRepository
	log     *slog.Logger
	// sse es opcional: sin hub el modulo anda igual, solo que la bandeja no
	// se entera sola de un cambio de pedido. Lo inyecta main.go con un setter
	// para no cambiar la firma del constructor (que usan los tests).
	sse *sse.Hub
}

// SetSSE conecta el hub de tiempo real al handler de analisis.
func (h *InsightHandler) SetSSE(hub *sse.Hub) { h.sse = hub }

func NewInsightHandler(svc *insight.Service, repo *insight.Repository, catalog *insight.CatalogRepository, log *slog.Logger) *InsightHandler {
	if log == nil {
		log = slog.Default()
	}
	return &InsightHandler{svc: svc, repo: repo, catalog: catalog, log: log}
}

// fail es el camino de error de los 500 de este handler.
//
// Existe porque un `return c.JSON(500, ...)` sin log es un agujero negro: el
// middleware de Echo registra el 500 pero no el motivo, y un desajuste entre
// el numero de columnas del SELECT y el del Scan aparece como "no se pudo
// leer el pedido" sin ninguna pista. Con esta linea, el error real queda en
// el log del contenedor.
func (h *InsightHandler) fail(c echo.Context, op string, err error) error {
	h.log.Error("insight: "+op,
		"path", c.Request().URL.Path,
		"err", err)
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": op})
}

// StatusResponse es lo que la UI necesita para el cartel de arriba:
// si el modelo esta, si procesa, y que fallo ultimamente.
type StatusResponse struct {
	Enabled       bool                    `json:"enabled"`
	MasterEnabled bool                    `json:"master_enabled"`
	ASREnabled    bool                    `json:"asr_enabled"`
	Model         string                  `json:"model"`
	ModelReady    bool                    `json:"model_ready"`
	ModelError    string                  `json:"model_error,omitempty"`
	ASRModel      string                  `json:"asr_model"`
	Channels      []insight.InsightConfig `json:"channels"`
	Counts        insight.AnalysisCounts  `json:"counts"`
	Queues        insight.QueueDepth      `json:"queues"`
	RecentErrors  []insight.AnalysisRow   `json:"recent_errors"`
}

// Status GET /api/insights/status
func (h *InsightHandler) Status(c echo.Context) error {
	ctx := c.Request().Context()
	rep, err := h.svc.Status(ctx)
	if err != nil {
		return h.fail(c, "no se pudo leer el estado del analizador", err)
	}

	channels, err := h.repo.ListConfigs(ctx, []string{"whatsapp", "instagram", "facebook"})
	if err != nil {
		return h.fail(c, "no se pudo leer la configuración", err)
	}

	return c.JSON(http.StatusOK, StatusResponse{
		Enabled:       rep.MasterEnabled,
		MasterEnabled: rep.MasterEnabled,
		ASREnabled:    rep.ASREnabled,
		Model:         rep.Model,
		ModelReady:    rep.ModelReady,
		ModelError:    rep.OllamaError,
		ASRModel:      rep.ASRModel,
		Channels:      channels,
		Counts:        rep.Counts,
		Queues:        rep.Queues,
		RecentErrors:  rep.RecentErrors,
	})
}

// MessageAnalysisResponse es el analisis de UN mensaje, para la burbuja.
type MessageAnalysisResponse struct {
	Found       bool              `json:"found"`
	MessageID   string            `json:"message_id,omitempty"`
	Intent      *string           `json:"intent,omitempty"`
	Resumen     *string           `json:"resumen,omitempty"`
	Productos   []string          `json:"productos"`
	Cantidades  []int             `json:"cantidades"`
	Detalles    map[string]string `json:"detalles"`
	Confianza   *float64          `json:"confianza,omitempty"`
	NeedsReview bool              `json:"needs_review"`
	Status      string            `json:"status,omitempty"`
	SkipReason  *string           `json:"skip_reason,omitempty"`
	Error       *string           `json:"error,omitempty"`
	Model       *string           `json:"model,omitempty"`
	LatencyMS   *int              `json:"latency_ms,omitempty"`
	ASRText     *string           `json:"asr_text,omitempty"`
	ASRMS       *int              `json:"asr_ms,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
}

// GetMessageAnalysis GET /api/insights/messages/:id
//
// Devuelve 200 con found=false en vez de 404: la UI lo consulta para CADA
// burbuja y un 404 por cada mensaje sin analizar seria ruido.
func (h *InsightHandler) GetMessageAnalysis(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de mensaje inválido"})
	}

	row, err := h.repo.GetAnalysisByMessage(ctx, id)
	if err != nil {
		return h.fail(c, "no se pudo leer el análisis", err)
	}
	if row == nil {
		return c.JSON(http.StatusOK, MessageAnalysisResponse{
			Found:      false,
			Productos:  []string{},
			Cantidades: []int{},
			Detalles:   map[string]string{},
		})
	}
	return c.JSON(http.StatusOK, MessageAnalysisResponse{
		Found:       true,
		MessageID:   row.MessageID.String(),
		Intent:      row.Intent,
		Resumen:     row.Resumen,
		Productos:   emptyStrings(row.Productos),
		Cantidades:  emptyInts(row.Cantidades),
		Detalles:    stringDetails(row.Detalles),
		Confianza:   row.Confianza,
		NeedsReview: row.NeedsReview,
		Status:      row.Status,
		SkipReason:  row.SkipReason,
		Error:       row.Error,
		Model:       row.Model,
		LatencyMS:   row.LatencyMS,
		ASRText:     row.ASRText,
		ASRMS:       row.ASRMS,
		CreatedAt:   row.CreatedAt.Format(timeLayout),
	})
}

// ConversationInsightResponse junta el pedido consolidado + el historial por
// mensaje en UNA respuesta: la pantalla del hilo pide las dos cosas juntas.
//
// Revisions es el historial del pedido (solo si hay mas de una) y Pending es el
// analysis que todavia no llego al pedido vigente. Van en la misma respuesta a
// proposito: el banner, la tarjeta y el timeline se pintan con UNA request, y
// ningun estado de la UI depende de una segunda que pueda fallar sola.
type ConversationInsightResponse struct {
	Order     *insight.OrderView    `json:"order"`
	Analyses  []insight.AnalysisRow `json:"analyses"`
	Revisions []insight.OrderView   `json:"revisions,omitempty"`
	Pending   *insight.AnalysisRow  `json:"pending"`
	Enabled   bool                  `json:"enabled"`
	OffBy     string                `json:"off_by,omitempty"`
}

// GetConversation GET /api/insights/conversation/:id
func (h *InsightHandler) GetConversation(c echo.Context) error {
	ctx := c.Request().Context()
	convID, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de conversación inválido"})
	}

	enabled, offBy, err := h.repo.EffectiveEnabled(ctx, convID)
	if err != nil {
		return h.fail(c, "no se pudo leer el estado del hilo", err)
	}

	order, err := h.repo.GetOrderByConversationView(ctx, convID)
	if err != nil {
		return h.fail(c, "no se pudo leer el pedido", err)
	}

	analyses, err := h.repo.ListByConversation(ctx, convID)
	if err != nil {
		return h.fail(c, "no se pudo leer el historial", err)
	}
	if analyses == nil {
		analyses = []insight.AnalysisRow{}
	}

	// El historial de revisiones solo se consulta si hay algo que mostrar: con
	// una sola revision la consulta es un wasted round trip en cada apertura
	// de hilo, que es la pantalla mas usada del producto.
	var revisions []insight.OrderView
	if order != nil {
		all, err := h.repo.ListRevisions(ctx, convID)
		if err != nil {
			return h.fail(c, "no se pudo leer el historial del pedido", err)
		}
		if len(all) > 1 {
			revisions = all
		}
	}

	pending, err := h.repo.PendingAnalysis(ctx, convID)
	if err != nil {
		return h.fail(c, "no se pudo leer el pedido pendiente", err)
	}

	return c.JSON(http.StatusOK, ConversationInsightResponse{
		Order:     order,
		Analyses:  analyses,
		Revisions: revisions,
		Pending:   pending,
		Enabled:   enabled,
		OffBy:     offBy,
	})
}

// UpdateSettings PATCH /api/insights/settings/:key  body: {"enabled": bool}
//
// Solo se aceptan las dos claves conocidas. Una key arbitraria en app_settings
// es basura silenciosa: nadie la va a leer nunca.
func (h *InsightHandler) UpdateSettings(c echo.Context) error {
	ctx := c.Request().Context()
	key := c.Param("key")
	if key != insight.SettingMasterEnabled && key != insight.SettingASREnabled {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "setting desconocido"})
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}

	userID := currentUserID(c)
	if err := h.repo.SetBoolSetting(ctx, key, req.Enabled, userID); err != nil {
		return h.fail(c, "no se pudo guardar el ajuste", err)
	}
	h.svc.InvalidateCache()
	return c.JSON(http.StatusOK, map[string]any{"key": key, "enabled": req.Enabled})
}

// UpdateChannelConfig PATCH /api/insights/config/:channel
func (h *InsightHandler) UpdateChannelConfig(c echo.Context) error {
	ctx := c.Request().Context()
	channel := normalizeChannel(c.Param("channel"))
	if channel == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "canal inválido"})
	}

	var req struct {
		Enabled         *bool    `json:"enabled"`
		ASREnabled      *bool    `json:"asr_enabled"`
		Model           *string  `json:"model"`
		SystemPrompt    *string  `json:"system_prompt"`
		Temperature     *float64 `json:"temperature"`
		ContextMessages *int     `json:"context_messages"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}

	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "temperature debe estar entre 0 y 2"})
	}
	if req.ContextMessages != nil && (*req.ContextMessages < 0 || *req.ContextMessages > 50) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "context_messages debe estar entre 0 y 50"})
	}
	if req.Model != nil && *req.Model != "" && strings.ContainsAny(*req.Model, " \t") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "modelo inválido"})
	}

	cfg, err := h.repo.UpdateConfig(ctx, channel, insight.InsightConfigPatch{
		Enabled:         req.Enabled,
		ASREnabled:      req.ASREnabled,
		Model:           req.Model,
		SystemPrompt:    req.SystemPrompt,
		Temperature:     req.Temperature,
		ContextMessages: req.ContextMessages,
	})
	if err != nil {
		return h.fail(c, "no se pudo guardar la configuración", err)
	}
	h.svc.InvalidateCache()
	return c.JSON(http.StatusOK, cfg)
}

// Backfill POST /api/insights/backfill  body: {"limit"?: 200}
//
// A proposito NO corre solo al arrancar: analizar 3.000 mensajes historicos de
// golpe deja el equipo sin CPU y tarda horas. El operador lo dispara y ve el
// progreso en /status.
func (h *InsightHandler) Backfill(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		Limit int `json:"limit"`
	}
	if err := c.Bind(&req); err != nil {
		req.Limit = 200
	}
	if req.Limit < 1 {
		req.Limit = 1
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	items, err := h.repo.ListUnanalyzed(ctx, req.Limit)
	if err != nil {
		return h.fail(c, "no se pudo buscar mensajes sin analizar", err)
	}

	queued := 0
	skipped := 0
	for _, it := range items {
		text := ""
		if it.Text != nil {
			text = *it.Text
		}
		hasAudio := len(it.Attachments) > 0 && strings.Contains(string(it.Attachments), `"audio"`)

		// Respetar el interruptor de la conversacion: backfill no es una
		// puerta trasera para analizar hilos que el operador apago.
		enabled, _, err := h.repo.EffectiveEnabled(ctx, it.ConversationID)
		if err != nil || !enabled {
			skipped++
			continue
		}

		h.svc.EnqueueBackfill(it.MessageID, it.ConversationID, it.Channel, text, it.Attachments, hasAudio)
		queued++
	}

	return c.JSON(http.StatusOK, map[string]any{
		"encontrados": len(items),
		"encolados":   queued,
		"omitidos":    skipped,
	})
}

// ---------------------------------------------------------------------------
// Pedidos
// ---------------------------------------------------------------------------

// ListOrders GET /api/orders?status=&intent=&q=&limit=&offset=
func (h *InsightHandler) ListOrders(c echo.Context) error {
	ctx := c.Request().Context()
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	orders, total, err := h.repo.ListOrders(ctx, insight.OrderFilter{
		Status: c.QueryParam("status"),
		Intent: c.QueryParam("intent"),
		Search: c.QueryParam("q"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return h.fail(c, "no se pudieron listar los pedidos", err)
	}
	if orders == nil {
		orders = []insight.OrderView{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"orders": orders,
		"total":  total,
		"limit":  orDefault(limit, 50),
		"offset": offset,
	})
}

// validOrderStatuses son los del CHECK de la DB. El mismo conjunto, validado
// aca, evita un 500 por un valor mal tipeado en la UI.
var validOrderStatuses = map[string]bool{
	"nuevo": true, "confirmado": true, "entregado": true, "descartado": true,
}

// GetOrder GET /api/orders/:id
func (h *InsightHandler) GetOrder(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de pedido inválido"})
	}
	order, err := h.repo.GetOrderByID(ctx, id)
	if err != nil {
		return h.fail(c, "no se pudo leer el pedido", err)
	}
	if order == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "pedido no encontrado"})
	}
	return c.JSON(http.StatusOK, order)
}

// UpdateOrder PATCH /api/orders/:id
//
// Guardar aqui marca edited=true: es la forma de que la IA deje de pisar este
// pedido. Por eso es irreversible desde la API y la UI lo dice.
func (h *InsightHandler) UpdateOrder(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de pedido inválido"})
	}

	var req struct {
		Intent      *string           `json:"intent"`
		Resumen     *string           `json:"resumen"`
		Productos   []string          `json:"productos"`
		Cantidades  []int             `json:"cantidades"`
		Detalles    map[string]string `json:"detalles"`
		Status      *string           `json:"status"`
		NeedsReview *bool             `json:"needs_review"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}

	if req.Intent != nil && !insightValidIntent(*req.Intent) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "intent inválido"})
	}
	if req.Status != nil && !validOrderStatuses[*req.Status] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "status inválido"})
	}
	if req.Resumen != nil && len(*req.Resumen) > 500 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "el resumen es demasiado largo"})
	}
	for k := range req.Detalles {
		if !insightValidDetail(k) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "detalle desconocido: " + k})
		}
	}

	order, err := h.repo.UpdateOrderByOperator(ctx, id, insight.OperatorOrderPatch{
		Intent:      req.Intent,
		Resumen:     req.Resumen,
		Productos:   req.Productos,
		Cantidades:  req.Cantidades,
		Detalles:    req.Detalles,
		Status:      req.Status,
		NeedsReview: req.NeedsReview,
	}, currentUserID(c))
	if errors.Is(err, insight.ErrNotFound) {
		// O no existe, o es una revision que ya fue reemplazada: en los dos
		// casos no hay un pedido editable con ese id.
		return c.JSON(http.StatusNotFound, map[string]string{"error": "pedido no encontrado"})
	}
	if err != nil {
		return h.fail(c, "no se pudo guardar el pedido", err)
	}
	if order == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "pedido no encontrado"})
	}

	// Se devuelve la vista completa y no el UPDATE: el operador necesita el id
	// y el status para seguir editando, y el RETURNING del UPDATE no los trae.
	// Una query extra en una accion humana no es nada comparado con devolver
	// un objeto que rompe el formulario al segundo click.
	view, err := h.repo.GetOrderByConversationView(ctx, order.ConversationID)
	if err != nil || view == nil {
		return c.JSON(http.StatusOK, order)
	}
	return c.JSON(http.StatusOK, view)
}

// SetOrderStatus PATCH /api/orders/:id/status
func (h *InsightHandler) SetOrderStatus(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de pedido inválido"})
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}
	if !validOrderStatuses[req.Status] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "status inválido"})
	}

	if err := h.repo.SetOrderStatus(ctx, id, req.Status); err != nil {
		if errors.Is(err, insight.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "pedido no encontrado"})
		}
		return h.fail(c, "no se pudo cambiar el estado", err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": req.Status})
}

// AcceptPending POST /api/orders/:id/accept-pending
//
// El pedido nuevo del cliente pasa a ser la revision vigente y el anterior queda
// en el historial. Es una accion de humano: la regla dura #9 sigue intacta
// porque la fila vieja queda sellada, solo deja de ser la vigente.
//
// 409 si no hay nada pendiente: el boton no deberia estar habilitado, pero si
// llega (doble click, dos pestañas) tiene que decir por que no hizo nada.
func (h *InsightHandler) AcceptPending(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de pedido inválido"})
	}

	view, err := h.repo.AcceptPending(ctx, id, currentUserID(c))
	switch {
	case errors.Is(err, insight.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "pedido no encontrado"})
	case errors.Is(err, insight.ErrNoPending):
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "no hay un pedido del cliente posterior a este pedido"})
	case err != nil:
		return h.fail(c, "no se pudo aplicar el pedido pendiente", err)
	}

	if err := h.broadcastOrder(view); err != nil {
		return h.fail(c, "se guardo pero no se pudo avisar a la bandeja", err)
	}
	return c.JSON(http.StatusOK, view)
}

// ReopenOrder POST /api/orders/:id/reopen
//
// Saca el sello de edicion para que la IA vuelva a consolidar sobre ESTA
// revision, sin crear una nueva. Es la respuesta a "mi correccion estaba bien,
// solo se dejo de actualizar".
//
// A diferencia de aplicar, si hay un analysis pendiente lo re-consolida sobre la
// misma fila con la misma MergeOrder del worker, asi que reabrir no tira el
// pedido que el cliente ya habia mandado.
func (h *InsightHandler) ReopenOrder(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id de pedido inválido"})
	}

	view, err := h.repo.ReopenOrder(ctx, id)
	switch {
	case errors.Is(err, insight.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "pedido no encontrado"})
	case errors.Is(err, insight.ErrNotEdited):
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "el pedido no está editado: no hay nada que reabrir"})
	case err != nil:
		return h.fail(c, "no se pudo reabrir el pedido", err)
	}

	if err := h.broadcastOrder(view); err != nil {
		return h.fail(c, "se guardo pero no se pudo avisar a la bandeja", err)
	}
	return c.JSON(http.StatusOK, view)
}

// broadcastOrder avisa a la bandeja y al hilo abierto que el pedido cambio, para
// que el badge y la tarjeta no queden desactualizados hasta el proximo refetch.
//
// El mismo tipo de evento que emite el worker (insight.updated con "order"), con
// message_id en null porque aca el cambio lo hizo un humano y no viene de un
// mensaje: el frontend no lo usa para patchear un analysis.
func (h *InsightHandler) broadcastOrder(view *insight.OrderView) error {
	if view == nil || h.sse == nil {
		return nil
	}
	h.sse.Broadcast(sse.Event{
		Type: "insight.updated",
		Data: map[string]any{
			"conversation_id": view.ConversationID,
			"message_id":      nil,
			"channel":         view.Channel,
			"order":           view,
		},
	})
	return nil
}

// ---------------------------------------------------------------------------
// Catalogo
// ---------------------------------------------------------------------------

// ListCatalog GET /api/catalog?all=true
func (h *InsightHandler) ListCatalog(c echo.Context) error {
	ctx := c.Request().Context()
	onlyActive := c.QueryParam("all") != "true"

	items, err := h.catalog.List(ctx, onlyActive)
	if err != nil {
		return h.fail(c, "no se pudo leer el catálogo", err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

// CreateCatalogItem POST /api/catalog
func (h *InsightHandler) CreateCatalogItem(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		Name      string   `json:"name"`
		Category  string   `json:"category"`
		Aliases   []string `json:"aliases"`
		Active    *bool    `json:"active"`
		SortOrder *int     `json:"sort_order"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "el nombre es obligatorio"})
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}
	sortOrder := 100
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	item, err := h.catalog.Create(ctx, insight.CatalogItem{
		Name:     req.Name,
		Category: req.Category,
		Aliases:  req.Aliases,
	}, active, sortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") ||
			strings.Contains(err.Error(), "already exists") {
			return c.JSON(http.StatusConflict, map[string]string{"error": "ya existe un producto con ese nombre"})
		}
		return h.fail(c, "no se pudo crear el producto", err)
	}

	h.svc.InvalidateCache()
	return c.JSON(http.StatusCreated, item)
}

// UpdateCatalogItem PATCH /api/catalog/:id
func (h *InsightHandler) UpdateCatalogItem(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id inválido"})
	}

	var req struct {
		Name      *string   `json:"name"`
		Category  *string   `json:"category"`
		Aliases   *[]string `json:"aliases"`
		Active    *bool     `json:"active"`
		SortOrder *int      `json:"sort_order"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}

	item, err := h.catalog.Update(ctx, id, insight.CatalogPatch{
		Name:      req.Name,
		Category:  req.Category,
		Aliases:   req.Aliases,
		Active:    req.Active,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, insight.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "producto no encontrado"})
		}
		if strings.Contains(err.Error(), "duplicate key") {
			return c.JSON(http.StatusConflict, map[string]string{"error": "ya existe un producto con ese nombre"})
		}
		return h.fail(c, "no se pudo guardar el producto", err)
	}

	h.svc.InvalidateCache()
	return c.JSON(http.StatusOK, item)
}

// DeleteCatalogItem DELETE /api/catalog/:id
func (h *InsightHandler) DeleteCatalogItem(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id inválido"})
	}

	if err := h.catalog.Delete(ctx, id); err != nil {
		if errors.Is(err, insight.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "producto no encontrado"})
		}
		return h.fail(c, "no se pudo eliminar el producto", err)
	}

	h.svc.InvalidateCache()
	return c.JSON(http.StatusOK, map[string]string{"status": "eliminado"})
}
