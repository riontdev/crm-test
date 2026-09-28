package insight

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func strp(s string) *string { return &s }

var testCatalog = []CatalogItem{
	{Name: "Maceta", Category: "pieza", Aliases: []string{"maceta", "jardinera"}},
	{Name: "Llavero", Category: "pieza", Aliases: []string{"llavero"}},
	{Name: "PLA", Category: "material", Aliases: []string{"filamento pla"}},
	{Name: "PETG", Category: "material", Aliases: []string{"petg", "filamento petg"}},
	{Name: "TPU (flexible)", Category: "material", Aliases: []string{"tpu", "flexible"}},
	{Name: "Nylon (PA)", Category: "material", Aliases: []string{"nylon", "pa6"}},
}

func TestNormText(t *testing.T) {
	cases := map[string]string{
		"Impresión  3D":      "impresion 3d",
		"  TPU   NEGRO  ":    "tpu negro",
		"MACETAS\n\nde 10cm": "macetas de 10cm",
		"":                   "",
	}
	for in, want := range cases {
		if got := normText(in); got != want {
			t.Errorf("normText(%q) = %q, want %q", in, got, want)
		}
	}
}

// hasWord es la defensa contra falsos positivos. El caso que importa:
// alias "pa" de nylon contra "papel de 300g" NO debe matchear.
func TestHasWordNoMatchInsideWords(t *testing.T) {
	if hasWord("imprimir en papel de 300g", "pa") {
		t.Error("matcheo 'pa' dentro de 'papel': hay que comparar por palabra")
	}
	if !hasWord("lo quiero en PA, thicken", "pa") {
		t.Error("no matcheo la palabra suelta 'pa'")
	}
	if !hasWord("en tpu negro", "TPU") {
		t.Error("no matcheo tpu en minusculas")
	}
	if hasWord("superpuesto", "per") {
		t.Error("matcheo un alias pegado dentro de otra palabra")
	}
	if !hasWord("quiero filamento PLA", "filamento pla") {
		t.Error("no matcheo un alias de varias palabras")
	}
	if hasWord("impresion", "impresion 3d") {
		t.Error("matcheo un alias multi-palabra parcial")
	}
}

// Enrich es el arreglo que sube el bench de 80% a ~95%: el modelo deja
// material vacio pero lo pone en el resumen.
func TestEnrichRecoversMaterialFromResumen(t *testing.T) {
	a := &Analysis{
		Intent:   "pedido",
		Resumen:  "Quiere un organizador de escritorio en TPU negro con 6 compartimentos",
		Material: nil,
	}
	Enrich(a, testCatalog, "")
	if a.Material == nil {
		t.Fatal("no recupero el material del resumen")
	}
	if *a.Material != "TPU (flexible)" {
		t.Errorf("material = %q, quiere el nombre canonico del catalogo", *a.Material)
	}
}

func TestEnrichDoesNotInventMaterial(t *testing.T) {
	a := &Analysis{Intent: "pedido", Resumen: "Quiere un llavero en 3D para regalar", Material: nil}
	Enrich(a, testCatalog, "")
	if a.Material != nil {
		t.Errorf("invento material %q donde el cliente no dijo ninguno", *a.Material)
	}
}

func TestEnrichKeepsWhatModelSaid(t *testing.T) {
	a := &Analysis{Intent: "pedido", Resumen: "Quiere macetas en PETG", Material: strp("PETG")}
	Enrich(a, testCatalog, "")
	if *a.Material != "PETG" {
		t.Errorf("sobrescribio el material del modelo: %q", *a.Material)
	}
}

func TestCanonicalizeProducts(t *testing.T) {
	a := &Analysis{Productos: []string{"macetas", "llavero", "maceta", "  ", "portaobjetos"}}
	Enrich(a, testCatalog, "")
	want := []string{"Maceta", "Llavero", "portaobjetos"}
	if len(a.Productos) != len(want) {
		t.Fatalf("productos = %v, want %v", a.Productos, want)
	}
	for i := range want {
		if a.Productos[i] != want[i] {
			t.Errorf("productos[%d] = %q, want %q", i, a.Productos[i], want[i])
		}
	}
}

