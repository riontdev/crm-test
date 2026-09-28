package insight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/riont/crm/backend/internal/asr"
	"github.com/riont/crm/backend/internal/sse"
)

// Tamanos de cola. No son "cuanto se procesa" sino "cuanto se tolera perder si
// el backend se cae": como todo queda en message_analyses, lo que no entra en
// la memoria se recupera al arrancar (recover). 128 es ~2 dias de mensajes.
const (
	TextQueueSize  = 128
	AudioQueueSize = 24
)

// MaxAttempts es cuantos intentos antes de dejar el mensaje en 'error' para
// siempre. 3 es el punto donde insistir mas es ruido: si Ollama no levanta en
// 3 intentos, no va a levantar en el cuarto.
const MaxAttempts = 3

// retryBackoff: el primer reintento va rapido (un 500 de Ollama se recupera en
// segundos), el segundo espera bastante mas.
var retryBackoff = []time.Duration{10 * time.Second, 90 * time.Second}

// Job es un mensaje en cola.
type Job struct {
	MessageID      uuid.UUID
	ConversationID uuid.UUID
	Channel        string
	Text           string
	Attachments    json.RawMessage
	Attempt        int
	// ASRText ya transcrito: evita volver a pasar el audio por Whisper cuando
	// el reintento es del analisis, no de la transcripcion.
	ASRText string
	// ASRID es la fila de message_analyses que reservo el worker de AUDIO.
	// Sin esto el worker de texto haria un segundo Claim, la fila ya existe,
	// ON CONFLICT devuelve false y la nota de voz se transcribe pero nunca se
	// analiza. Con ASRID se reutiliza la fila que el audio ya reservo.
	ASRID uuid.UUID
}

// Service es el motor del analisis: dos colas, dos workers, cero acoplamiento
// con el camino critico del webhook.
//
// Division del trabajo (y el motivo): el analisis de texto es de bajo costo y
// el operador lo quiere ver ya, asi que tiene su propia cola. La transcripcion
// es de costo alto y de valor diferido (una nota de voz se entiende leyendo el
// texto UN minuto despues igual), asi que va en una cola aparte que nunca le
// roba CPU al analisis.
type Service struct {
	repo    *Repository
	catalog *CatalogRepository
	llm     *OllamaClient
	asr     *asr.Client
	fetcher *asr.Fetcher
	sse     *sse.Hub
	log     *slog.Logger

	textQ  chan Job
	audioQ chan Job
	wg     sync.WaitGroup

	textWorkers int

	modelReady atomic.Bool
	lastErr    atomic.Value // string

	// cacheTTL evita leer catalogo y configs de la DB en cada mensaje. 60s es
	// invisible para el operador y le saca 2 queries a la cola de texto.
	cacheMu      sync.Mutex
	catalogCache []CatalogItem
	catalogAt    time.Time
	configCache  map[string]*InsightConfig
}

type ServiceDeps struct {
	Repo        *Repository
	Catalog     *CatalogRepository
	LLM         *OllamaClient
	ASR         *asr.Client
	Fetcher     *asr.Fetcher
	SSE         *sse.Hub
	Logger      *slog.Logger
	TextWorkers int
}

func NewService(d ServiceDeps) *Service {
	if d.TextWorkers < 1 {
		d.TextWorkers = 1
	}
	return &Service{
		repo:        d.Repo,
		catalog:     d.Catalog,
		llm:         d.LLM,
		asr:         d.ASR,
		fetcher:     d.Fetcher,
		sse:         d.SSE,
		log:         d.Logger,
		textQ:       make(chan Job, TextQueueSize),
		audioQ:      make(chan Job, AudioQueueSize),
		textWorkers: d.TextWorkers,
		configCache: map[string]*InsightConfig{},
	}
}

// Start levanta los workers y recupera lo que quedo a medias en el arranque.
func (s *Service) Start(ctx context.Context) {
	for i := 0; i < s.textWorkers; i++ {
		s.wg.Add(1)
		go s.textLoop(ctx, i)
	}
	s.wg.Add(1)
	go s.audioLoop(ctx)

	s.wg.Add(1)
	go s.healthLoop(ctx)

	go s.recover(ctx)
	s.log.Info("insight: workers arrancados",
		"text_workers", s.textWorkers, "text_queue", TextQueueSize,
		"audio_queue", AudioQueueSize, "model", s.llm.Model())
}

