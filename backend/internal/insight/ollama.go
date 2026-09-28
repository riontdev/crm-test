package insight

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ollamaSchema se pasa a Ollama como campo `format`. Hace que el servidor
// guilie la generacion con el grammar de la respuesta y devuelva JSON valido
// siempre, en vez de esperar que el modelo "se porte bien".
//
// Se escribe a mano y NO se deriva de los structs: el schema tiene que ser
// plano y estable aunque cambien los structs internos.
var ollamaSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"intent": map[string]any{
			"type": "string",
			"enum": []string{"pedido", "info", "reclamo", "otro"},
		},
		"resumen":    map[string]any{"type": "string"},
		"productos":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"cantidades": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
		// delta | total | null. MergeOrder NO deduce esto del texto: el modelo
		// lo declara y el merge lo obedece. Ver MergeOrder y la migracion 000016.
		"tipo_cantidad":   nullableTipoCantidad,
		"material":        nullableString,
		"medidas":         nullableString,
		"personalizacion": nullableString,
		"fecha_entrega":   nullableString,
		"confianza":       map[string]any{"type": "number"},
	},
	"required": []string{"intent", "resumen", "productos", "cantidades", "tipo_cantidad", "confianza"},
}

var nullableString = map[string]any{"type": []string{"string", "null"}}

// nullableTipoCantidad admite el enum y null. null es legitimo y frecuente:
// significa "este mensaje no trae numeros de piezas".
var nullableTipoCantidad = map[string]any{
	"enum": []any{"delta", "total", nil},
}

// Analysis es lo que devuelve el modelo. Es un subset de message_analyses:
// lo que no viene, se guarda como null y la UI lo oculta.
type Analysis struct {
	Intent     string   `json:"intent"`
	Resumen    string   `json:"resumen"`
	Productos  []string `json:"productos"`
	Cantidades []int    `json:"cantidades"`
	// TipoCantidad declara si Cantidades es un incremento ("delta") o el total
	// ("total"). NULL = este mensaje no trae numeros de piezas.
	//
	// No es un detalle del prompt: es lo que decide si MergeOrder suma o
	// reemplaza, y por eso va tipado y no inferred. Ver MergeOrder.
	TipoCantidad    *string `json:"tipo_cantidad"`
	Material        *string `json:"material"`
	Medidas         *string `json:"medidas"`
	Personalizacion *string `json:"personalizacion"`
	FechaEntrega    *string `json:"fecha_entrega"`
	Confianza       float64 `json:"confianza"`
}

// TipoDelta y TipoTotal son los unicos valores que acepta la DB (CHECK en la
// migracion 000016).
const (
	TipoDelta = "delta"
	TipoTotal = "total"
)

// normalizaTipoCantidad deja el valor en uno de los dos valores validos.
// Cualquier otra cosa (invetido por el modelo, vacio, con espacios) se vuelve
// nil, que MergeOrder trata como "no se": no toca el pedido y marca revision.
func (a *Analysis) normalizaTipoCantidad() *string {
	if a.TipoCantidad == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(*a.TipoCantidad)) {
	case TipoDelta:
		v := TipoDelta
		return &v
	case TipoTotal:
		v := TipoTotal
		return &v
	default:
		return nil
	}
}

// validIntents es el unico vocabulary que la DB acepta (CHECK constraint).
// Si el modelo inventa uno, caemos a "otro" en vez de romper el INSERT.
var validIntents = map[string]bool{
	"pedido": true, "info": true, "reclamo": true, "otro": true,
}

// OllamaClient habla con la API local de Ollama. Solo hace POST /api/chat:
// este paquete no tiene forma de enviar nada a un cliente.
type OllamaClient struct {
	baseURL string
	model   string
	http    *http.Client
	log     *slog.Logger
}

// NewOllamaClient devuelve un cliente. timeout_http va en la request: si el
// modelo se cuelga, el worker no se queda bloqueado para siempre.
func NewOllamaClient(baseURL, model string, timeoutHTTP time.Duration, log *slog.Logger) *OllamaClient {
	if strings.HasSuffix(baseURL, "/") {
		baseURL = strings.TrimRight(baseURL, "/")
	}
	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: timeoutHTTP},
		log:     log,
	}
}

func (c *OllamaClient) Model() string { return c.model }