func TestNeedsReview(t *testing.T) {
	cases := []struct {
		name string
		a    *Analysis
		want bool
	}{
		{"pedido completo", &Analysis{Intent: "pedido", Resumen: "Quiere 4 macetas",
			Productos: []string{"Maceta"}, Material: strp("PLA"), Confianza: 0.9}, false},
		{"pedido sin material", &Analysis{Intent: "pedido", Resumen: "Quiere 4 macetas",
			Productos: []string{"Maceta"}, Confianza: 0.9}, true},
		{"pedido sin producto", &Analysis{Intent: "pedido", Resumen: "Quiere algo",
			Material: strp("PLA"), Confianza: 0.9}, true},
		{"info no pide revision", &Analysis{Intent: "info", Resumen: "Pregunta horarios",
			Confianza: 0.9}, false},
		{"confianza baja", &Analysis{Intent: "info", Resumen: "Pregunta algo raro",
			Confianza: 0.3}, true},
		{"resumen vacio", &Analysis{Intent: "pedido", Confianza: 0.9}, true},
	}
	for _, c := range cases {
		if got := NeedsReview(c.a); got != c.want {
			t.Errorf("%s: NeedsReview = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOrderFromAnalysisDerivesEnvio(t *testing.T) {
	o := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent:     "pedido",
		Resumen:    "Quiere 2 macetas y que se las envien a Rosario",
		Productos:  []string{"Maceta"},
		Cantidades: []int{2},
		Confianza:  0.9,
	}, "granite3.3:2b")

	if o.Detalles["envio"] != "si" {
		t.Errorf("no detecto el envio: %v", o.Detalles)
	}
}

func TestOrderFromAnalysisNoInventedDetails(t *testing.T) {
	o := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere un llavero", Confianza: 0.8,
	}, "m")

	for _, k := range []string{"material", "medidas", "personalizacion", "fecha_entrega", "envio"} {
		if _, exists := o.Detalles[k]; exists {
			t.Errorf("se invento el detalle %q: %v", k, o.Detalles)
		}
	}
}

// El consolidado es el corazon del producto: "quiero una maceta" + "en PETG" +
// "son 3 con logo" tiene que verse como una sola cosa.
func TestMergeOrderAccumulatesHilo(t *testing.T) {
	convID := uuid.New()

	prev := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere una maceta", Productos: []string{"Maceta"},
		Cantidades: []int{1}, Confianza: 0.8,
	}, "m")

	next := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 3 macetas en PETG con logo",
		Productos: []string{"Maceta"}, Cantidades: []int{3},
		Material: strp("PETG"), Personalizacion: strp("logo"),
		Confianza: 0.9,
	}, "m")

	// "son 3" sobre un pedido de 1 es un TOTAL, no un incremento: el cliente
	// esta corrigiendo su propia cifra. Sin tipo lo unico que se puede hacer con
	// un numero sobre una linea existente es dejar la duda a la vista.
	got := MergeOrder(prev, next, strp(TipoTotal))
	if got.Detalles["material"] != "PETG" {
		t.Errorf("no tomo el material nuevo: %v", got.Detalles)
	}
	if got.Detalles["personalizacion"] != "logo" {
		t.Errorf("no tomo la personalizacion: %v", got.Detalles)
	}
	if len(got.Cantidades) != 1 || got.Cantidades[0] != 3 {
		t.Errorf("cantidades = %v, quiere [3] (mismo producto, cantidad refinada)", got.Cantidades)
	}
	if *got.Confianza != 0.9 {
		t.Errorf("confianza = %v, quiere 0.9 (gana el ultimo mensaje)", *got.Confianza)
	}
}

func TestMergeOrderSumaProductosDistintos(t *testing.T) {
	convID := uuid.New()
	prev := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere una maceta", Productos: []string{"Maceta"},
		Cantidades: []int{1}, Confianza: 0.8,
	}, "m")
	next := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Sumale un llavero", Productos: []string{"Llavero"},
		Cantidades: []int{1}, Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, nil)
	if len(got.Productos) != 2 {
		t.Errorf("productos = %v, quiere los dos acumulados", got.Productos)
	}
	if len(got.Cantidades) != 2 {
		t.Errorf("cantidades = %v, quiere [1 1]", got.Cantidades)
	}
}

// "gracias!" no puede borrar el pedido que el cliente ya hizo antes.
func TestMergeOrderMensajeVacioNoBorraElPedido(t *testing.T) {
	convID := uuid.New()
	prev := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 3 macetas en PETG", Productos: []string{"Maceta"},
		Cantidades: []int{3}, Material: strp("PETG"), Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "otro", Resumen: "Agradece", Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, nil)
	if len(got.Productos) != 1 {
		t.Errorf("un 'gracias' borro los productos: %v", got.Productos)
	}
	if got.Detalles["material"] != "PETG" {
		t.Errorf("un 'gracias' borro el material: %v", got.Detalles)
	}
}

// La regla dura de conversation_orders: un humano edito, la IA no escribe.
func TestMergeOrderRespectsEdited(t *testing.T) {
	convID := uuid.New()
	prev := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 3 macetas", Productos: []string{"Maceta"},
		Cantidades: []int{3}, Confianza: 0.9,
	}, "m")
	prev.Edited = true
	prev.Detalles["material"] = "ABS"

	next := OrderFromAnalysis(convID, uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Cambiado a PETG", Productos: []string{"Maceta"},
		Cantidades: []int{5}, Material: strp("PETG"), Confianza: 0.95,
	}, "m")

	got := MergeOrder(prev, next, nil)
	if got.Detalles["material"] != "ABS" {
		t.Errorf("la IA piso un pedido editado por el operador: %v", got.Detalles)
	}
	if len(got.Cantidades) != 1 || got.Cantidades[0] != 3 {
		t.Errorf("la IA cambio las cantidades de un pedido editado: %v", got.Cantidades)
	}
}

