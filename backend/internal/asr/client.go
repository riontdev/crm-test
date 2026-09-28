// Package asr transcribe notas de voz. Vive FUERA de internal/zernio a
// proposito: el paquete insight (que solo analiza) importa aca para bajar y
// transcribir, y este paquete no importa zernio en ningun sentido.
//
// Esa separacion es la garantia estructural de que el analizador no puede
// responderle a un cliente ni por accidente: no tiene el cliente de Zernio.
package asr

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxAudioBytes es el tope de descarga. Una nota de voz de 20 MB son horas de
// audio; lo que pase de aca no es una nota de voz, es un archivo.
const MaxAudioBytes = 20 << 20

// MaxAudioDurationSec descarta audios larguisimos antes de gastar CPU. 5
// minutos es el tope de una nota de voz util para un pedido.
const MaxAudioDurationSec = 300

// ErrTooLarge y ErrTooLong son fallos esperables (no bugs): se loguean como
// info y el mensaje queda sin transcripcion, sin reintentar.
var (
	ErrTooLarge = errors.New("audio supera el tamano maximo")
	ErrTooLong  = errors.New("audio supera la duracion maxima")
)

// asrSegment es lo que devuelve whisper por tramo. Solo se leen los campos
// que hacen falta aca: el resto (tokens, seek) es ruido.
type asrSegment struct {
	Start        float64 `json:"start"`
	End          float64 `json:"end"`
	Text         string  `json:"text"`
	NoSpeechProb float64 `json:"no_speech_prob"`
	AvgLogprob   float64 `json:"avg_logprob"`
}

// noSpeechCutoff es el umbral de "esto no era voz". 0.8 es conservador a
// proposito: preferimos transcribir de mas un audio con ruido de fondo que
// inventar un pedido a partir de un silencio.
const noSpeechCutoff = 0.8

// mostlySilence devuelve true si el modelo no vio habla en ningun segmento.
func mostlySilence(segs []asrSegment) bool {
	if len(segs) == 0 {
		return false
	}
	for _, s := range segs {
		if s.NoSpeechProb < noSpeechCutoff {
			return false
		}
	}
	return true
}

// Client habla con whisper-asr-webservice.
//
// La API NO es la de OpenAI. El contrato real, verificado contra el
// openapi.json del servicio (version 1.10.0), es:
//
//	POST /asr?language=es&task=transcribe&output=json
//	Content-Type: multipart/form-data
//	campo: audio_file  (NO "file")
//	respuesta 200: {"language":"es","text":"...","segments":[...]}
//
// Con output=txt (default) devuelve text plano, no JSON. Se pide json
// explicitamente porque del segmento sale el no_speech_prob, que es lo que
// permite distinguir "el cliente mando silencio" de "no se pudo transcribir".
type Client struct {
	baseURL string
	model   string
	http    *http.Client
	log     *slog.Logger
}

func NewClient(baseURL, model string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: timeout},
		log:     log,
	}
}

func (c *Client) Model() string { return c.model }

// Result es una transcripcion.
type Result struct {
	Text  string
	Lang  string
	MS    int
	Bytes int
	Model string
}

