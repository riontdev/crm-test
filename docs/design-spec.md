# SocialCRM — Especificación de Diseño (Kinetic CRM)

> Fuente de verdad única para todo el redesign. Extraído del design system de Stitch
> ("Social Media CRM Dashboard" / Kinetic CRM) + HTML estructural de "Bandeja de Entrada (Final)".
> Todos los agentes DEBEN usar estos tokens sin desviarse.

## 1. Tokens de color (modo claro)

| Token | Valor | Uso |
|---|---|---|
| `primary` | `#0F172A` (Deep Slate) | Navegación, headers, texto principal |
| `secondary` | `#38BDF8` (Sky Blue) | Acciones primarias, estados activos, burbujas salientes |
| `tertiary` / accent | `#6366F1` (Indigo) | Detalles, links, indicadores secundarios |
| `neutral` | `#94A3B8` | Texto terciario, iconos apagados |
| `background` | `#F8F9FF` | Fondo general (L0) |
| `surface` / cards | `#FFFFFF` | Cards, paneles (L1) |
| `on-surface` | `#0D1C2D` | Texto principal |
| texto secundario | `#45464D` | Descripciones, placeholders |
| `border` | `#E2E8F0` | Bordes 1px solo cuando haga falta definición |
| outline-variant suave | `#C6C6CD` | Divisores sutiles |
| `error` | `#BA1A1A` | Errores, acciones destructivas |
| success/status | `#10B981` | Badges: fondo al 10% opacidad + texto full-color |

### Acentos de canal
Solo en elementos pequeños (iconos, badges, bordes finos) para no chocar con la marca:
| Canal | Color |
|---|---|
| WhatsApp | `#25D366` |
| Instagram | `#E1306C` |
| Facebook/Messenger | `#1877F2` |

## 2. Modo oscuro (derivado, coherente)

| Token | Valor |
|---|---|
| `background` | `#0B1220` |
| cards/surface | `#101828` |
| texto principal | `#E5EDF8` |
| texto secundario | `#94A3B8` |
| `border` | `#1E293B` |
| burbuja saliente | `#0284C7` (sky-600) |
| burbuja entrante | `#1E293B` |

Toggle dark/light en TopBar (`dark_mode` / `light_mode`). Clase `.dark` sobre `<html>`.

## 3. Tipografía — Inter exclusivamente

| Nivel | Tamaño/línea | Peso | Tracking | Uso |
|---|---|---|---|---|
| display-lg | 36/44px | 700 | -0.02em | KPIs, números grandes |
| headline-md | 24/32px | 600 | -0.01em | Títulos de página |
| headline-sm | 20/28px | 600 | — | Títulos de sección |
| body-lg | 16/24px | 400 | — | Texto destacado |
| body-md | 14/20px | 400 | — | Cuerpo estándar |
| label-md | 12/16px | 600 | +0.05em uppercase | Labels/meta-info |
| label-sm | 11/14px | 500 | — | Timestamps, micro-texto |

Datos numéricos en KPIs: weight 700, tracking -0.02em.

## 4. Espaciado y forma

- Escala 8px: `xs` 4 · `sm` 8 · `md` 16 (padding de cards) · `lg` 24 (gutter de página) · `xl` 32
- Sidebar: **260px fijo** · max-content-width: 1440px
- Radios: `sm` 4px · **default 8px** (botones/inputs/cards chicas) · `md` 12px · `lg` 16px (contenedores grandes/feed) · `pill` 9999px (badges/tags)

## 5. Elevación

| Nivel | Superficie | Sombra |
|---|---|---|
| L0 | Fondo `#F8F9FF` | — |
| L1 | Cards blancas | `0 1px 3px rgba(0,0,0,.1), 0 1px 2px rgba(0,0,0,.06)` |
| L2 | Modals/dropdowns | Sombra pronunciada + backdrop blur 8px |

Evitar bordes pesados; borde 1px `#E2E8F0` únicamente para definición contra blanco.

## 6. Componentes

### Botones
- **Primary**: bg sky blue `#38BDF8`, texto blanco, radius 8px
- **Secondary**: transparente + borde 1px slate
- **Ghost**: sin bg ni borde, para acciones de baja prioridad
- Focus ring visible en todos.

### Sidebar
- 260px fijo, iconos stroke (Material Symbols Outlined)
- Item activo: fondo primary al 5% + píldora vertical de 4px en el borde izquierdo

### TopBar
- Search global, notificaciones con dot rojo, toggle `dark_mode`

### Burbujas de chat
- **Entrante**: `#F1F5F9` texto slate, alineada izquierda
- **Saliente**: `#38BDF8` texto blanco, alineada derecha; ticks `done_all` azules cuando leído
- Timestamp `label-sm`; icono pequeño de canal en la burbuja