func TestBuildSystemPromptInjectsCatalog(t *testing.T) {
	p := BuildSystemPrompt(testCatalog, "Priorizar imprenta sobre 3D")
	if contains(p, "{{catalog}}") {
		t.Error("quedo el placeholder {{catalog}} sin reemplazar")
	}
	if !contains(p, "Maceta") {
		t.Error("el catalogo no entro al prompt")
	}
	if !contains(p, "Priorizar imprenta sobre 3D") {
		t.Error("las instrucciones del operador no entraron")
	}
	if !contains(p, `"intent"`) {
		t.Error("el contrato de salida no esta en el prompt")
	}
}

func TestBuildSystemPromptVacio(t *testing.T) {
	p := BuildSystemPrompt(nil, "")
	if !contains(p, "CATALOGO") {
		t.Error("sin catalogo deberia avisar igual, para que el modelo no invente nombres")
	}
}

func TestBuildUserPromptMarcaElUltimo(t *testing.T) {
	u := BuildUserPrompt("Cliente: hola\nAgente: que necesitas?", "quiero una maceta en PETG")
	if !contains(u, "HILO PREVIO") {
		t.Error("no incluyo el hilo previo")
	}
	if !contains(u, ">>> MENSAJE ACTUAL") {
		t.Error("no marco el mensaje actual")
	}
	if !contains(u, "quiero una maceta en PETG") {
		t.Error("no incluyo el mensaje a analizar")
	}
}

func TestInvalidIntentDegradesToOtro(t *testing.T) {
	// El CHECK constraint de la DB solo acepta 4 valores. Si el modelo inventa
	// un quinto, tiene que caer a 'otro' y no romper el INSERT.
	if validIntents["reclamo_cliente"] {
		t.Error("el test esta mal: 'reclamo_cliente' no deberia ser valido")
	}
	if !validIntents["otro"] {
		t.Error("'otro' deberia ser el fallback valido")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}

// ---------------------------------------------------------------------------
// El 5cm no es la cantidad
// ---------------------------------------------------------------------------

func TestSaneaCantidadesSacaMedidas(t *testing.T) {
	cases := []struct {
		name    string
		a       *Analysis
		want    []int
		medida  string
		confMin float64
	}{
		{
			// El caso real del bench: el 2B confundo el 5 de "5cm" con la
			// cantidad y se confidio 0.95.
			name:    "medida pegada al numero",
			a:       &Analysis{Resumen: "Quiere 2 stickers circulares de 5cm en vinil", Productos: []string{"stickers"}, Cantidades: []int{5}, Confianza: 0.95},
			want:    []int{2},
			medida:  "5cm",
			confMin: 0.5,
		},
		{
			name:    "medida ya presente no se pisa",
			a:       &Analysis{Resumen: "3 macetas de 8 cm en PLA", Productos: []string{"Maceta"}, Cantidades: []int{8}, Medidas: strp("8 cm"), Confianza: 0.9},
			want:    []int{3},
			medida:  "8 cm",
			confMin: 0.5,
		},
		{
			name:   "cantidad correcta se respeta",
			a:      &Analysis{Resumen: "3 macetas de 8 cm en PLA", Productos: []string{"Maceta"}, Cantidades: []int{3}, Confianza: 0.95},
			want:   []int{3},
			medida: "",
		},
		{
			name:   "sin unidades no toca nada",
			a:      &Analysis{Resumen: "quiero 25 llaveros de madera", Productos: []string{"Llavero"}, Cantidades: []int{25}, Confianza: 0.95},
			want:   []int{25},
			medida: "",
		},
		{
			name:   "varias unidades",
			a:      &Analysis{Resumen: "2 piezas de 10x20 cm", Productos: []string{"Pieza a medida"}, Cantidades: []int{10}, Confianza: 0.9},
			want:   []int{2},
			medida: "10x20 cm",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			saneaCantidades(c.a)
			if len(c.a.Cantidades) != len(c.want) {
				t.Fatalf("cantidades = %v, want %v", c.a.Cantidades, c.want)
			}
			for i := range c.want {
				if c.a.Cantidades[i] != c.want[i] {
					t.Errorf("cantidades = %v, want %v", c.a.Cantidades, c.want)
				}
			}
			if c.medida == "" {
				if c.a.Medidas != nil && strings.TrimSpace(*c.a.Medidas) != "" {
					t.Errorf("medidas = %q, se esperaba vacio", *c.a.Medidas)
				}
			} else if c.a.Medidas == nil || !strings.Contains(*c.a.Medidas, strings.TrimSpace(strings.Fields(c.medida)[0])) {
				t.Errorf("medidas = %v, want %q", c.a.Medidas, c.medida)
			}
			if c.a.Confianza >= c.confMin && c.confMin > 0 {
				t.Errorf("confianza = %v, deberia caer bajo %v para que NeedsReview se encienda", c.a.Confianza, c.confMin)
			}
		})
	}
}

