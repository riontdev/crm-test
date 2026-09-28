package handlers

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Helpers compartidos por los handlers de insight. Viven aca y no en insights.go
// para que inbox.go y los nuevos endpoints los usen sin imports cruzados.

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(s))
}

// normalizeChannel acepta solo los tres canales del producto. Un canal
// arbitrario en la URL crearia una insight_configs basura que nadie limpia.
func normalizeChannel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "whatsapp":
		return "whatsapp"
	case "instagram":
		return "instagram"
	case "facebook":
		return "facebook"
	}
	return ""
}

// currentUserID saca el usuario de la sesion. Si no hay (endpoint sin auth, o
// un token viejo), devuelve el UUID cero: las columnas updated_by/edited_by son
// nullable y preferimos perder la trazabilidad antes que romper el INSERT.
func currentUserID(c echo.Context) uuid.UUID {
	if v, ok := c.Get("user_id").(string); ok {
		if id, err := uuid.Parse(v); err == nil {
			return id
		}
	}
	return uuid.Nil
}

func emptyStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func emptyInts(in []int) []int {
	if in == nil {
		return []int{}
	}
	return in
}

// stringDetails convierte el jsonb de detalles a string.
//
// Postgres devuelve map[string]any: los valores que escribimos son siempre
// strings, pero un numero o un bool no puede romper la respuesta de la UI. Un
// valor raro se saltea en vez de inventarle una representacion.
func stringDetails(in map[string]any) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch t := v.(type) {
		case string:
			if t != "" {
				out[k] = t
			}
		case float64:
			out[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			out[k] = strconv.FormatBool(t)
		}
	}
	return out
}

func orDefault(v, def int) int {
	if v < 1 {
		return def
	}
	return v
}

// insightValidIntent replica el CHECK de message_analyses/conversation_orders.
// Se valida en el borde para devolver 400 con un mensaje claro en vez de un 500
// por constraint violada.
func insightValidIntent(s string) bool {
	switch s {
	case "pedido", "info", "reclamo", "otro":
		return true
	}
	return false
}

// insightValidDetail es la lista cerrada de claves del campo detalles. Cerrada a
// proposito: es lo que la UI sabe pintar, y una clave arbitraria ahi no
// significaria nada para nadie.
var insightValidDetail = func(k string) bool {
	switch k {
	case "material", "medidas", "personalizacion", "fecha_entrega", "envio", "color":
		return true
	}
	return false
}