// ChatResult envuelve el analisis con la metadata que se persiste.
type ChatResult struct {
	Analysis  *Analysis
	Raw       json.RawMessage
	LatencyMS int
	PromptTok int
	OutputTok int
}

func (c *OllamaClient) post(ctx context.Context, path string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	// Ollama manda el error en el body con status 4xx/5xx, asi que hay que
	// leerlo para poder distinguir "modelo inexistente" de otra cosa.
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(string(payload))
		if len(detail) > 300 {
			detail = detail[:300] + "..."
		}
		return &APIError{Status: resp.StatusCode, Body: detail}
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// Analyze llama al modelo y devuelve el analisis ya normalizado.
// system es el prompt completo (reglas + catalogo + formato) y user es el
// contexto del hilo.
func (c *OllamaClient) Analyze(ctx context.Context, system, user string) (*ChatResult, error) {
	start := time.Now()

	var out struct {
		Model           string `json:"model"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
		Message         struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	err := c.post(ctx, "/api/chat", map[string]any{
		"model":  c.model,
		"stream": false,
		// think:false es obligatorio en modelos tipo LFM/Qwen3: con thinking
		// habilitado tardan 20s+ y alucinan. Sin esto, este cliente no sirve.
		"think":  false,
		"format": ollamaSchema,
		// keep_alive largo: sin esto cada mensaje paga de nuevo la carga del
		// modelo (10-25s) y el consumo de CPU se multiplica.
		"keep_alive": "30m",
		"options": map[string]any{
			"temperature": 0,
			"num_ctx":     8192,
			"num_predict": 400,
		},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}, &out)
	if err != nil {
		return nil, err
	}

	var a Analysis
	if err := json.Unmarshal([]byte(out.Message.Content), &a); err != nil {
		return nil, &ParseError{Err: err, Content: out.Message.Content}
	}

	if !validIntents[a.Intent] {
		c.log.Warn("insight intent invalido del modelo, degrado a 'otro'",
			"intent", a.Intent, "model", c.model)
		a.Intent = "otro"
	}
	if a.Productos == nil {
		a.Productos = []string{}
	}
	if a.Cantidades == nil {
		a.Cantidades = []int{}
	}
	a.TipoCantidad = a.normalizaTipoCantidad()
	if a.Confianza < 0 {
		a.Confianza = 0
	}
	if a.Confianza > 1 {
		a.Confianza = 1
	}

	// Guarda de coherencia: declarar como total/delta sin cantidades no tiene
	// sentido y solo streamuja al merge a adivinar. Sin numeros, tipo null.
	if len(a.Cantidades) == 0 {
		a.TipoCantidad = nil
	}

	return &ChatResult{
		Analysis:  &a,
		Raw:       json.RawMessage(out.Message.Content),
		LatencyMS: int(time.Since(start).Milliseconds()),
		PromptTok: out.PromptEvalCount,
		OutputTok: out.EvalCount,
	}, nil
}

// Ping comprueba que el servidor este vivo y que el modelo este descargado.
// Lo usa /api/insights/status para no tener que adivinar el estado.
func (c *OllamaClient) Ping(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return false, err
	}
	for _, m := range tags.Models {
		if m.Name == c.model {
			return true, nil
		}
	}
	return false, nil
}

// APIError es un 4xx/5xx de Ollama. El cuerpo va incluido porque ahi esta el
// motivo real ("model not found", "context length exceeded", ...).
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ollama %d: %s", e.Status, e.Body)
}

// Retryable decide si vale la pena reintentar. Un 400 (prompt roto, modelo
// inexistente) no mejora solo; un 500 o un timeout casi siempre mejora.
func (e *APIError) Retryable() bool {
	return e.Status >= 500 || e.Status == http.StatusTooManyRequests
}

// ParseError es JSON invalido. Con `format` puesto deberia ser imposible, pero
// si el grammar se rompe preferimos registrarlo y seguir que tumbar la cola.
type ParseError struct {
	Err     error
	Content string
}

func (e *ParseError) Error() string {
	short := e.Content
	if len(short) > 200 {
		short = short[:200] + "..."
	}
	return fmt.Sprintf("respuesta no es JSON: %v (%q)", e.Err, short)
}

func (e *ParseError) Unwrap() error { return e.Err }

// Retryable: un JSON invalido casi siempre es culpa del modelo, no del
// servicio, asi que NO se reintenta (repetir da el mismo resultado).
func (e *ParseError) Retryable() bool { return false }
