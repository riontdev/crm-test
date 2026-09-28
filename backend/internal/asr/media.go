package asr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// attachment es la forma en que Zernio manda un adjunto en el webhook.
type attachment struct {
	Type    string          `json:"type"`
	URL     string          `json:"url"`
	Payload json.RawMessage `json:"payload"`
}

// audioPayload son los datos extra que manda Zernio para los audios.
type audioPayload struct {
	ID       string `json:"id"`
	MimeType string `json:"mimeType"`
}

// Fetcher baja los adjuntos de Zernio. Necesita la API key, pero no el
// cliente de Zernio: asi el paquete insight no puede enviar mensajes (el
// cliente de envio vive en internal/zernio y no se importa desde aca).
type Fetcher struct {
	apiKey string
	http   *http.Client
}

func NewFetcher(apiKey string, timeout time.Duration) *Fetcher {
	return &Fetcher{apiKey: apiKey, http: &http.Client{Timeout: timeout}}
}

// Media es un adjunto ya bajado a memoria.
type Media struct {
	Audio       []byte
	URL         string
	MimeType    string
	Filename    string
	DurationSec float64
}

// HasAudio dice si los attachments del mensaje traen un audio. Lo exporta el
// worker para decidir a que cola va el mensaje sin descargar nada.
func HasAudio(attachments json.RawMessage) bool {
	_, _, ok := audioAttachment(attachments)
	return ok
}

// audioAttachment encuentra el primer adjunto de audio del mensaje.
func audioAttachment(attachments json.RawMessage) (string, string, bool) {
	if len(attachments) == 0 {
		return "", "", false
	}
	var list []attachment
	if err := json.Unmarshal(attachments, &list); err != nil {
		return "", "", false
	}
	for _, a := range list {
		if !strings.EqualFold(a.Type, "audio") || a.URL == "" {
			continue
		}
		mime := "audio/ogg"
		var p audioPayload
		if len(a.Payload) > 0 && json.Unmarshal(a.Payload, &p) == nil && p.MimeType != "" {
			mime = p.MimeType
		}
		return a.URL, mime, true
	}
	return "", "", false
}

// FetchAudio descarga el audio del primer adjunto de tipo audio.
//
// Devuelve nil (sin error) cuando el mensaje no tiene audio: "no hay nada que
// hacer" no es un fallo y no debe ensuciar los logs de error.
func (f *Fetcher) FetchAudio(ctx context.Context, attachments json.RawMessage) (*Media, error) {
	url, mime, ok := audioAttachment(attachments)
	if !ok {
		return nil, nil
	}
	if f.apiKey == "" {
		return nil, fmt.Errorf("ZERNIO_API_KEY no configurada: no se puede bajar el audio")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.apiKey)

	resp, err := f.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{Status: resp.StatusCode, Body: "fallo descargando el adjunto"}
	}

	// Content-Length primero: si el server lo manda, se evita bajar 20 MB
	// para descubrir que eran 200 MB.
	if resp.ContentLength > MaxAudioBytes {
		return nil, ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxAudioBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}
	if len(data) > MaxAudioBytes {
		return nil, ErrTooLarge
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("el adjunto esta vacio")
	}

	// El nombre importa: Whisper decide el decoder por la extension.
	filename := "audio.ogg"
	if strings.Contains(mime, "mp4") || strings.Contains(mime, "m4a") {
		filename = "audio.m4a"
	} else if strings.Contains(mime, "mpeg") {
		filename = "audio.mp3"
	} else if strings.Contains(mime, "webm") {
		filename = "audio.webm"
	} else if strings.Contains(mime, "wav") {
		filename = "audio.wav"
	}

	return &Media{
		Audio:    data,
		URL:      url,
		MimeType: mime,
		Filename: filename,
	}, nil
}