// Stop espera a que terminen los workers. Se usa en el apagado ordenado.
func (s *Service) Stop() {
	s.wg.Wait()
}

// recover reencola lo que quedo pendiente. Los 'processing' son trabajos que el
// proceso anterior dejo a medias al morir; los 'pending' son reintentos
// diferidos o mensajes que nunca他的人.
func (s *Service) recover(ctx context.Context) {
	n, err := s.repo.ResetStale(ctx, StaleThreshold)
	if err != nil {
		s.log.Warn("insight: no se pudieron recuperar los jobs processing", "err", err)
	} else if n > 0 {
		s.log.Info("insight: jobs processing liberados al reiniciar", "count", n)
	}

	items, err := s.repo.ListRecoverable(ctx, 200)
	if err != nil {
		s.log.Warn("insight: no se pudo listar la cola recuperable", "err", err)
		return
	}
	if len(items) == 0 {
		return
	}

	enq := 0
	for _, it := range items {
		job := jobFromRecover(it)
		if s.hasAudio(it.Attachments) && (it.Text == nil || strings.TrimSpace(*it.Text) == "") {
			if s.enqueueAudio(job) {
				enq++
			}
			continue
		}
		if s.enqueueText(job) {
			enq++
		}
	}
	s.log.Info("insight: cola recuperada", "enqueued", enq, "candidatos", len(items))
}

func jobFromRecover(it RecoverItem) Job {
	text := ""
	if it.Text != nil {
		text = *it.Text
	}
	return Job{
		MessageID:      it.MessageID,
		ConversationID: it.ConversationID,
		Channel:        it.Channel,
		Text:           text,
		Attachments:    it.Attachments,
		Attempt:        it.Attempts,
	}
}

// EnqueueAfterMessage es el punto de entrada desde el webhook.
//
// NUNCA bloquea ni falla el request: si la cola esta llena se loguea y se
// pierde el job (queda recuperable desde message_analyses, asi que no se pierde
// el mensaje, solo se posterga el analisis). La regla dura 2 del AGENTS.md
// manda sobre cualquier Vanguardia de esta funcion.
func (s *Service) EnqueueAfterMessage(_ context.Context, msgID, convID uuid.UUID, channel, text string, attachments json.RawMessage) {
	job := Job{
		MessageID: msgID, ConversationID: convID, Channel: channel,
		Text: strings.TrimSpace(text), Attachments: attachments,
	}

	// Texto primero: si hay mensaje escrito, no se gasta CPU en transcribir.
	if job.Text != "" {
		s.enqueueText(job)
		return
	}
	if s.hasAudio(attachments) {
		s.enqueueAudio(job)
		return
	}
	// Mensaje entrante sin texto y sin audio (un archivo, una foto): no hay
	// nada que analizar y no se ensucia la DB con filas vacias.
	s.log.Debug("insight: mensaje sin texto ni audio, no se encola",
		"message_id", msgID, "conversation_id", convID)
}

func (s *Service) enqueueText(job Job) bool {
	select {
	case s.textQ <- job:
		return true
	default:
		s.log.Warn("insight: cola de texto llena, se posterga el analisis",
			"message_id", job.MessageID, "conversation_id", job.ConversationID,
			"channel", job.Channel, "queue", TextQueueSize)
		return false
	}
}

func (s *Service) enqueueAudio(job Job) bool {
	select {
	case s.audioQ <- job:
		return true
	default:
		s.log.Warn("insight: cola de audio llena, se posterga la transcripcion",
			"message_id", job.MessageID, "conversation_id", job.ConversationID,
			"channel", job.Channel, "queue", AudioQueueSize)
		return false
	}
}

func (s *Service) hasAudio(attachments json.RawMessage) bool {
	return asr.HasAudio(attachments)
}

// ---------------------------------------------------------------------------
// Worker de texto
// ---------------------------------------------------------------------------

func (s *Service) textLoop(ctx context.Context, id int) {
	defer s.wg.Done()
	log := s.log.With("worker", fmt.Sprintf("text-%d", id))
	for {
		select {
		case <-ctx.Done():
			log.Info("insight: worker de texto cerrando")
			return
		case job := <-s.textQ:
			s.processText(ctx, job, log)
		}
	}
}