// El error del modelo tiene que encender el badge aunque la confianza viniera
// alta:NeedsReview mira confianza, no un campo "el modelo se equivoco".
func TestSaneaCantidadesEnciendeNeedsReview(t *testing.T) {
	a := &Analysis{
		Resumen:    "Quiere 2 stickers circulares de 5cm",
		Productos:  []string{"stickers"},
		Cantidades: []int{5},
		Confianza:  0.95,
	}
	saneaCantidades(a)
	if !NeedsReview(a) {
		t.Fatalf("NeedsReview = false; el operador no veria un pedido con la cantidad mal")
	}
}

// Enrich es el hook que corre en el worker: el guardarrail tiene que estar ahi,
// no solo disponible.
func TestEnrichAplicaElGuardarrailDeCantidades(t *testing.T) {
	a := &Analysis{
		Resumen:    "Quiere 2 stickers circulares de 5cm",
		Productos:  []string{"stickers"},
		Cantidades: []int{5},
		Intent:     "pedido",
		Confianza:  0.95,
	}
	Enrich(a, nil, "2 stickers circulares de 5cm")
	if len(a.Cantidades) != 1 || a.Cantidades[0] != 2 {
		t.Fatalf("Enrich no saneo las cantidades: %v", a.Cantidades)
	}
}

// --- tipo_cantidad: sumar o reemplazar (migracion 000016) ------------------
//
// Estos tests fijan el fallo que motivo la 000016: con 30 stickers pedidos,
// "sumale 2 stickers mas" consolidate 2 en vez de 32 y el pedido perdia 28
// piezas sin error, sin warning y con confianza 0.9. Medido sobre
// granite3.3:2b, 3 de 8 redacciones naturales de ese mismo mensaje caian.

func TestMergeOrderDeltaSuma(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers de 5cm",
		Productos: []string{"Sticker"}, Cantidades: []int{30},
		Medidas: strp("5cm"), Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere sumar 2 stickers mas a los 30 del pedido",
		Productos: []string{"Sticker"}, Cantidades: []int{2},
		// El material viene para que NeedsReview del analysis sea false y la
		// unica forma de que el flag suba sea la ambiguedad del merge.
		Material: strp("vinil"), Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, strp(TipoDelta))
	if len(got.Cantidades) != 1 || got.Cantidades[0] != 32 {
		t.Fatalf("cantidades = %v, quiere [32] (30 + 2, no 2)", got.Cantidades)
	}
	if got.Detalles["medidas"] != "5cm" {
		t.Errorf("un incremento borro la medida: %v", got.Detalles)
	}
	if got.NeedsReview {
		t.Error("un delta bien declarado no tiene que pedir revision")
	}
}

func TestMergeOrderTotalReemplaza(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{30}, Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Cambia el pedido a 50 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{50}, Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, strp(TipoTotal))
	if len(got.Cantidades) != 1 || got.Cantidades[0] != 50 {
		t.Errorf("cantidades = %v, quiere [50] (reemplaza, no suma)", got.Cantidades)
	}
}

// La correccion de medidas reemplaza la linea. Es la decision del operador: si
// pasa a 3 stickers de 10cm son 3 stickers de 10cm, no 33 stickers de 5cm.
func TestMergeOrderTotalCorrigeMedida(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers de 5cm",
		Productos: []string{"Sticker"}, Cantidades: []int{30},
		Medidas: strp("5cm"), Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Mejor 3 stickers de 10cm",
		Productos: []string{"Sticker"}, Cantidades: []int{3},
		Medidas: strp("10cm"), Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, strp(TipoTotal))
	if len(got.Cantidades) != 1 || got.Cantidades[0] != 3 {
		t.Errorf("cantidades = %v, quiere [3]", got.Cantidades)
	}
	if got.Detalles["medidas"] != "10cm" {
		t.Errorf("medidas = %q, quiere 10cm (gana el ultimo mensaje)", got.Detalles["medidas"])
	}
}

// "agrega 3 llaveros" a un pedido que no tiene llaveros: sumar a una linea
// inexistente es crearla con esa cantidad. El tipo da igual en este caso.
func TestMergeOrderDeltaProductoInexistenteCrea(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 3 macetas",
		Productos: []string{"Maceta"}, Cantidades: []int{3}, Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Suma 5 llaveros",
		Productos: []string{"Llavero"}, Cantidades: []int{5}, Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, strp(TipoDelta))
	if len(got.Productos) != 2 {
		t.Fatalf("productos = %v, quiere los dos", got.Productos)
	}
	if len(got.Cantidades) != 2 || got.Cantidades[0] != 3 || got.Cantidades[1] != 5 {
		t.Errorf("cantidades = %v, quiere [3 5]", got.Cantidades)
	}
}

// Un mensaje que trae un producto que ya estaba y un producto nuevo: el
// repetido se suma y el nuevo se agrega, en el mismo mensaje. El merge viejo no
// podia hacer esto porque working por posicion.
func TestMergeOrderMezclaExistenteYNuevo(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{30}, Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Suma 2 stickers y agrega 5 llaveros",
		Productos: []string{"Sticker", "Llavero"}, Cantidades: []int{2, 5},
		Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, strp(TipoDelta))
	if len(got.Productos) != 2 {
		t.Fatalf("productos = %v", got.Productos)
	}
	if len(got.Cantidades) != 2 || got.Cantidades[0] != 32 || got.Cantidades[1] != 5 {
		t.Errorf("cantidades = %v, quiere [32 5] en el orden de productos %v", got.Cantidades, got.Productos)
	}
}