// Transcribe manda el audio a Whisper.
//
// filename y contentType salen de los attachments del webhook. filename no es
// cosmetico: el servicio decide como decodificar segun la extension cuando
// encode=true no alcanza.
func (c *Client) Transcribe(ctx context.Context, audio []byte, filename, contentType string) (*Result, error) {
	if len(audio) == 0 {
		return nil, fmt.Errorf("audio vacio")
	}
	if len(audio) > MaxAudioBytes {
		return nil, ErrTooLarge
	}

	// Chequeo previo de duracion cuando se puede. Si el audio dura 40 minutos
	// la CPU se va en una transcripcion que despues se tira a la basura: en
	// Ogg/Opus (que es lo que manda WhatsApp) la duracion esta en el contenedor
	// y se lee sin decodificar nada.
	if sec, ok := oggDurationSec(audio); ok && sec > MaxAudioDurationSec {
		return nil, fmt.Errorf("%w: %.0fs", ErrTooLong, sec)
	}

	if filename == "" {
		filename = "audio.ogg"
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	// El nombre del campo es audio_file. "file" devuelve 422 y es el error mas
	// dificil de ver de esta API, porque el servicio responde 422 con un
	// detalle generico de validacion.
	part, err := mw.CreateFormFile("audio_file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to build multipart: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return nil, fmt.Errorf("failed to write audio: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart: %w", err)
	}

	// Los parametros van en la QUERY, no en el multipart: el servicio los lee
	// de ahi. vad_filter se deja en false porque el Filtro por voz recorta
	// palabras cortas ("papel", "100") en audios con ruido de calle.
	q := url.Values{}
	q.Set("task", "transcribe")
	q.Set("output", "json")
	q.Set("encode", "true")
	q.Set("vad_filter", "false")

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/asr?"+q.Encode(), &body)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("asr unreachable: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(string(payload))
		if len(detail) > 300 {
			detail = detail[:300] + "..."
		}
		return nil, &APIError{Status: resp.StatusCode, Body: detail}
	}

	var out struct {
		Language string       `json:"language"`
		Text     string       `json:"text"`
		Segments []asrSegment `json:"segments"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Red de seguridad del limite de duracion para contenedores que no son
	// Ogg (mp4/webm), donde el chequeo previo no pudo leer nada.
	for _, seg := range out.Segments {
		if seg.End > MaxAudioDurationSec {
			return nil, fmt.Errorf("%w: %.0fs segun whisper", ErrTooLong, seg.End)
		}
	}

	text := strings.TrimSpace(out.Text)
	// Si todos los segmentos son casi-silencio no hay nada que analizar: el
	// cliente mando una nota de voz muda o un archivo de ruido.
	if text != "" && mostlySilence(out.Segments) {
		return nil, fmt.Errorf("audio casi sin voz (no_speech_prob alta)")
	}

	return &Result{
		Text:  text,
		Lang:  out.Language,
		MS:    int(time.Since(start).Milliseconds()),
		Bytes: len(audio),
		Model: c.model,
	}, nil
}

// Ping comprueba que el servicio responda. Distingue "esta caido" de "esta
// levantando el modelo todavia", que son dos cosas muy distintas en la UI.
func (c *Client) Ping(ctx context.Context) error {
	// /docs lo sirve FastAPI siempre; un 200 aca significa proceso arriba.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/openapi.json", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 500 {
		return fmt.Errorf("asr devuelve %d", resp.StatusCode)
	}
	return nil
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("asr %d: %s", e.Status, e.Body)
}

// Retryable: 4xx es un archivo que Whisper no entiende (reintentar es
// inútil); 5xx o saturacion de CPU si son reintentables.
func (e *APIError) Retryable() bool {
	return e.Status >= 500 || e.Status == http.StatusTooManyRequests
}

// oggDurationSec lee la duracion de un Ogg sin decodificar el audio.
//
// El ultimo pagina Ogg del stream tiene la posicion del grano final, que para
// Opus esta siempre en muestras de 48 kHz. Es el unico formato que se puede
// medir asi, y es exactamente el que manda WhatsApp.
func oggDurationSec(audio []byte) (float64, bool) {
	if len(audio) < 27 || !bytes.Equal(audio[:4], []byte("OggS")) {
		return 0, false
	}
	// Se recorre desde el final: el grano final esta en la ultima pagina.
	// Backwards a mano porque un Ogg de nota de voz son pocos KB y no vale la
	// pena un parser de paginas.
	pos := len(audio) - 27
	for pos >= 0 {
		if audio[pos] != 'S' || pos+3 >= len(audio) ||
			!bytes.Equal(audio[pos-3:pos+1], []byte("OggS")) {
			pos--
			continue
		}
		headerType := audio[pos+5]
		granule := int64(binary.LittleEndian.Uint64(audio[pos+6 : pos+14]))
		segCount := int(audio[pos+26])
		// La pagina termina donde dice la tabla de segmentos. La ultima pagina
		// real es la que no es "continued" (bit 0x01).
		if headerType&0x01 == 0 && segCount > 0 && pos+27+segCount <= len(audio) {
			if granule > 0 {
				return float64(granule) / 48000.0, true
			}
			return 0, true
		}
		pos -= 4
	}
	return 0, false
}