func (s *Service) processText(ctx context.Context, job Job, log *slog.Logger) {
	logCtx := log.With(
		"message_id", job.MessageID,
		"conversation_id", job.ConversationID,
		"channel", job.Channel,
	)

	// El interruptor se mira OTRA vez aca, no solo al encolar: el operador
	// puede haber apagado el canal mientras el job esperaba.
	enabled, offBy, err := s.repo.EffectiveEnabled(ctx, job.ConversationID)
	if err != nil {
		logCtx.Warn("insight: no se pudo leer el interruptor, se procesa igual", "err", err)
	} else if !enabled {
		logCtx.Info("insight: analisis desactivado para este hilo", "apagado_por", offBy)
		return
	}

	cfg, err := s.configFor(ctx, job.Channel)
	if err != nil {
		logCtx.Warn("insight: sin config del canal, se usan los defaults", "err", err)
		cfg = &InsightConfig{Channel: job.Channel, ContextMessages: 10}
	}

	// Si el mensaje venia de la cola de AUDIO, la fila de message_analyses ya
	// existe (la reservo processAudio) y esta EN PROCESAMIENTO, no terminada.
	// Claim haria ON CONFLICT DO NOTHING y devolveria false, asi que el
	// analisis no pasaria nunca. Se verifica y se reutiliza esa fila.
	analysisID := job.ASRID
	if analysisID != uuid.Nil {
		ok, err := s.repo.ClaimExisting(ctx, analysisID)
		if err != nil {
			logCtx.Error("insight: fallo al recuperar el analisis de audio", "err", err)
			return
		}
		if !ok {
			logCtx.Debug("insight: el analisis de audio ya estaba terminado")
			return
		}
	} else {
		id, claimed, err := s.repo.Claim(ctx, job.MessageID, job.ConversationID)
		if err != nil {
			logCtx.Error("insight: fallo al reservar el analisis", "err", err)
			return
		}
		if !claimed {
			logCtx.Debug("insight: mensaje ya reservado por otro worker")
			return
		}
		analysisID = id
	}

	// Sin texto utilizable no se gasta CPU ni se llama al modelo.
	text := job.Text
	if text == "" {
		text = job.ASRText
	}
	if strings.TrimSpace(text) == "" {
		s.repo.MarkSkipped(ctx, analysisID, "sin texto para analizar")
		logCtx.Info("insight: mensaje sin texto util, skipped")
		return
	}

	transcript, err := s.repo.Transcript(ctx, job.ConversationID, job.MessageID, cfg.ContextMessages)
	if err != nil {
		logCtx.Warn("insight: no se pudo leer el contexto, se sigue sin el", "err", err)
	}

	catalog, err := s.catalogItems(ctx)
	if err != nil {
		logCtx.Warn("insight: sin catalogo, se sigue sin catalogo", "err", err)
	}

	system := BuildSystemPrompt(catalog, derefString(cfg.SystemPrompt))
	user := BuildUserPrompt(transcript, text)

	res, err := s.llm.Analyze(ctx, system, user)
	if err != nil {
		s.handleTextError(ctx, logCtx, analysisID, job, err)
		return
	}

	// Enriquecimiento determinista: recupera el material del resumen cuando el
	// modelo lo dejo vacio (medido: pasa de 80% a ~95% en el bench).
	// texto, no el resumen: el guardarraíl de tipo_cantidad necesita las
	// palabras del cliente, no las del modelo.
	Enrich(res.Analysis, catalog, text)

	detalles := detailsFrom(res.Analysis)
	needsReview := NeedsReview(res.Analysis)

	if err := s.repo.MarkOK(ctx, analysisID, AnalysisResult{
		Analysis:    res.Analysis,
		Detalles:    detalles,
		NeedsReview: needsReview,
		Model:       s.llm.Model(),
		LatencyMS:   res.LatencyMS,
		Raw:         res.Raw,
		ASRText:     optionalString(job.ASRText),
		ASRModel:    optionalString(s.asrModel()),
	}); err != nil {
		logCtx.Error("insight: fallo al guardar el analisis", "err", err)
		return
	}

	// Consolidado del hilo. MergeOrder resuelve el conflicto edited=true.
	s.persistOrder(ctx, job, res.Analysis, logCtx)

	logCtx.Info("insight: mensaje analizado",
		"intent", res.Analysis.Intent,
		"productos", len(res.Analysis.Productos),
		"needs_review", needsReview,
		"latency_ms", res.LatencyMS,
		"prompt_tok", res.PromptTok,
		"out_tok", res.OutputTok,
		"model", s.llm.Model(),
	)
	s.broadcastAnalysis(job, res.Analysis, detalles, needsReview, res.LatencyMS)
}