// Varios incrementos seguidos: es lo que hace pendingAnalyses al recorrer los
// pendientes en orden cronologico. Perder el primero seria el mismo fallo que
// la 000016 vino a arreglar.
func TestMergeOrderVariosDeltaEnCadena(t *testing.T) {
	cur := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{30}, Confianza: 0.9,
	}, "m")

	for _, paso := range []struct {
		resumen string
		n       int
	}{
		{"Suma 2 stickers mas", 2},
		{"Y ahora 3 mas", 3},
		{"Sumale 5 mas", 5},
	} {
		next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
			Intent: "pedido", Resumen: paso.resumen,
			Productos: []string{"Sticker"}, Cantidades: []int{paso.n}, Confianza: 0.9,
		}, "m")
		cur = MergeOrder(cur, next, strp(TipoDelta))
	}

	if len(cur.Cantidades) != 1 || cur.Cantidades[0] != 40 {
		t.Errorf("cantidades = %v, quiere [40] (30+2+3+5)", cur.Cantidades)
	}
}

// Sin tipo no se sabe si el numero suma o reemplaza. Pisar el pedido consolidado
// a ciegas es peor que mostrar la duda: se conserva la cantidad y se marca
// revision.
func TestMergeOrderSinTipoNoPisaLaCantidad(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{30}, Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Suma 2 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{2}, Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, nil)
	if len(got.Cantidades) != 1 || got.Cantidades[0] != 30 {
		t.Errorf("cantidades = %v, quiere [30] intacto: sin tipo no se toca", got.Cantidades)
	}
	if !got.NeedsReview {
		t.Error("un numero sobre linea existente sin tipo tiene que pedir revision, no pasar inadvertido")
	}
}

// Producto que ya estaba pero el mensaje no trae cantidad ("sumale stickers
// mas"): tampoco hay nada que sumar ni que reemplazar.
func TestMergeOrderCantidadAusenteNoInventa(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{30}, Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere mas stickers",
		Productos: []string{"Sticker"}, Confianza: 0.7,
	}, "m")

	got := MergeOrder(prev, next, strp(TipoDelta))
	if len(got.Cantidades) != 1 || got.Cantidades[0] != 30 {
		t.Errorf("cantidades = %v, quiere [30] (no se inventa cantidad)", got.Cantidades)
	}
	if !got.NeedsReview {
		t.Error("un 'mas stickers' sin numero tiene que pedir revision")
	}
}

// El catalogo canonicaliza "stickers" a "Sticker". Si el matching se hiciera
// por texto exacto, el delta no encontraria la linea y la duplicaria.
func TestMergeOrderDeltaEmparejaPorProductoCanonicalizado(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{30}, Confianza: 0.9,
	}, "m")
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Suma 2 stickers",
		Productos: []string{"sticker"}, Cantidades: []int{2}, Confianza: 0.9,
	}, "m")

	got := MergeOrder(prev, next, strp(TipoDelta))
	if len(got.Productos) != 1 {
		t.Fatalf("duplico la linea por diferencia de mayusculas: %v", got.Productos)
	}
	if got.Cantidades[0] != 32 {
		t.Errorf("cantidades = %v, quiere [32]", got.Cantidades)
	}
}

func TestNormalizaTipoCantidad(t *testing.T) {
	casos := []struct {
		in   *string
		want *string
		name string
	}{
		{strp("delta"), strp("delta"), "delta"},
		{strp("total"), strp("total"), "total"},
		{strp(" DELTA "), strp("delta"), "espacios y mayusculas"},
		{strp("mas"), nil, "valor inventado por el modelo"},
		{strp(""), nil, "vacio"},
		{nil, nil, "ausente"},
	}
	for _, c := range casos {
		a := &Analysis{TipoCantidad: c.in}
		got := a.normalizaTipoCantidad()
		if c.want == nil && got != nil {
			t.Errorf("%s: got %q, quiere nil", c.name, *got)
		}
		if c.want != nil && (got == nil || *got != *c.want) {
			t.Errorf("%s: got %v, quiere %q", c.name, got, *c.want)
		}
	}
}

// --- guardarraíl: la cantidad de un delta tiene que estar en el mensaje -----
//
// Caso medido contra granite3.3:2b: con un pedido de 30 stickers, "y sumale 2
// mas" devuelve tipo=delta con cantidad 32. El prompt lo prohibe (el delta es
// solo lo que el cliente dijo) pero el modelo igual hizo la cuenta. Si el merge
// le creyera, el pedido pasaria de 30 a 62.

