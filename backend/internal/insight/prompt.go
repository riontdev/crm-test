package insight

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed prompts/analyst.tmpl
var analystTemplate string

// MaxCatalogItems es el tope de productos que entran al prompt. El catalogo es
// la parte variable del system prompt: cada item extra son ~15 tokens que se
// pagan en CADA mensaje. 150 cubre un taller sin reventar el contexto.
const MaxCatalogItems = 150

// CatalogItem es una fila de product_catalog.
type CatalogItem struct {
	Name     string
	Category string
	Aliases  []string
}

// buildCatalogBlock arma la seccion del prompt con el catalogo. Agrupa por
// categoria y agrega los alias para que el modelo normalice "filamento" o
// "3d" al nombre canonico sin tener que inventar una equivalencia.
func buildCatalogBlock(items []CatalogItem) string {
	if len(items) == 0 {
		return "## CATALOGO\n(vacio: usa las palabras del cliente, en minusculas)"
	}
	if len(items) > MaxCatalogItems {
		items = items[:MaxCatalogItems]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## CATALOGO (%d productos disponibles)\n", len(items))

	byCategory := map[string][]CatalogItem{}
	var order []string
	for _, it := range items {
		if _, seen := byCategory[it.Category]; !seen {
			order = append(order, it.Category)
		}
		byCategory[it.Category] = append(byCategory[it.Category], it)
	}

	for _, cat := range order {
		fmt.Fprintf(&b, "\n### %s\n", strings.ToUpper(cat))
		for _, it := range byCategory[cat] {
			b.WriteString("- " + it.Name)
			if len(it.Aliases) > 0 {
				b.WriteString(" (tambien: " + strings.Join(it.Aliases, ", ") + ")")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// BuildSystemPrompt arma el system prompt completo.
//
// extra son las "instrucciones adicionales" que el operador puede guardar en
// insight_configs.system_prompt. Van como SUPLAMENTO, nunca reemplazan las
// reglas: si el operador puede borrar el contrato de salida, un prompt mal
// escrito rompe el JSON y la cola se cae sola. Por eso el nombre de la
// columna es system_prompt pero el contrato con el modelo no se toca.
func BuildSystemPrompt(items []CatalogItem, extra string) string {
	prompt := strings.ReplaceAll(analystTemplate, "{{catalog}}", buildCatalogBlock(items))

	if e := strings.TrimSpace(extra); e != "" {
		prompt = "## INSTRUCCIONES ADICIONALES DEL OPERADOR\n" + e + "\n\n" + prompt
	}
	return prompt
}

// BuildUserPrompt arma el mensaje del usuario: el hilo del cliente.
//
// El formato "Cliente:/Agente:" es el mismo de los ejemplos, asi que el modelo
// ya lovio en el system prompt y no tiene que adivinar donde termina cada
// turno. El ultimo mensaje va marcado con >> porque es el que manda.
func BuildUserPrompt(transcript string, current string) string {
	var b strings.Builder
	if strings.TrimSpace(transcript) != "" {
		b.WriteString("HILO PREVIO (contexto, no es el pedido actual):\n")
		b.WriteString(strings.TrimRight(transcript, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString(">>> MENSAJE ACTUAL A ANALIZAR:\n")
	b.WriteString(strings.TrimSpace(current))
	return b.String()
}