// handleTextError clasifica el fallo, decide si reintenta y deja registro.
func (s *Service) handleTextError(ctx context.Context, log *slog.Logger, analysisID uuid.UUID, job Job, cause error) {
	attempt := job.Attempt + 1

	retryable := false
	var re interface{ Retryable() bool }
	if errors.As(cause, &re) {
		retryable = re.Retryable()
	} else {
		// Errores de red genericos (DNS, conexion rechazada, EOF) son
		// transitorios aunque no vengan envueltos en un tipo nuestro.
		retryable = true
	}

	willRetry, err := s.repo.MarkError(ctx, analysisID, cause, retryable, attempt, MaxAttempts)
	if err != nil {
		log.Error("insight: fallo al registrar el error", "err", err)
		return
	}

	fields := []any{
		"err", cause.Error(),
		"kind", errorKind(cause),
		"reintentable", retryable,
		"intento", attempt,
	}
	if willRetry {
		delay := retryBackoff[len(retryBackoff)-1]
		if attempt-1 < len(retryBackoff) {
			delay = retryBackoff[attempt-1]
		}
		job.Attempt = attempt
		log.Warn("insight: fallo transitorio, se reintenta", append(fields, "en", delay.String())...)
		time.AfterFunc(delay, func() {
			if !s.enqueueText(job) {
				// La cola estaba llena: el analisis queda en 'pending' y lo
				// levanta el próximo recover. No se pierde nada.
				s.log.Warn("insight: no se pudo reencolar el reintento, queda pendiente",
					"message_id", job.MessageID, "conversation_id", job.ConversationID)
			}
		})
		return
	}

	log.Error("insight: analisis fallido", fields...)
}

func errorKind(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status >= 500 {
			return "ollama_5xx"
		}
		return "ollama_4xx"
	}
	var parseErr *ParseError
	if errors.As(err, &parseErr) {
		return "json_invalido"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "red"
}

func detailsFrom(a *Analysis) map[string]string {
	d := map[string]string{}
	if a.Material != nil && strings.TrimSpace(*a.Material) != "" {
		d["material"] = strings.TrimSpace(*a.Material)
	}
	if a.Medidas != nil && strings.TrimSpace(*a.Medidas) != "" {
		d["medidas"] = strings.TrimSpace(*a.Medidas)
	}
	if a.Personalizacion != nil && strings.TrimSpace(*a.Personalizacion) != "" {
		d["personalizacion"] = strings.TrimSpace(*a.Personalizacion)
	}
	if a.FechaEntrega != nil && strings.TrimSpace(*a.FechaEntrega) != "" {
		d["fecha_entrega"] = strings.TrimSpace(*a.FechaEntrega)
	}
	if mentionsEnvio(a.Resumen) {
		d["envio"] = "si"
	}
	return d
}