func TestSaneaTipoCantidadRechazaCantidadCalculada(t *testing.T) {
	a := &Analysis{
		Intent: "pedido", Resumen: "Suma 2 stickers mas a lo pedido",
		Productos: []string{"Sticker"}, Cantidades: []int{32},
		TipoCantidad: strp(TipoDelta), Confianza: 0.9,
	}
	saneaTipoCantidad(a, "y sumale 2 mas")
	if a.TipoCantidad != nil {
		t.Errorf("dejo tipo=%q con una cantidad (32) que el cliente no escribio", *a.TipoCantidad)
	}
	if a.Confianza >= 0.5 {
		t.Errorf("confianza = %v: una cantidad inventada tiene que encender el badge", a.Confianza)
	}
}

func TestSaneaTipoCantidadAceptaCantidadDelCliente(t *testing.T) {
	a := &Analysis{
		Intent: "pedido", Resumen: "Suma 2 stickers mas",
		Productos: []string{"Sticker"}, Cantidades: []int{2},
		TipoCantidad: strp(TipoDelta), Confianza: 0.9,
	}
	saneaTipoCantidad(a, "sumale 2 stickers mas por favor")
	if a.TipoCantidad == nil || *a.TipoCantidad != TipoDelta {
		t.Errorf("descarto un delta legitimo: %v", a.TipoCantidad)
	}
	if a.Confianza != 0.9 {
		t.Errorf("bajo la confianza de un delta correcto: %v", a.Confianza)
	}
}

// El total NO se toca con esta regla: un total corregido puede no estar escrito
// ("dejame 5 de los 30") y seguir siendo correcto.
func TestSaneaTipoCantidadNoTocaElTotal(t *testing.T) {
	a := &Analysis{
		Intent: "pedido", Resumen: "Reduce el pedido a 5",
		Productos: []string{"Sticker"}, Cantidades: []int{5},
		TipoCantidad: strp(TipoTotal), Confianza: 0.9,
	}
	saneaTipoCantidad(a, "dejame solo 5 de los 30 stickers")
	if a.TipoCantidad == nil || *a.TipoCantidad != TipoTotal {
		t.Errorf("descarto un total valido: %v", a.TipoCantidad)
	}
}

// Varias cantidades: si una sola no esta escrita, no se aplica ninguna. Aplicar
// la parte que el modelo se acertó seria peor que no aplicar nada.
func TestSaneaTipoCantidadExigeTodasLasCantidades(t *testing.T) {
	a := &Analysis{
		Intent: "pedido", Resumen: "Suma 2 stickers y 3 llaveros",
		Productos: []string{"Sticker", "Llavero"}, Cantidades: []int{2, 40},
		TipoCantidad: strp(TipoDelta), Confianza: 0.9,
	}
	saneaTipoCantidad(a, "sumale 2 stickers y 3 llaveros")
	if a.TipoCantidad != nil {
		t.Errorf("acepto un delta donde una de las cantidades no la escribio el cliente: %v", *a.TipoCantidad)
	}
}

// Sin texto crudo no se descarta nada: no hay evidencia para hacerlo. Con ASR el
// texto siempre esta, pero el guardarraíl no debe romper ese camino.
func TestSaneaTipoCantidadSinTextoNoDescarta(t *testing.T) {
	a := &Analysis{
		Intent: "pedido", Resumen: "Suma 2",
		Productos: []string{"Sticker"}, Cantidades: []int{2},
		TipoCantidad: strp(TipoDelta), Confianza: 0.9,
	}
	saneaTipoCantidad(a, "")
	if a.TipoCantidad == nil {
		t.Error("descarto el tipo sin tener el texto para verificar")
	}
}

// Y el camino completo: Enrich tiene que dejar el pedido intacto cuando el
// modelo se paso de listo con la suma.
func TestEnrichNeutralizaDeltaCalculado(t *testing.T) {
	a := &Analysis{
		Intent: "pedido", Resumen: "Quiere 32 stickers en total",
		Productos: []string{"Sticker"}, Cantidades: []int{32},
		TipoCantidad: strp(TipoDelta), Confianza: 0.95,
	}
	Enrich(a, nil, "y sumale 2 mas")
	if a.TipoCantidad != nil {
		t.Fatalf("Enrich dejo pasar un delta con la cuenta del modelo: %v", *a.TipoCantidad)
	}

	// Con el tipo caido, el merge no toca el pedido: 30 siguen siendo 30.
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers",
		Productos: []string{"Sticker"}, Cantidades: []int{30}, Confianza: 0.9,
	}, "m")
	got := MergeOrder(prev, OrderFromAnalysis(uuid.New(), uuid.New(), a, "m"), a.TipoCantidad)
	if got.Cantidades[0] != 30 {
		t.Errorf("cantidades = %v, quiere [30]: el pedido no se toca sin tipo", got.Cantidades)
	}
	if !got.NeedsReview {
		t.Error("tiene que quedar pedido a revision, no pasar inadvertido")
	}
}

