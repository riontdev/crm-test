package insight

// TestLiveModeloDeltaVsTotal es la verificacion HONESTA del contrato de
// tipo_cantidad: corre el pipeline real (BuildSystemPrompt -> OllamaClient ->
// Enrich -> MergeOrder) contra el modelo que esta corriendo, y mira el pedido
// consolidado que un operador vera.
//
// Vive en el repo y no en un script porque lo que se tiene que verificar no es
// "que dice el modelo" sino "que pedido queda". Reimplementar el merge en otro
// lenguaje para probarlo dio dos falsos negativos seguidos: la copia se buggyaba
// y el backend-era correcto. Un solo codigo, una sola verdad.
//
// No corre en `go test ./...`: necesita Ollama prendido. Se corre a mano:
//
//	LIVE_OLLAMA=http://127.0.0.1:11434 LIVE_MODEL=granite3.3:2b \
//	  go test ./internal/insight/ -run TestLiveModelo -v
//
// Requiere ademas un producto "Sticker" en el catalogo, asi que el catalogo se
// arma en memoria a proposito (la version real tiene 45 items y el modelo
// canoniza distinto). Lo que se prueba es el contrato suma/reemplaza, no el
// catalogo.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLiveModeloDeltaVsTotal(t *testing.T) {
	base := os.Getenv("LIVE_OLLAMA")
	if base == "" {
		t.Skip("sin LIVE_OLLAMA: este test necesita el modelo local")
	}
	model := os.Getenv("LIVE_MODEL")
	if model == "" {
		model = "granite3.3:2b"
	}

	llm := NewOllamaClient(base, model, 300*time.Second, slog.New(slog.DiscardHandler))
	catalog := []CatalogItem{
		{Name: "Sticker", Category: "vinilos", Aliases: []string{"calca", "pegatina"}},
		{Name: "Llavero", Category: "regalos", Aliases: []string{}},
		{Name: "Maceta", Category: "hogar", Aliases: []string{"macetas"}},
	}
	system := BuildSystemPrompt(catalog, "")
	transcript := "Cliente: hola, quiero 30 stickers de 5cm\nAgente: perfecto, te confirmo"

	// prev: el pedido ya consolidado, 30 stickers.
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers de 5cm",
		Productos: []string{"Sticker"}, Cantidades: []int{30},
		Medidas: strp("5cm"), Confianza: 0.9,
	}, model)

	casos := []struct {
		texto   string
		quiere  string
		comment string
	}{
		{"sumale 2 stickers mas", "32", "delta con el numero escrito"},
		{"agrega 2 stickers", "32", "delta: otra redaccion del mismo mensaje"},
		{"necesito 2 stickers mas por el cumple", "32", "delta con motivo de por medio"},
		{"poneme 2 stickers mas", "32", "delta en imperativo"},
		{"y sumale 2 mas", "32", "delta con elipsis: el caso que el modelo calcula solo"},
		{"mejor 50 stickers", "50", "total explicito"},
		{"dejame solo 2 stickers", "2", "total que baja la cantidad"},
		{"en total son 12 stickers", "12", "total con la palabra total"},
		{"y 12 llaveros", "30+12", "producto nuevo: se agrega, no reemplaza"},
		{"sumale 5 llaveros mas", "30+5", "producto nuevo con 'mas'"},
	}

	ok, conRevision := 0, 0
	var corruptos []string
	for _, c := range casos {
		t.Run(c.texto, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
			defer cancel()

			res, err := llm.Analyze(ctx, system, BuildUserPrompt(transcript, c.texto))
			if err != nil {
				t.Fatalf("no se pudo analizar: %v", err)
			}
			a := res.Analysis
			Enrich(a, catalog, c.texto)

			next := OrderFromAnalysis(prev.ConversationID, uuid.New(), a, model)
			got := MergeOrder(prev, next, a.TipoCantidad)

			// El verificador es el pedido completo, no un campo.
			//
			// Y hay DOS salidas validas, no una: el pedido correcto, o el pedido
			// intacto con revision. Un caso donde el modelo se equivoco y el
			// guardarraíl lo veto esta bien resuelto SI Y SOLO SI el pedido
			// quedo como estaba y quedo marcado. needs_review tambien lo activa
			// la regla vieja de "pedido sin material", asi que no sirve para
			// distinguir un veto de una duda de material: por eso se compara el
			// pedido, no el flag.
			estado := describe(got)
			queria := describePedido(c.quiere)
			t.Logf("tipo=%s cantidades=%v -> %s (revision=%v) [%s]",
				tipoTexto(a.TipoCantidad), a.Cantidades, estado, got.NeedsReview, c.comment)

			switch {
			case estado == queria:
				ok++
			case estado == describe(prev) && got.NeedsReview:
				// el modelo se equivoco y el sistema se nego a pisar el pedido
				conRevision++
			default:
				corruptos = append(corruptos, fmt.Sprintf("%s: %s (queria %s)", c.texto, estado, queria))
			}
		})
	}

	t.Logf("pedidos correctos: %d/%d | defendidos con revision: %d | CORRUPTOS: %d",
		ok, len(casos), conRevision, len(corruptos))

	// Lo unico que no se tolera es un pedido que quedo con una cantidad que
	// nadie pidio, sin aviso. Si el modelo se equivoca, el guardarraíl tiene que
	// convertarlo en "no toco nada + revision", nunca en un numero inventado.
	for _, c := range corruptos {
		t.Errorf("PEDIDO CORRUPTO, sin revision que lo delate -> %s", c)
	}
}

func tipoTexto(t *string) string {
	if t == nil {
		return "nil"
	}
	return *t
}

func describe(o *Order) string {
	var b strings.Builder
	for i, p := range o.Productos {
		if i > 0 {
			b.WriteString("+")
		}
		q := 0
		if i < len(o.Cantidades) {
			q = o.Cantidades[i]
		}
		b.WriteString(p)
		b.WriteString(" ")
		b.WriteString(itoa(q))
	}
	if b.Len() == 0 {
		return "(vacio)"
	}
	return b.String()
}

// describePedido arma el texto esperado desde "32" o "30+12".
func describePedido(want string) string {
	partes := strings.Split(want, "+")
	// el primero es el producto Sticker; el resto, Llavero
	out := ""
	for i, p := range partes {
		nombre := "Sticker"
		if i > 0 {
			nombre = "Llavero"
		}
		if out != "" {
			out += "+"
		}
		out += nombre + " " + p
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