// persistOrder consolida el pedido del hilo y avisa por SSE.
//
// Solo lo hace un analysis 'pedido'. El pedido es lo que hay que fabricar y
// cobrar: un reclamo ("se me llego roto el llavero") o una pregunta ("hacen
// envios a cordoba?") pueden nombrar productos, y si se consolidaran sumarian
// lineas que el cliente no pidio. Por ahi el hilo de Jorge Mujica empezo con
// una pregunta que el modelo clasifico como pedido y le creo una linea de base
// que nadie habia pedido; clasificar bien la pregunta (desambiguaPregunta) es
// la primera mitad del arreglo, y esta compuerta es la segunda.
//
// Los analyses no-'pedido' no se pierden: quedan en el historial de la
// conversacion y se ven en la burbuja del mensaje.
//
// Si la fila esta editada por un humano, UpsertOrder devuelve false y no se
// emite evento: el operador no quiere ver su pedido pisado por un modelo.
func (s *Service) persistOrder(ctx context.Context, job Job, a *Analysis, log *slog.Logger) {
	if !Consolida(a) {
		return
	}

	prev, err := s.repo.GetOrderByConversation(ctx, job.ConversationID)
	if err != nil {
		log.Warn("insight: no se pudo leer el pedido previo", "err", err)
		prev = nil
	}
	if prev != nil && prev.Edited {
		log.Info("insight: pedido editado a mano, la IA no lo toca",
			"conversation_id", job.ConversationID)
		return
	}

	next := OrderFromAnalysis(job.ConversationID, job.MessageID, a, s.llm.Model())
	merged := MergeOrder(prev, next, a.TipoCantidad)

	updated, err := s.repo.UpsertOrder(ctx, merged)
	if err != nil {
		log.Error("insight: no se pudo guardar el pedido consolidado", "err", err)
		return
	}
	if !updated {
		log.Info("insight: el pedido fue editado en el medio, no se pisa",
			"conversation_id", job.ConversationID)
		return
	}

	if s.sse != nil {
		s.sse.Broadcast(sse.Event{
			Type: "insight.updated",
			Data: map[string]any{
				"conversation_id": job.ConversationID,
				"message_id":      job.MessageID,
				"channel":         job.Channel,
				"order": map[string]any{
					"intent":       derefString(merged.Intent),
					"resumen":      derefString(merged.Resumen),
					"productos":    merged.Productos,
					"cantidades":   merged.Cantidades,
					"detalles":     merged.Detalles,
					"confianza":    derefFloat(merged.Confianza),
					"needs_review": merged.NeedsReview,
				},
			},
		})
	}
}

func (s *Service) broadcastAnalysis(job Job, a *Analysis, detalles map[string]string, needsReview bool, latencyMS int) {
	if s.sse == nil {
		return
	}
	s.sse.Broadcast(sse.Event{
		Type: "insight.updated",
		Data: map[string]any{
			"conversation_id": job.ConversationID,
			"message_id":      job.MessageID,
			"channel":         job.Channel,
			"analysis": map[string]any{
				"intent":       a.Intent,
				"resumen":      a.Resumen,
				"productos":    a.Productos,
				"cantidades":   a.Cantidades,
				"detalles":     detalles,
				"confianza":    a.Confianza,
				"needs_review": needsReview,
				"latency_ms":   latencyMS,
			},
			"asr_text": job.ASRText,
		},
	})
}

// ---------------------------------------------------------------------------
// Worker de audio
// ---------------------------------------------------------------------------

func (s *Service) audioLoop(ctx context.Context) {
	defer s.wg.Done()
	log := s.log.With("worker", "audio")
	for {
		select {
		case <-ctx.Done():
			log.Info("insight: worker de audio cerrando")
			return
		case job := <-s.audioQ:
			s.processAudio(ctx, job, log)
		}
	}
}