// ---------------------------------------------------------------------------
// Regresion del hilo de Jorge Mujica (2026-09-27)
// ---------------------------------------------------------------------------
//
// El pedido quedo en 2 figuras cuando el cliente ya tenia 2 y habia pedido 1
// mas. La cadena fue:
//
//	"Ustedes hacen un figura de kratos de 15 CM de alto?"  -> pedido, 1 figura
//	"suma 1 figuras de kratos"                              -> delta 1
//	"quiero tambien una figura de Mario de 23 cm"          -> se perdio
//
// El delta estaba BIEN (1+1=2). La base no: la primera linea la creo una
// PREGUNTA, que el modelo clasifico como pedido con el producto pero sin
// cantidad. Una linea fantasma no se ve en el pedido, pero a partir de ahi
// todos los deltas se suman sobre un numero que nadie pidio.

func linea(t *testing.T, a *Analysis, texto string) *Order {
	t.Helper()
	Enrich(a, nil, texto)
	if !Consolida(a) {
		return nil
	}
	return OrderFromAnalysis(uuid.New(), uuid.New(), a, "test")
}

func TestPreguntaNoCreaPedido(t *testing.T) {
	casos := []struct {
		texto     string
		quiere    string
		pertenece string
	}{
		{"Ustedes hacen un figura de kratos de 15 CM de alto?", "info", "info"},
		{"hacen envios a cordoba?", "info", "info"},
		{"tenen vinil de 3mm?", "info", "info"},
		// Con verbo de pedido la '?' es otra cosa: el pedido manda.
		{"quiero 30 stickers de vinil, los tienen?", "pedido", "pedido"},
		{"necesito 2 llaveros mas, tienen?", "pedido", "pedido"},
	}
	for _, c := range casos {
		a := &Analysis{
			Intent: "pedido", Resumen: "prueba",
			Productos: []string{"Miniatura / figura"}, Confianza: 0.9,
		}
		Enrich(a, nil, c.texto)
		if a.Intent != c.quiere {
			t.Errorf("%q: intent = %q, quiere %q", c.texto, a.Intent, c.quiere)
		}

		// Y lo que de verdad importa: no deja linea en el pedido. Un reclamo
		// tampoco, aunque nombre el producto a proposito.
		for _, texto := range []string{c.texto, "se me llego roto el llavero que me entregaste"} {
			got := linea(t, &Analysis{
				Intent: "reclamo", Resumen: "prueba",
				Productos: []string{"Llavero"}, Confianza: 0.9,
			}, texto)
			if got != nil {
				t.Errorf("%q: dejo linea %v en el pedido", texto, got.Productos)
			}
		}
		if c.pertenece == "info" {
			got := linea(t, &Analysis{
				Intent: "pedido", Resumen: "prueba",
				Productos: []string{"Miniatura / figura"}, Confianza: 0.9,
			}, c.texto)
			if got != nil && len(got.Productos) > 0 {
				t.Errorf("%q: una pregunta dejo linea %v en el pedido", c.texto, got.Productos)
			}
		}
	}
}

// El deltasis se aplica sobre una base REAL. Con la pregunta ya clasificada
// como info, "suma 1" funda la linea en 1 y queda en 2, que es lo que el
// cliente pidio en el mensaje: no mas.
func TestHiloJorgeDeltaSobreBaseReal(t *testing.T) {
	var order *Order

	q := &Analysis{Intent: "pedido", Resumen: "Pregunta por Kratos", Confianza: 0.9,
		Productos: []string{"Miniatura / figura"}}
	Enrich(q, nil, "Ustedes hacen un figura de kratos de 15 CM de alto?")
	order = MergeOrder(order, OrderFromAnalysis(uuid.New(), uuid.New(), q, "test"), q.TipoCantidad)
	if *order.Intent != "info" {
		t.Fatalf("la pregunta quedo como %q", *order.Intent)
	}

	d := &Analysis{Intent: "pedido", Resumen: "Suma 1 Kratos", Confianza: 0.9,
		Productos: []string{"Miniatura / figura"}, Cantidades: []int{1},
		TipoCantidad: strp(TipoDelta)}
	order = MergeOrder(order, OrderFromAnalysis(uuid.New(), uuid.New(), d, "test"), d.TipoCantidad)

	if len(order.Productos) != 1 || order.Cantidades[0] != 2 {
		t.Fatalf("pedido = %v %v, quiere [Miniatura / figura] [2]", order.Productos, order.Cantidades)
	}

	// "quiero tambien una figura de Mario": entra como pedido, es el mismo
	// producto del catalogo y no trae cantidad. No se inventa un +1 ni se
	// divide la linea; queda la de Kratos intacta y el pedido a revision, que
	// es lo unico honesto que se puede hacer sin un campo de variante por
	// linea (el modelo no pone "Mario" en ningun campo: deja medidas
	// "23 cm, 15 cm" y la personalizacion vacia).
	mario := &Analysis{Intent: "pedido", Resumen: "Tambien una figura Mario 23cm", Confianza: 0.9,
		Productos: []string{"Miniatura / figura"}}
	Enrich(mario, nil, "quiero tambien una figura de Mario de 23 cm")
	if !Consolida(mario) {
		t.Fatal("un pedido con producto tiene que consolidar")
	}
	order = MergeOrder(order, OrderFromAnalysis(uuid.New(), uuid.New(), mario, "test"), mario.TipoCantidad)

	if len(order.Productos) != 1 || order.Cantidades[0] != 2 {
		t.Errorf("pedido = %v %v, quiere [Miniatura / figura] [2]", order.Productos, order.Cantidades)
	}
	if !order.NeedsReview {
		t.Error("el item sin cantidad tiene que quedar a revision: si no desaparece sin que nadie lo vea")
	}
}

