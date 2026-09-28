package insight

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

// ---------------------------------------------------------------------------
// Normalizacion de texto
// ---------------------------------------------------------------------------

// normText deja el texto comparable: minusculas, sin acentos, espacios
// colapsados. El cliente escribe "impresión" y el catalogo tiene "impresion";
// sin esto el match falla y el modelo se queda sin catalogo que usar.
func normText(s string) string {
	// NFD primero: "ó" llega como U+00F3 (un solo rune), no como "o" + acento
	// combinante, asi que sin descomponer no hay nada que quitar.
	decomposed := norm.NFD.String(strings.ToLower(s))
	var b strings.Builder
	b.Grow(len(decomposed))
	prevSpace := false
	for _, r := range decomposed {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// isWordChar dice si un rune es parte de una palabra. Todo lo demas (espacio,
// coma, punto, parentesis) es borde legitimo: "en PA, thicken" tiene que matchear
// "pa" igual que "en pa".
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '\''
}

// hasWord busca needle como palabra completa dentro de haystack.
//
// Comparar por palabra entera evita que el alias "pa" (nylon) matchee "papel".
// Ademas tolera el plural: el cliente escribe "macetas" y el catalogo tiene
// "Maceta", asi que "maceta" + s/es cuenta como match.
func hasWord(haystack, needle string) bool {
	n := normText(needle)
	if n == "" {
		return false
	}
	h := normText(haystack)
	runes := []rune(h)
	nrunes := []rune(n)

	// El plural solo aplica a la ultima palabra del alias multi-palabra.
	suffixes := []string{"", "s", "es"}
	for _, suf := range suffixes {
		if matchRunes(runes, append(nrunes, []rune(suf)...)) {
			return true
		}
	}
	return false
}

func matchRunes(hay, needle []rune) bool {
	if len(needle) == 0 || len(needle) > len(hay) {
		return false
	}
	first := needle[0]
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i] != first {
			continue
		}
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if i > 0 && isWordRune(hay[i-1]) {
			continue // pegado a la izquierda: "superpa"
		}
		if end := i + len(needle); end < len(hay) && isWordRune(hay[end]) {
			continue // pegado a la derecha: "3dmake"
		}
		return true
	}
	return false
}

