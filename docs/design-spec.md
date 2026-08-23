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
- Panel contacto: avatar grande, nombre, compañía; tabs Profile/Edit; secciones **Información de Contacto** (email/teléfono) y **Detalles CRM** (status + tags pills)

## 8. Iconografía

Material Symbols Outlined via Google Fonts, clase CSS `material-symbols-outlined`.

Iconos del diseño: `dashboard, inbox, hub, description, bar_chart, group, settings, help, logout, search, notifications, dark_mode, light_mode, filter_list, chat, mail, send, more_vert, bolt, attach_file, done_all, arrow_back, close, add, mood, check`

## 9. Reglas transversales

1. UI text 100% **español**
2. Dark mode desde el día 1 (todos los componentes con variantes `dark:`)
3. Inter como única fuente; labels meta en uppercase con tracking
4. Acentos de canal contenidos en elementos pequeños
5. Sin librerías externas de componentes — Tailwind utilities + composables propios