// "quiero tambien una figura de Mario": el modelo no da cantidad y el producto
// ya existe con la misma linea. La cantidad NO se toca (eso seria inventar) y
// el pedido queda a revision, que es la unica forma de que el operador vea que
// hay un item sin resolver.
func TestHiloJorgeItemDistintoSinCantidadQuedaVisible(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere Kratos", Confianza: 0.9,
		Productos: []string{"Miniatura / figura"}, Cantidades: []int{2},
	}, "test")

	mario := &Analysis{Intent: "pedido", Resumen: "Tambien una figura Mario 23cm", Confianza: 0.9,
		Productos: []string{"Miniatura / figura"}}
	Enrich(mario, nil, "quiero tambien una figura de Mario de 23 cm")
	got := MergeOrder(prev, OrderFromAnalysis(uuid.New(), uuid.New(), mario, "test"), mario.TipoCantidad)

	if got.Cantidades[0] != 2 {
		t.Errorf("la cantidad quedo en %d: un item sin cantidad no puede pisar la que ya se sabia", got.Cantidades[0])
	}
	if !got.NeedsReview {
		t.Error("un pedido sin cantidad tiene que quedar a revision: si no, el item desaparece sin que nadie lo vea")
	}
	// Y la accion del operador: el pedido no esta sellado, se puede corregir.
	if got.Edited {
		t.Error("un merge no puede sellar el pedido")
	}
}

// Una fila guardada con productos sin cantidad (o eso, o un PATCH a medias) no
// puede reventar el worker con index out of range: el mensaje se quedaria sin
// consolidar para siempre, con el analysis en 'ok' y el pedido viejo en pantalla.
func TestMergeNoRevientaConCantidadesCortas(t *testing.T) {
	prev := &Order{
		Productos:  []string{"Sticker"},
		Cantidades: []int{}, // fila vieja: el producto existe, la cantidad no
		Detalles:   map[string]string{},
	}
	tipo := TipoDelta
	next := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Suma 2", Confianza: 0.9,
		Productos: []string{"Sticker"}, Cantidades: []int{2},
	}, "test")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MergeOrder reviento con una fila de cantidades corta: %v", r)
		}
	}()

	got := MergeOrder(prev, next, &tipo)
	if len(got.Cantidades) != len(got.Productos) {
		t.Fatalf("desalineado: productos %v cantidades %v", got.Productos, got.Cantidades)
	}
}

// Una pieza que el modelo nombro sin contar NO es una linea con cantidad 1. El
// 1 de alineado es un relleno de indices, y dejarlo pasar convierte "y 12
// llaveros" (que el modelo responde productos [Sticker, Llavero] y cantidades
// [30]) en un pedido de "Llavero 1": un numero que el cliente nunca dijo.
func TestPiezaSinCantidadContadaNoEntraAlPedido(t *testing.T) {
	prev := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 30 stickers", Confianza: 0.9,
		Productos: []string{"Sticker"}, Cantidades: []int{30},
	}, "test")

	// "quiero mas stickers": el producto ya estaba, la cantidad no vino.
	tipo := TipoDelta
	sinNumero := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere mas stickers", Confianza: 0.7,
		Productos: []string{"Sticker"},
	}, "test")
	got := MergeOrder(prev, sinNumero, &tipo)
	if got.Cantidades[0] != 30 {
		t.Errorf("cantidades = %v: 'quiero mas stickers' no es +1, es una duda", got.Cantidades)
	}
	if !got.NeedsReview {
		t.Error("el pedido tiene que quedar a revision: hay una duda sin resolver")
	}

	// Y el caso que lo dispara, tal como responde el modelo a "y 12 llaveros":
	// productos [Sticker, Llavero] y cantidades [30] (el numero del sticker
	// repetido, el del llavero olvidado). La linea del llavero no se crea,
	// porque un 1 seria inventar el numero.
	total := TipoTotal
	parcial := OrderFromAnalysis(uuid.New(), uuid.New(), &Analysis{
		Intent: "pedido", Resumen: "Quiere 12 llaveros", Confianza: 0.8,
		Productos: []string{"Sticker", "Llavero"}, Cantidades: []int{30},
	}, "test")
	got = MergeOrder(prev, parcial, &total)
	if len(got.Productos) != 1 || got.Cantidades[0] != 30 {
		t.Fatalf("pedido = %v %v, quiere [Sticker] [30]: el llavero sin cantidad contada no puede entrar con un 1",
			got.Productos, got.Cantidades)
	}
	if !got.NeedsReview {
		t.Error("un item que nombro producto pero no cantidad tiene que quedar a revision")
	}
}