func (s *Service) processAudio(ctx context.Context, job Job, log *slog.Logger) {
	logCtx := log.With(
		"message_id", job.MessageID,
		"conversation_id", job.ConversationID,
		"channel", job.Channel,
	)

	cfg, err := s.configFor(ctx, job.Channel)
	if err != nil {
		logCtx.Warn("insight: sin config del canal para ASR", "err", err)
		cfg = &InsightConfig{Channel: job.Channel, ContextMessages: 10}
	}

	asrOn, err := s.repo.GetBoolSetting(ctx, SettingASREnabled, true)
	if err != nil {
		logCtx.Warn("insight: no se pudo leer el interruptor de ASR", "err", err)
		asrOn = true
	}
	if !asrOn || !cfg.ASREnabled {
		logCtx.Info("insight: transcripcion desactivada para este canal",
			"master", asrOn, "canal", cfg.ASREnabled)
		return
	}

	if s.fetcher == nil || s.asr == nil {
		logCtx.Warn("insight: ASR no configurado (falta el servicio o la API key)")
		return
	}

	// Se reserva la fila igual que en texto: asi queda trazabilidad de que
	// hubo un audio y de que no se pudo transcribir.
	analysisID, claimed, err := s.repo.Claim(ctx, job.MessageID, job.ConversationID)
	if err != nil {
		logCtx.Error("insight: fallo al reservar el analisis de audio", "err", err)
		return
	}
	if !claimed {
		logCtx.Debug("insight: audio ya reservado")
		return
	}

	// Contexto con timeout propio: la nota de voz no puede retener al worker
	// de audio indefinidamente.
	jobCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	media, err := s.fetcher.FetchAudio(jobCtx, job.Attachments)
	if err != nil {
		s.handleAudioError(ctx, logCtx, analysisID, job, err)
		return
	}
	if media == nil {
		s.repo.MarkSkipped(ctx, analysisID, "no hay adjunto de audio")
		return
	}

	logCtx.Info("insight: audio descargado", "bytes", len(media.Audio), "mime", media.MimeType)

	res, err := s.asr.Transcribe(jobCtx, media.Audio, media.Filename, media.MimeType)
	if err != nil {
		s.handleAudioError(ctx, logCtx, analysisID, job, err)
		return
	}

	// Transcripcion vacia: el cliente mando un audio mudo o ruido. No es un
	// fallo, pero tampoco hay nada que analizar.
	if strings.TrimSpace(res.Text) == "" {
		s.repo.MarkSkipped(ctx, analysisID, "transcripcion vacia")
		logCtx.Info("insight: audio sin texto util", "ms", res.MS, "bytes", len(media.Audio))
		return
	}

	// Se guarda ANTES de analizar: si Ollama se cae, el operador igual ve la
	// transcripcion en la burbuja del audio.
	if err := s.repo.SetTranscript(ctx, analysisID, res.Text, res.MS, res.Model); err != nil {
		logCtx.Warn("insight: no se pudo guardar la transcripcion", "err", err)
	}

	if s.sse != nil {
		s.sse.Broadcast(sse.Event{
			Type: "insight.transcript",
			Data: map[string]any{
				"conversation_id": job.ConversationID,
				"message_id":      job.MessageID,
				"channel":         job.Channel,
				"text":            res.Text,
				"asr_ms":          res.MS,
				"asr_model":       res.Model,
			},
		})
	}
	logCtx.Info("insight: nota de voz transcrita",
		"ms", res.MS, "chars", len(res.Text), "model", res.Model)

	// La transcripcion entra a la cola de TEXTO: la analizan los mismos rules
	// que un mensaje escrito, no una logica duplicada. Se pasa analysisID para
	// que el worker de texto reutilice la fila en vez de crear otra.
	job.ASRText = res.Text
	job.ASRID = analysisID
	job.Attempt = 0
	if !s.enqueueText(job) {
		// La cola de texto esta llena. La fila queda en 'processing' con la
		// transcripcion guardada: recover() la encuentra al proximo reinicio.
		// Log explicito porque "funciona a veces" es como se rompe un pipeline.
		logCtx.Warn("insight: transcripcion lista pero la cola de texto esta llena",
			"message_id", job.MessageID, "analysis_id", analysisID,
			"recuperable_en", "reinicio del backend")
	}
}

func (s *Service) handleAudioError(ctx context.Context, log *slog.Logger, analysisID uuid.UUID, job Job, cause error) {
	attempt := job.Attempt + 1

	// Errores de tamano/duracion NO son reintentables ni bugs: el cliente
	// mando algo que no es una nota de voz.
	if errors.Is(cause, asr.ErrTooLarge) || errors.Is(cause, asr.ErrTooLong) {
		s.repo.MarkSkipped(ctx, analysisID, cause.Error())
		log.Info("insight: audio descartado por limite", "err", cause.Error())
		return
	}

	retryable := true
	var re interface{ Retryable() bool }
	if errors.As(cause, &re) {
		retryable = re.Retryable()
	}

	willRetry, err := s.repo.MarkError(ctx, analysisID, cause, retryable, attempt, MaxAttempts)
	if err != nil {
		log.Error("insight: fallo al registrar el error de audio", "err", err)
		return
	}

	fields := []any{"err", cause.Error(), "reintentable", retryable, "intento", attempt}
	if willRetry {
		delay := retryBackoff[len(retryBackoff)-1]
		if attempt-1 < len(retryBackoff) {
			delay = retryBackoff[attempt-1]
		}
		job.Attempt = attempt
		log.Warn("insight: fallo de ASR transitorio, se reintenta", append(fields, "en", delay.String())...)
		time.AfterFunc(delay, func() { s.enqueueAudio(job) })
		return
	}
	log.Error("insight: transcripcion fallida", fields...)
}