// isMaterialCategory marca los items del catalogo que son materiales: son los
// unicos que tienen sentido en el campo `material`.
func isMaterialCategory(cat string) bool {
	switch strings.ToLower(cat) {
	case "material", "resina":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Enriquecimiento determinista (sin llamar al modelo otra vez)
// ---------------------------------------------------------------------------

// Enrich corrige la salida del modelo contra el catalogo.
//
// Existe por un motivo concreto y medido: granite3.3:2b clasifica el intent
// 10/10 pero deja `material` vacio en ~20% de los casos, aunque el dato SI esta
// en el `resumen` (que si acierta siempre). En vez de gastar una segunda
// llamada al LLM para recuperar un campo, lo leemos del resumen. Es
// deterministico, gratis y testeable.
// Enrich es el enriquecimiento determinista del analysis.
//
// texto es el mensaje CRUDO del cliente (o su transcripcion). Va aparte del
// resumen porque el resumen lo escribe el modelo, y un guardarrail que se apoya
// en texto generado por el mismo modelo que hay que vigilar no vigila nada.
// Con el texto crudo se puede comprobar que las cantidades existen de verdad en
// lo que escribio el cliente.
func Enrich(a *Analysis, items []CatalogItem, texto string) {
	if a == nil {
		return
	}
	a.Productos = canonicalizeProducts(a.Productos, items)
	if a.Material == nil || strings.TrimSpace(*a.Material) == "" {
		if m := findMaterial(a.Resumen, items); m != "" {
			v := m
			a.Material = &v
		}
	}
	saneaCantidades(a)
	saneaTipoCantidad(a, texto)
	desambiguaPregunta(a, texto)
}

// ---------------------------------------------------------------------------
// Cantidades: el 5cm NO es la cantidad
// ---------------------------------------------------------------------------

// quantConUnidadMatchea numeros que en el texto van pegados a una unidad, que
// son medidas y no cantidades de piezas: "5cm", "20x30", "2 kg", "9x5 cm".
var quantConUnidad = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(cm|mm|dm|ml|m\b|g|gr|kg|kgs|lt|l\b|x|"|')`)

// dimCompuesta agarra una dimension completa para poder mostrarla: "9x5 cm",
// "20x30x10", "10 x 20 cm". El patron simple la parte en pedazos, asi que sin
// esto el operador veria medidas = "9x" y no "9x5 cm".
var dimCompuesta = regexp.MustCompile(`(?i)\d+\s*[x×]\s*\d+(?:\s*[x×y]\s*\d+)?\s*(?:cm|mm|dm|m)?`)

// sanaeCantidades saca de `cantidades` los numeros que en el texto del cliente
// son claramente una MEDIDA.
//
// Por que hace falta si el prompt ya lo dice: granite3.3:2b es un modelo de 2
// parametros. En el bench, "2 stickers circulares de 5cm" devolvia
// cantidades:[5] con confianza 0.95, porque el 5 esta mas cerca del producto
// que el 2. Pedirle mas precision por prompt no sirvio. Un guardarrail de 20 lineas
// sobre el texto crudo si sirve, y es deterministico.
//
// Regla: si un numero de `cantidades` aparece en el texto seguido de unidad, es
// medida. Se saca, y si el modelo no habia puesto la medida se completa con
// ella. Ante la duda se deja VACIO y se fuerza needs_review: es mejor que el
// operador complete un dato a que se invente una cantidad equivocada y cotice
// 5 piezas de algo que pidio 2.
func saneaCantidades(a *Analysis) {
	if len(a.Cantidades) == 0 {
		return
	}
	texto := a.Resumen
	if extra := firstNonEmpty(a.Medidas, a.Personalizacion); extra != "" {
		texto += " " + extra
	}

	// Medidas que el cliente escribio de verdad, con su unidad.
	tokens := quantConUnidad.FindAllString(texto, -1)
	esMedida := make(map[int]bool, len(tokens))
	for _, t := range tokens {
		if n, ok := numeroDe(t); ok {
			esMedida[n] = true
		}
	}
	if len(esMedida) == 0 {
		return
	}
	// "10x20 cm" es UNA sola medida, no dos numeros sueltos. El patron simple
	// de arriba la parte en "10x", asi que para mostrarla al operador se usa
	// el patron de dimension completa.
	medidaLegible := ""
	if d := dimCompuesta.FindString(texto); d != "" {
		medidaLegible = d
	}

	out := make([]int, 0, len(a.Cantidades))
	var primeraMedida string
	for _, c := range a.Cantidades {
		if esMedida[c] {
			if primeraMedida == "" {
				primeraMedida = medidaLegible
				if primeraMedida == "" {
					primeraMedida = tokens[0]
				}
			}
			continue
		}
		out = append(out, c)
	}
	if len(out) == len(a.Cantidades) {
		return // el modelo no se equivoco con esta regla
	}

	a.Cantidades = out
	if a.Medidas == nil || strings.TrimSpace(*a.Medidas) == "" {
		v := strings.TrimSpace(primeraMedida)
		a.Medidas = &v
	}
	// Si el modelo se habia quedado sin cantidades (su unico numero era la
	// medida), se recupera el conteo de piezas del texto crudo: "2 stickers de
	// 5cm" son 2 stickers, no 5.
	if len(a.Cantidades) == 0 {
		if n := primerConteoPiezas(texto); n > 0 {
			a.Cantidades = []int{n}
		}
	}
	// El modelo se equivoco con un dato de la lista: el operador tiene que
	// mirarlo, diga lo que diga la confianza que el mismo le puso. NeedsReview
	// dispara con confianza < 0.5, asi que bajar de ahi alcanza para que el
	// badge se encienda sin inventar un campo nuevo en la struct.
	if a.Confianza >= 0.5 {
		a.Confianza = 0.4
	}
}

// ---------------------------------------------------------------------------
// Una pregunta no es un pedido
// ---------------------------------------------------------------------------

// desambiguaPregunta baja a 'info' el analysis que el modelo dejo como pedido
// cuando el cliente en realidad ESTABA PREGUNTANDO.
//
// El caso que la trajo: "Ustedes hacen un figura de kratos de 15 CM de alto?"
// salio como pedido con el producto "Miniatura / figura". Eso crea una linea en
// el consolidado con una unidad que NADIE pidio. Despues el cliente pidio "suma
// 1 figura de kratos" y el delta sumo 1 a ese 1 inventado: el pedido quedo en 2
// cuando el cliente ya tenia 2 y queria 3. El incremento estaba bien; la base
// era fantasma, y una base fantasma convierte todos los deltas siguientes en
// cuentas equivocadas.
//
// Por que es una regla y no una instruccion del prompt: granite3.3:2bya
// clasifica bien la intencion, pero un pedido con un producto sin cantidad es
// justo lo que el prompt no puede proscribir sin inventar numeros. Aca no hace
// falta inventar nada: alcanza con mirar si el cliente escribio un '?' y no
// escribio un verbo de pedido.
//
// El lado dangerouso es el que se elige: 'info' no crea linea de pedido, asi que
// una falsa negativa solo cuesta que el operador lea el mensaje (que siempre
// esta a mano). Al reves, un falso positivo mete una linea con una cantidad
// inventada en el consolidado, y de ahi en adelante todos los deltas se suman
// sobre un numero que no existe.
func desambiguaPregunta(a *Analysis, texto string) {
	if a == nil || a.Intent != "pedido" {
		return
	}
	t := strings.ToLower(strings.TrimSpace(texto))
	if t == "" || !strings.Contains(t, "?") {
		return
	}
	// Con un verbo de pedido escrito, la '?' es del cliente preguntando por
	// otra cosa en la misma frase: "3 stickers? mas 2 llaveros". No se toca.
	if tieneVerboDePedido(t) {
		return
	}
	a.Intent = "info"
}

// tieneVerboDePedido mira el texto CRUDO del cliente, no el resumen del modelo:
// el resumen es texto generado y un guardarraíl que se apoya en la salida del
// mismo modelo que hay que vigilar no vigila nada (mismo criterio que
// sanoaTipoCantidad).
func tieneVerboDePedido(t string) bool {
	// "querer" tambien sirve: "quiero", "quisiera", "querria", "queria".
	for _, v := range []string{
		"quiero", "quiera", "quisiera", "queria", "querria",
		"necesito", "necesitamos", "necesito",
		"suma", "sumale", "sumar", "agrega", "agregale", "agregar",
		"ponme", "pon", "mandame", "manda", "mandome", "hazme", "haz",
		"encarga", "encargue", "encargar", "pedi", "pedido", "pedidos",
		"comprame", "cómprame", "me gustaria", "me gustaría",
		"tengo que", "hay que", "toca", "haganme", "hagasme",
	} {
		if contienePalabra(t, v) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// tipo_cantidad: la cantidad tiene que existir en lo que escribio el cliente
// ---------------------------------------------------------------------------

// sanoaTipoCantidad descarta el tipo cuando la cantidad no aparece en el mensaje
// del cliente.
//
// Por que hace falta si el prompt ya lo dice: granite3.3:2b declara el tipo
// bien (medido 11 de 12), pero a veces se le escapa la cantidad que el cliente
// esta CORRIENDO. Con un pedido de 30 stickers, "y sumale 2 mas" devuelve
// tipo=delta con cantidad 32, que el prompt prohibe explicitamente: el modelo
// hizo la suma por su cuenta. Si el merge le creyera, el pedido pasaria de 30 a
// 62, que es peor que el fallo que venia a arreglar.
//
// Regla: si tipo es "delta", cada cantidad tiene que aparecer LITERALMENTE en el
// mensaje actual. Un incremento es un numero que el cliente acaba de escribir;
// si el modelo devuelve un numero que el cliente no dijo, no es un incremento, es
// una cuenta que el modelo hizo solo y no se aplica.
//
// "total" NO se toca con esta regla: un total corregido puede no estar en el
// mensaje ("dejame 5 de los 30") y aun asi ser correcto. Solo el delta, que es
// donde el error se propaga sumando.
//
// Ante la duda el tipo pasa a null, y MergeOrder trata null como "no se": no
// toca el pedido y marca needs_review. Es el mismo criterio que saneaCantidades,
// y por el mismo motivo: con un modelo de 2B la precision se pide con codigo,
// no con prompt.
func saneaTipoCantidad(a *Analysis, texto string) {
	if a.TipoCantidad == nil || *a.TipoCantidad != TipoDelta {
		return
	}
	if len(a.Cantidades) == 0 {
		a.TipoCantidad = nil
		return
	}
	if strings.TrimSpace(texto) == "" {
		// sin texto crudo no se puede verificar: no se descarta nada, pero no
		// se inventa la certeza tampoco.
		return
	}

	escrito := numerosEnTexto(texto)
	for _, c := range a.Cantidades {
		if !escrito[c] {
			a.TipoCantidad = nil
			if a.Confianza >= 0.5 {
				a.Confianza = 0.4
			}
			return
		}
	}
}

// numerosEnTexto son los enteros que aparecen escritos en el texto, como los
// escribio el cliente. "2 stickers" -> {2}. "32" -> {32}. "2x3" -> {2,3}.
func numerosEnTexto(texto string) map[int]bool {
	out := map[int]bool{}
	for _, m := range regexp.MustCompile(`\d+`).FindAllString(texto, -1) {
		if n, err := strconv.Atoi(m); err == nil {
			out[n] = true
		}
	}
	return out
}

// numeroSeguidoDePalabra agarra "2 stickers", "25 llaveros", "3 macetas".
var numeroSeguidoDePalabra = regexp.MustCompile(`(?i)(\d+)\s+([a-záéíóúüñ]+)`)

// palabrasQueNoSonUnidad son las que aparecen entre un numero y su unidad, o
// conectores que hacen que un numero NO sea una cantidad: "15 de mayo" (fecha),
// "de 8 a 10 cm" (rango), "x2" (veces).
var palabrasQueNoSonUnidad = map[string]bool{
	"de": true, "del": true, "la": true, "el": true, "los": true, "las": true,
	"a": true, "al": true, "en": true, "para": true, "por": true, "con": true,
	"y": true, "o": true, "u": true, "mas": true, "menos": true, "veces": true,
	"unidad": true, "unidades": true, "hora": true, "horas": true, "dia": true,
	"dias": true, "semana": true, "mes": true, "meses": true, "ano": true,
}

// primerConteoPiezas devuelve el primer numero del texto que va seguido de una
// palabra normal (un sustantivo), o sea un conteo de piezas y no una medida,
// fecha ni rango. 0 si no hay ninguno.
func primerConteoPiezas(texto string) int {
	for _, m := range numeroSeguidoDePalabra.FindAllStringSubmatch(texto, -1) {
		siguiente := normText(m[2])
		if palabrasQueNoSonUnidad[siguiente] {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// numeroDe saca el entero inicial de un token tipo "5cm" o "20x30".
func numeroDe(tok string) (int, bool) {
	n := 0
	digits := 0
	for _, r := range strings.TrimSpace(tok) {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		digits++
	}
	// 5,5cm -> 5 no es una medida util como cantidad, pero 20x30 -> 20 si.
	return n, digits > 0 && n > 0
}

func firstNonEmpty(vals ...*string) string {
	for _, v := range vals {
		if v != nil && strings.TrimSpace(*v) != "" {
			return strings.TrimSpace(*v)
		}
	}
	return ""
}

// canonicalizeProducts mapea las palabras del cliente a los nombres del
// catalogo y saca duplicados. Si no hay match, deja el texto del cliente.
func canonicalizeProducts(products []string, items []CatalogItem) []string {
	out := []string{}
	seen := map[string]bool{}

	for _, raw := range products {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if canon, ok := matchCatalog(p, items, nil); ok {
			p = canon
		}
		key := normText(p)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

// findMaterial busca en el texto un material del catalogo. Devuelve el nombre
// canonico del primero que aparezca.
func findMaterial(text string, items []CatalogItem) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if canon, ok := matchCatalog(text, items, isMaterialCategory); ok {
		return canon
	}
	return ""
}

// matchCatalog busca el item del catalogo que el texto menciona. categoryFn
// filtra que categorias son validas (nil = todas).
func matchCatalog(text string, items []CatalogItem, categoryFn func(string) bool) (string, bool) {
	trimmed := strings.TrimSpace(text)
	for _, it := range items {
		if categoryFn != nil && !categoryFn(it.Category) {
			continue
		}
		if hasWord(trimmed, it.Name) {
			return it.Name, true
		}
		for _, alias := range it.Aliases {
			if hasWord(trimmed, alias) {
				return it.Name, true
			}
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// needs_review: reglas deterministas, no opinion del modelo
// ---------------------------------------------------------------------------

// Consolida dice si un analysis puede tocar el pedido consolidado del hilo.
//
// El pedido es lo que hay que fabricar y cobrar, y solo lo crea un 'pedido'. Un
// reclamo ("se me llego roto el llavero") o una pregunta ("hacen envios?")
// pueden nombrar productos: si se consolidaran, sumarian lineas que el cliente
// no pidio. Por ahi el hilo de Jorge Mujica empezo con una pregunta clasificada
// como pedido, que le creo una linea de base fantasma sobre la que despues todos
// los deltas se sumaron mal.
//
// Los analyses no-'pedido' no se pierden: quedan en el historial de la
// conversacion y se ven en la burbuja del mensaje.
func Consolida(a *Analysis) bool {
	return a != nil && a.Intent == "pedido"
}

// NeedsReview decide si el operador tiene que mirar el analisis a mano.
// Son reglas, no feelings: asi el badge no parpadea segun como salio la
// generacion.
//
//   - confianza baja: el modelo mismo dijo que no esta seguro
//   - pedido sin producto: casi siempre falta leer el mensaje
//   - pedido sin material: en un taller de impresion 3D el material define el
//     precio, asi que sin material el pedido esta incompleto
//   - resumen vacio: no hay nada que mostrarle al operador
func NeedsReview(a *Analysis) bool {
	if a == nil {
		return false
	}
	if strings.TrimSpace(a.Resumen) == "" {
		return true
	}
	if a.Confianza < 0.5 {
		return true
	}
	if a.Intent == "pedido" {
		if len(a.Productos) == 0 {
			return true
		}
		if a.Material == nil || strings.TrimSpace(*a.Material) == "" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Consolidacion del pedido por conversacion
// ---------------------------------------------------------------------------

// Order es la vista consolidada de la conversacion (conversation_orders).
type Order struct {
	ConversationID  uuid.UUID         `json:"conversation_id"`
	Intent          *string           `json:"intent"`
	Resumen         *string           `json:"resumen"`
	Productos       []string          `json:"productos"`
	Cantidades      []int             `json:"cantidades"`
	Detalles        map[string]string `json:"detalles"`
	Confianza       *float64          `json:"confianza"`
	NeedsReview     bool              `json:"needs_review"`
	SourceMessageID *uuid.UUID        `json:"source_message_id"`
	Model           *string           `json:"model"`
	Edited          bool              `json:"edited"`

	// Historial. El pedido de un hilo es una cadena de revisiones: la vigente
	// (is_current) es la que ve el operador y en la que la IA escribe. Las
	// anteriores quedan para consultar, no se editan ni las vuelve a tocar
	// nadie. Solo el humano crea revisiones.
	Revision     int        `json:"revision"`
	IsCurrent    bool       `json:"is_current"`
	SupersededAt *time.Time `json:"superseded_at"`
	SupersededBy *uuid.UUID `json:"superseded_by"`

	// CantidadesContadas[i] dice si cantidades[i] la dijo el modelo o es el 1
	// de relleno de alineado. Vive solo en memoria: la fila no lo guarda porque
	// la regla es "una pieza sin cantidad contada no entra al pedido", y eso se
	// decide al consolidar, no al mostrar.
	//
	// Sin esto hay que distinguir "1" de "no dijo": el 1 es un relleno para que
	// los indices de productos y cantidades no se corran, y dejarlo pasar como
	// cantidad convierte "y 12 llaveros" (productos [Sticker, Llavero],
	// cantidades [30]) en un pedido de "Llavero 1".
	CantidadesContadas []bool `json:"-"`
}

// Detalle booleano derivado por palabras: el modelo no tiene un campo `envio`
// (porque "envio" puede ser pregunta o pedido) pero el operador lo necesita
// para cotizar. Se resuelve con una lista corta y visible.
// envioStems son RAICES, no palabras: el cliente escribe "se las envien",
// "que la envien", "entregar", "lo retiro", "delivery" y una lista de palabras
// exactas se comia la mitad.
var envioStems = []string{
	"envi", "entreg", "delivery", "correo", "kurier", "courier", "retir", "repart",
}

func mentionsEnvio(text string) bool {
	n := normText(text)
	for _, stem := range envioStems {
		if strings.Contains(n, stem) {
			return true
		}
	}
	return false
}

// OrderFromAnalysis arma el pedido consolidado a partir del analisis de un
// mensaje, dejando las claves del catalogo ya resueltas.
func OrderFromAnalysis(convID, msgID uuid.UUID, a *Analysis, model string) *Order {
	intent := a.Intent
	resumen := strings.TrimSpace(a.Resumen)
	conf := a.Confianza

	detalles := map[string]string{}
	if a.Material != nil && strings.TrimSpace(*a.Material) != "" {
		detalles["material"] = strings.TrimSpace(*a.Material)
	}
	if a.Medidas != nil && strings.TrimSpace(*a.Medidas) != "" {
		detalles["medidas"] = strings.TrimSpace(*a.Medidas)
	}
	if a.Personalizacion != nil && strings.TrimSpace(*a.Personalizacion) != "" {
		detalles["personalizacion"] = strings.TrimSpace(*a.Personalizacion)
	}
	if a.FechaEntrega != nil && strings.TrimSpace(*a.FechaEntrega) != "" {
		detalles["fecha_entrega"] = strings.TrimSpace(*a.FechaEntrega)
	}
	// El "hacen envios?" del cliente cuenta: se mira el resumen, que es donde
	// el modelo siempre deja lo que el cliente dijo.
	if mentionsEnvio(a.Resumen) {
		detalles["envio"] = "si"
	}

	// ---------------------------------------------------------------------------
	// NO hay identidad de variante por linea, y es a proposito.
	//
	// "figura de Kratos 15cm" y "figura de Mario 23cm" son el mismo producto
	// del catalogo y dos piezas distintas del pedido, y con un array de
	// productos plano no hay donde meter el "de Mario". Se probo etiquetando
	// la linea ("Miniatura / figura (Kratos 15cm)") y partir por variante, y
	// hace las dos cosas mal: "suma 1 figura de kratos" (que no repite la
	// medida) abre una linea nueva en vez de sumar, y el caso que lo motivo
	// tampoco lo agarra, porque granite3.3:2b no pone "Mario" en ningun campo
	// estructurado: deja medidas "23 cm, 15 cm" y la personalizacion vacia.
	//
	// Sin una senal confiable de variante, cualquier heuristica parte pedidos
	// que son el mismo item y deja pasar el que importa. Asi que la linea se
	// identifica por producto, y lo que no se puede decidir queda marked para
	// el operador en vez de repartido al azar entre dos lineas.

	// Las cantidades van SIEMPRE en la misma posicion que su producto. Una
	// linea sin cantidad no es representable en el array, y el desajuste no
	// queda en un numero feo: en MergeOrder, cantidades[pos] sobre un array
	// corto revienta el worker con index out of range y el mensaje se queda sin
	// consolidar nunca mas.
	//
	// alineado() completa con 1 las piezas que el modelo nombro sin contar, y
	// ese 1 NO es un dato: es un relleno para que los indices no se corran. Por
	// eso mergeLines consulta las posiciones reales antes de sumar. Un analysis
	// con productos y sin cantidades es comun ("y 12 llaveros" con productos
	// [Sticker, Llavero] y cantidades [30]): completar con 1 ponia "Llavero 1"
	// en un pedido de 12, que es justo el fallo que este modulo no puede
	// cometer. La linea sin cantidad contada no entra al pedido y el pedido
	// queda a revision; el item sigue en el historial del analysis, con "Traer
	// al pedido", para que el operador lo ponga.
	cants, reales := alineado(a.Productos, a.Cantidades)
	hayDudas := NeedsReview(a)
	for _, v := range reales {
		hayDudas = hayDudas || !v
	}

	return &Order{
		ConversationID: convID,
		Intent:         &intent,
		Resumen:        &resumen,
		Productos:      a.Productos,
		Cantidades:     cants,
		// los flags se guardan al reves (1 = relleno) porque alineado los
		// devuelve asi; aca se leen al derecho
		CantidadesContadas: invertidos(reales),
		Detalles:           detalles,
		Confianza:          &conf,
		NeedsReview:        hayDudas,
		SourceMessageID:    &msgID,
		Model:              &model,
	}
}

// alineado deja cantidades con la misma longitud que productos, completando
// con 1 lo que falte, y dice que posiciones las dijo el modelo de verdad.
//
// El 1 de relleno NO es una cantidad: existe solo para que los indices de
// productos y cantidades no se corran (MergeOrder indexa los dos en paralelo).
// Por eso se devuelven aparte, en absoluto, para que mergeLines pregunte "esta
// cantidad la dijo el modelo?" antes de sumar o reemplazar nada.
func alineado(productos []string, cantidades []int) ([]int, []bool) {
	tot := len(productos)
	if tot == 0 {
		return []int{}, nil
	}

	out := make([]int, 0, tot)
	out = append(out, cantidades...)
	// Si el modelo devolvio mas cantidades que productos, las de mas no tienen
	// linea a la que pertenecer.
	if len(out) > tot {
		out = out[:tot]
	}

	// alineado completa por el final. Los indices se marcan en absoluto para
	// que mergeLines pueda preguntar "esta cantidad la dijo el modelo?" por
	// posicion y no tenga que saber cuantas se.agregaron.
	var inferidas []bool
	for len(out) < tot {
		out = append(out, 1)
		if inferidas == nil {
			inferidas = make([]bool, tot)
		}
		inferidas[len(out)-1] = true
	}
	return out, inferidas
}

// invertidos pasa los flags de alineado (true = relleno) a los del Order
// (true = contado de verdad). Son el mismo dato al reves, y el nombre del
// Order es el que se lee en mergeLines.
func invertidos(inferidas []bool) []bool {
	if inferidas == nil {
		return nil
	}
	out := make([]bool, len(inferidas))
	for i, v := range inferidas {
		out[i] = !v
	}
	return out
}

// MergeOrder consolida el pedido nuevo sobre el anterior del mismo hilo.
//
// Esto es la parte que hace util el consolidado: el cliente escribe "quiero una
// maceta" en el mensaje 1, "en PETG por favor" en el 2 y "son 3 y con logo" en
// el 3. El operador tiene que ver las tres cosas juntas, no la ultima.
//
// tipo es lo que declaro el modelo en Analysis.TipoCantidad (delta o total, ver
// migracion 000016) y es LA DECISION de si la cantidad se suma o reemplaza. No
// se deduce del texto, a proposito: el modelo es inconsistente. Ante "sumale 2
// stickers mas" con 30 ya pedidos devuelve a veces 32 (total) y a veces 2
// (delta); medido sobre granite3.3:2b, 5 de 8 redacciones naturales dan el
// total y las otras 3 dan el incremento. Con el merge reemplazando siempre,
// esas 3 dejaban el pedido en 2 stickers, sin error ni warning y con confianza
// 0.9: 28 piezas perdidas en silencio. Que lo decida el codigo y no el modelo es
// lo que hace que esto sea confiable.
//
// Reglas, en orden de precedencia:
//   - next nil: se conserva prev
//   - prev nil: entra el nuevo tal cual. No hay linea a la que sumarle, y
//     "sumale 3 stickers" en un hilo sin pedido es un pedido de 3, no un error
//   - prev editado por un humano: NO se toca (gana el operador, siempre)
//   - el mensaje no nombra producto: se conservan productos y cantidades
//   - tipo nil con cantidades: no se pisa la cantidad y se marca needs_review.
//     Es "no sabemos si esto suma o reemplaza": un analysis anterior a la
//     000016, o un valor que el modelo invento. Tocar a ciegas el pedido
//     consolidado es peor que dejar la duda a la vista del operador
//   - tipo total sobre un producto que ya esta: el numero ES el total de esa
//     linea, la reemplaza. Tambien cubre la correccion de medidas, que por
//     definicion reemplaza la linea y no la suma
//   - tipo delta sobre un producto que ya esta: se SUMA a esa linea
//   - producto que no estaba: se agrega al final con la cantidad pedida, sea
//     delta o total. Sumar a una linea inexistente es crearla con ese numero
//
// detalles: gana el nuevo cuando trae el campo, si no se conserva el viejo.
// intent y resumen: siempre gana el ultimo mensaje.
func MergeOrder(prev, next *Order, tipo *string) *Order {
	if next == nil {
		return prev
	}
	if prev == nil {
		return next
	}
	if prev.Edited {
		return prev
	}

	out := *next
	out.Edited = false
	out.SourceMessageID = next.SourceMessageID
	out.Model = next.Model

	merged := map[string]string{}
	for k, v := range prev.Detalles {
		merged[k] = v
	}
	for k, v := range next.Detalles {
		merged[k] = v
	}
	out.Detalles = merged

	productos, cantidades := mergeLines(prev, next, tipo, &out)
	out.Productos = dedupeStrings(productos, 20)
	out.Cantidades = capInts(cantidades, 20)

	// El resumen del hilo tiene que ser legible de un vistazo: si el mensaje
	// nuevo no agrego producto, mantener el resumen anterior es mas util que
	// reemplazarlo por algo mas corto.
	if len(normSet(next.Productos)) == 0 && next.Resumen != nil && normText(*next.Resumen) == "" {
		out.Resumen = prev.Resumen
	}

	return &out
}

// mergeLines resuelve el array de productos y el de cantidades, que son la
// unica parte del consolidado con historia: el resto de variables "gana el
// ultimo" y no necesita mirar atras.
//
// Las lineas se emparejan POR PRODUCTO, no por posicion: el modelo puede
// devolver los productos en otro orden o repetir uno, y las cantidades van en el
// MISMO indice que su producto. Un merge posicional en esos casos produce
// pedidos con la cantidad pegada al producto equivocado, que es peor que no
// tener cantidades.
func mergeLines(prev, next *Order, tipo *string, out *Order) ([]string, []int) {
	if len(normSet(next.Productos)) == 0 {
		// el mensaje no nombro producto: no se pisa lo que ya sabiamos
		return prev.Productos, prev.Cantidades
	}

	productos := append([]string{}, prev.Productos...)

	// indice por producto normalizado -> posicion en el pedido vigente
	idx := map[string]int{}
	for i, p := range prev.Productos {
		if k := normText(p); k != "" {
			if _, dup := idx[k]; !dup {
				idx[k] = i
			}
		}
	}

	// Una fila guardada antes de alineado() puede tener menos cantidades que
	// productos. Sin esto, cantidades[pos] += q revienta con index out of
	// range: un panic en el worker, y el mensaje se queda sin consolidar para
	// siempre con el analysis en 'ok' y el pedido viejo en pantalla.
	cantidades, _ := alineado(prev.Productos, prev.Cantidades)

	// Un 1 de relleno NO cuenta como cantidad: es el que produce las lineas con
	// numero inventado. Los flags vienen del analysis (ver
	// Order.CantidadesContadas); si no vinieron, la cantidad vale, porque solo
	// hay dos formas de armarlos y las dos pasan por OrderFromAnalysis.

	ambiguo := false
	for i, p := range next.Productos {
		k := normText(p)
		if k == "" {
			continue
		}
		// La cantidad tiene que existir Y que la haya dicho el modelo. Sin las
		// dos cosas, este mensaje no suma nada: niincrementa, ni reemplaza, ni
		// crea la linea. Un item sin cantidad contada no es un pedido que se
		// pueda cotizar, y poner un 1 (o un 0) seria mostrarle al operador un
		// numero que el cliente nunca dijo.
		q, tieneQ := 0, false
		if i < len(next.Cantidades) {
			q, tieneQ = next.Cantidades[i], true
		}
		if tieneQ && i < len(next.CantidadesContadas) && !next.CantidadesContadas[i] {
			tieneQ = false
		}

		pos, existe := idx[k]
		switch {
		case !tieneQ:
			// No se inventa ni se pisa nada. Si la linea es nueva, el item
			// queda fuera del pedido: visible en el historial del analysis, con
			// "Traer al pedido", y el pedido marcado para que el operador lo
			// mire. Es la unica salida honesta para "quiero tambien una figura
			// de Mario", que el modelo no cuenta y no separa del mismo
			// producto del catalogo.
			ambiguo = true

		case !existe:
			idx[k] = len(productos)
			productos = append(productos, p)
			cantidades = append(cantidades, q)

		case tipo == nil:
			ambiguo = true

		case *tipo == TipoDelta:
			cantidades[pos] += q

		default: // TipoTotal
			cantidades[pos] = q
		}
	}

	if ambiguo {
		// La duda queda a la vista en el pedido. Es lo que el operador
		// necesita para decidir; ocultarla seria volver al fallo que esta
		// funcion arregla.
		out.NeedsReview = true
	}

	return productos, cantidades
}

// contienePalabra busca un termino como palabra completa, no como pedazo de
// otra. Sin limites de palabra, "pon" matchea "ponsé" y "suman" matchea
// "suman"... que es justo lo que no se quiere.
func contienePalabra(t, termino string) bool {
	idx := strings.Index(t, termino)
	for idx >= 0 {
		antes := idx == 0 || !esLetra(rune(t[idx-1]))
		fin := idx + len(termino)
		despues := fin >= len(t) || !esLetra(rune(t[fin]))
		if antes && despues {
			return true
		}
		sig := strings.Index(t[idx+1:], termino)
		if sig < 0 {
			return false
		}
		idx = idx + 1 + sig
	}
	return false
}

func esLetra(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
		(r >= 'A' && r <= 'Z') || r > 127 // acentos y ñ, para no partir palabras
}

func normSet(in []string) map[string]bool {
	m := make(map[string]bool, len(in))
	for _, s := range in {
		if k := normText(s); k != "" {
			m[k] = true
		}
	}
	return m
}

func dedupeStrings(in []string, max int) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		k := normText(s)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	return out
}

func capInts(in []int, max int) []int {
	if len(in) > max {
		return in[:max]
	}
	return in
}