### Badges de estado
Pill con bg del color al 10% y texto full-color.

### Chips de acción rápida
Pill con icono `bolt`, borde sutil, hover eleva ligeramente.

### Estados vacíos y carga
Skeletons `animate-pulse`; empty states con icono + título + descripción.

## 7. Estructura del Inbox (referencia Stitch)

```
┌──────────┬──────────────────┬─────────────────────┬──────────────┐
│ Sidebar  │ Lista conversac. │ Chat                │ Panel contacto│
│ 260px    │ header+tabs      │ header c/estado     │ avatar+nombre │
│ nav items│ Todos/No leídos/ │ separadores día     │ compañía      │
│ + badges │ WhatsApp         │ burbujas + ticks    │ tabs Profile  │
│          │ cards: avatar,   │ chips acción rápida │ Contact Info  │
│          │ nombre, tiempo,  │ composer attach+send│ CRM Details   │
│          │ preview, tag     │                     │ status+tags   │
└──────────┴──────────────────┴─────────────────────┴──────────────┘
```

- Lista: header "Inbox" + filtro; tabs **Todos / No leídos / WhatsApp**; card = avatar coloreado, nombre, tiempo relativo, preview 1 línea, tag de categoría
- Chat header: avatar iniciales, nombre, subtítulo "Activo ahora en WhatsApp", menú `more_vert`
- Mensajes agrupados por día ("Hoy"); composer con `attach_file` + input + `send`
- Panel contacto: avatar grande, nombre, compañía; tabs Profile/Edit; secciones **Información de Contacto** (email/teléfono), **Detalles CRM** (status + tags pills), **Pedido** (`OrderCard`) y **Responsable**

## 8. Análisis de pedidos (IA local)

Todo el módulo es **información, nunca acción**: no hay botón de "responder
automático" en ninguna pantalla. El operador lee, corrige y sigue contestando
a mano.

### Badge de intención (`OrderBadge`)

Pill de 10-11px que va junto al `ChannelBadge` en la lista del inbox.

| Intención | Color | Label |
|---|---|---|
| `pedido` | índigo | Pedido |
| `reclamo` | **rojo** | Reclamo |
| `info` | cielo | Info |
| `otro` | gris pizarra | Otro |

`reclamo` es el único estado que hay que mirar ya, por eso no comparte color
neutro con `info`. Cuando `needs_review` se agrega `· revisar` y el icono pasa a
`alert_triangle`.

`order_pending` (el cliente pidió algo más y todavía no se consolidó) **le gana a
`needs_review`**: pasa a `undo_2`, suma `· nuevo pedido` y un anillo ámbar
`ring-1 ring-amber-500/60`. Uno es una decisión del operador que se pierde si no
se ve; el otro es una duda de la IA. El color del texto sigue diciendo el tipo
de pedido, el anillo dice "esto necesita una decisión".

### Tarjeta de pedido (`OrderCard`)

Va en el panel de contacto, entre "Detalles CRM" y "Responsable".

- **Lectura:** badge, chip `Lock` + "Editado" si el humano corrigió, chip `v{n}`
  si el hilo tiene más de una revisión, resumen, productos como pills índigo con
  `cantidad×` en tabular-nums, detalles en `dl`, pie con confianza / modelo /
  fecha y links "Ver historial" y "Ver versiones (n)".
- **Aviso ámbar:** si `needs_review` o confianza < 70%, una banda ámbar explica
  que hay que revisarlo y que al corregirlo queda fijado.
- **Edición:** textarea de resumen, selects de intención y estado, input de
  material, filas de producto con cantidad + nombre + borrar, y al pie un
  recordatorio "Al guardar, la IA deja de tocar este pedido". Guardar es la
  única acción que sella `edited`.

La confianza baja se marca en ámbar pero **nunca se oculta**: el operador tiene
que ver que el modelo no estaba seguro.

#### Pedido pendiente: el banner que no se puede pasar por alto

Un pedido sellado (`edited=true`) ya no lo toca la IA. Cuando el cliente pide
algo más, ese analysis nuevo se guarda pero no se consolida, y el pedido
vigente queda viejo. Sin una salida, el operador mira un pedido viejo y cree que
ya lo vio todo: **el trabajo nuevo se pierde en silencio**.

Por eso el pendiente va **primero en la tarjeta**, con borde ámbar, no como una
nota al pie:

```
┌─────────────────────────────────────────────┐
│ ↪ El cliente pidió algo más hace 12 min     │  undo_2, ámbar, borde amber-400
│ "Suma un sticker de 5 cm"                   │  la cita del cliente, text-sm
│ [3× Sticker]                                │  pills ámbar con cantidad×
│ medidas: 5 cm · personalización: Kratos       │  details en 11px
│ [✓ Aplicar como pedido vigente] [🔓 Reabrir]│  2 acciones, la 2da ghost
└─────────────────────────────────────────────┘
```

- **Aplicar** (primary) crea la revisión siguiente; la anterior queda archivada
  en el historial. Gana el humano sobre lo que dijo.
- **Reabrir** (ghost) deja que la IA actualice **esta misma** revisión y saca el
  sello. Solo aparece si el pedido está editado: una revisión que creó la IA no
  está "fijada" y no hay nada que reabrir.

Debajo, una línea de 10px explica la diferencia entre los dos caminos, porque no
son sinónimos: *"Aplicar guarda este pedido en el historial y crea la revisión
siguiente. Reabrir deja que la IA lo actualice sobre esta misma revisión."*

Con `order.edited && !pending`, el botón `lock_open` queda también junto al
lápiz, para poder devolver el pedido a la IA cuando el operador cambió de idea.

#### Timeline de revisiones

Solo si hay más de una (`revisions.length > 1`); con una sola el botón no tiene
nada que decir. Cada fila lleva `v{n}` en tabular-nums, sello "Vigente" en índigo,
`lock` "editado" si el humano la tocó, y "Reemplazada el {fecha}" en las
archivadas. La corrección del humano sobrevive porque vive en la fila archivada,
no en el pendiente.

### Burbuja de audio

Debajo del `<audio>`, en tipografía de 11px y separador `border-current/10`:
la transcripción en itálica con ícono `graphic_eq`, y el resumen etiquetado
`IA`. Si la IA todavía corre, `progress_activity` animado + "Analizando…".

Se pinta **sin refetch** cuando llega el evento `insight.transcript`: leer lo
que dijo el cliente no puede esperar a los ~5s del modelo.

### Vista Pedidos (`/orders`)

Lista de cards ordenadas por pedido: contacto, canal, badge, chips de producto
y select de estado en línea. Selectores de estado, intención y búsqueda de
texto arriba; paginación abajo. Encabezado con el recordatorio "Corregí uno y
queda fijado: la IA no lo vuelve a tocar". Si el módulo está apagado, la vista
lo dice y no muestra una lista vacía sin explicación.

La lista muestra **una fila por hilo**, la vigente: las revisiones anteriores se
piden explícitas y no se mezclan en el listado. El punto ámbar del badge avisa de
que ese hilo tiene un pedido pendiente, que se resuelve en el panel de contacto.

### Vista Catálogo (`/catalog`)

Tabla agrupada por categoría, 45 productos. Alta arriba, filas con
nombre + alias + acciones (editar / activar / eliminar). Editar es inline.
Pie con el conteo de productos activos; si el módulo está apagado, avisa que el
catálogo se puede editar pero la IA no lo está usando.

### Panel de control (en `AgentsView`)

Tarjeta "Análisis de pedidos" **antes** de los agentes de chat, para que nadie
confunda "agente que responde" con "IA que resume":

- Interruptor maestro (índigo) con estado Activo/Pausado.
- Banner de salud según `ok` / `degraded` / `off`: si el modelo no está
  cargado muestra el comando `ollama pull` exacto.
- 6 contadores: analizados, pendientes, procesando, omitidos, con error,
  para revisar.
- Interruptor de notas de voz (esmeralda) con el modelo de Whisper.
- Chips por canal (toggle) + "Analizar lo que falta".

## 9. Iconografía

Material Symbols Outlined via Google Fonts, clase CSS `material-symbols-outlined`.

Iconos del diseño: `dashboard, inbox, hub, description, bar_chart, group, settings, help, logout, search, notifications, dark_mode, light_mode, filter_list, chat, mail, send, more_vert, bolt, attach_file, done_all, arrow_back, close, add, mood, check`

Los componentes nuevos del análisis usan `lucide-vue-next` (`PackageCheck`,
`AlertTriangle`, `Lock`, `Mic`, `Pencil`, `Save`, `Trash2`, `X`, `Check`) porque
el resto del código nuevo ya lo usa; conviven los dos sets sin conflicto.

## 10. Reglas transversales

1. UI text 100% **español**
2. Dark mode desde el día 1 (todos los componentes con variantes `dark:`)
3. Inter como única fuente; labels meta en uppercase con tracking
4. Acentos de canal contenidos en elementos pequeños
5. Sin librerías externas de componentes — Tailwind utilities + composables propios