// ---------------------------------------------------------------------------
// Cache y health
// ---------------------------------------------------------------------------

func (s *Service) catalogItems(ctx context.Context) ([]CatalogItem, error) {
	s.cacheMu.Lock()
	if s.catalogCache != nil && time.Since(s.catalogAt) < time.Minute {
		items := s.catalogCache
		s.cacheMu.Unlock()
		return items, nil
	}
	s.cacheMu.Unlock()

	items, err := s.catalog.PromptItems(ctx)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	s.catalogCache = items
	s.catalogAt = time.Now()
	s.cacheMu.Unlock()
	return items, nil
}

func (s *Service) configFor(ctx context.Context, channel string) (*InsightConfig, error) {
	s.cacheMu.Lock()
	if cfg, ok := s.configCache[channel]; ok {
		s.cacheMu.Unlock()
		return cfg, nil
	}
	s.cacheMu.Unlock()

	cfg, err := s.repo.ConfigForChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	s.configCache[channel] = cfg
	s.cacheMu.Unlock()
	return cfg, nil
}

// InvalidateCache lo llaman los handlers cuando editan catalogo o config, para
// que el cambio se vea en el proximo mensaje y no 60 segundos despues.
func (s *Service) InvalidateCache() {
	s.cacheMu.Lock()
	s.catalogCache = nil
	s.configCache = map[string]*InsightConfig{}
	s.cacheMu.Unlock()
}

func (s *Service) asrModel() string {
	if s.asr == nil {
		return ""
	}
	return s.asr.Model()
}

// healthLoop sondea Ollama para que la UI pueda decir la verdad en vez de
// suponer. Sin esto, "el modelo no responde" y "el modelo no existe" se ven
// igual desde afuera.
func (s *Service) healthLoop(ctx context.Context) {
	defer s.wg.Done()
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()

	check := func() {
		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		ready, err := s.llm.Ping(pingCtx)
		s.modelReady.Store(ready)
		if err != nil {
			s.lastErr.Store(err.Error())
			s.log.Warn("insight: Ollama no responde", "err", err)
			return
		}
		s.lastErr.Store("")
		if !ready {
			s.lastErr.Store("modelo " + s.llm.Model() + " no descargado en Ollama")
			s.log.Warn("insight: modelo no disponible en Ollama", "model", s.llm.Model())
			return
		}
		if s.sse != nil {
			s.sse.Broadcast(sse.Event{Type: "insight.health", Data: map[string]any{"ready": true}})
		}
	}

	check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			check()
		}
	}
}

// Status del servicio: lo usa el endpoint de estado.
func (s *Service) Status(ctx context.Context) (*StatusReport, error) {
	var ollamaErr error
	if msg, _ := s.lastErr.Load().(string); msg != "" {
		ollamaErr = errors.New(msg)
	}
	return s.repo.Status(ctx, s.llm.Model(), s.asrModel(),
		s.modelReady.Load(), ollamaErr, QueueDepth{
			Text:  len(s.textQ),
			Audio: len(s.audioQ),
		})
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func optionalString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := s
	return &v
}

func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

// EnqueueBackfill manda un mensaje historico al analizador.
//
// El interruptor se mira en dos capas y no es redundancia: el handler lo
// revisa para descartar el lote entero de un jalón (barato), y el worker lo
// vuelve a mirar porque entre el chequeo y el procesamiento el operador puede
// haber apagado el canal. Una query por mensaje contra un toggle en la UI: no
// vale la pena ni el riesgo de asumir que no cambio.
func (s *Service) EnqueueBackfill(msgID, convID uuid.UUID, channel, text string, attachments json.RawMessage, hasAudio bool) {
	job := Job{
		MessageID: msgID, ConversationID: convID, Channel: channel,
		Text: strings.TrimSpace(text), Attachments: attachments,
	}
	switch {
	case job.Text != "":
		s.enqueueText(job)
	case hasAudio:
		s.enqueueAudio(job)
	}
}
