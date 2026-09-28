# CRM Multicanal — AGENTS.md

## Qué es
Una bandeja única donde entran y se responden WhatsApp, Instagram DM y Facebook Messenger, con un agente de IA configurable por canal que contesta solo.

## Stack
- **Backend:** Golang (framework Echo) — SIN TypeScript en el backend
- **Frontend:** Vue.js with Pinia state manager + TypeScript + Tailwind 4 + shadcn/ui
- **Database:** Supabase (Postgres + Auth + Storage), migraciones SQL manuales con golang-migrate
- **Proveedor de canales:** Zernio — docs.zernio.com
- **Agente IA:** Un LLM vía OpenRouter
- **Deploy:** Vercel (frontend), Railway (backend)

## URLs de Producción

| Servicio | URL |
|---|---|
| Frontend | `https://formas3d-crm.vercel.app` |
| Backend | `https://crm-test-production-6d2d.up.railway.app` |
| Webhook | `POST https://crm-test-production-6d2d.up.railway.app/webhook/zernio` |
| Health | `GET /health` |
| API | `GET /api/inbox/conversations` |
| SSE | `GET /api/events` |
| Upload | `POST /api/upload` |
| Media proxy | `GET /api/media?url=...` |

## Credenciales (NO commitear)

- **Supabase:** proyecto `mnlxthvltwvlujzfdtjo`, region `us-west-2`
- **Supabase Pooler:** `postgresql://postgres.mnlxthvltwvlujzfdtjo:PASSWORD@aws-0-us-west-2.pooler.supabase.com:6543/postgres?sslmode=require`
- **Zernio API key:** `sk_248723...` (read-write)
- **GitHub:** `riontdev/crm-test` (PAT configurada)
- **Railway:** env vars configuradas (DATABASE_URL, ZERNIO_API_KEY, SUPABASE_URL, SUPABASE_SERVICE_KEY)

## Modelo conceptual

### 1. CANAL y PROVEEDOR son dos ejes distintos
- `channel` = la red que ve la persona (whatsapp | instagram | facebook)
- `provider` = por dónde viaja el mensaje (zernio | meta | ...)
- Guardar en columnas separadas. Van a convivir combinaciones distintas.

### 2. La unidad NO es el contacto: es la CONVERSACIÓN
- La misma persona puede tener un hilo de WhatsApp y otro de Instagram, y no se mezclan.
- Las rutas y el historial del agente van por conversationId, nunca por contactId.

### 3. La identidad NO es el teléfono
- Instagram y Messenger no tienen número.
- Usar una tabla contact_identities con (channel, external_id) único y resolver el contacto por ahí.
- En WhatsApp el teléfono puede venir nulo: anclar en el id que da el proveedor.

## Reglas duras
Romper cualquiera de estas no produce un error: produce mensajes que se pierden en silencio.

1. **OpenAPI real:** ANTES de escribir el cliente HTTP, descargar el OpenAPI real de Zernio y tipar contra ese archivo. No adivinar nombres de campos.
2. **Webhook 5 segundos:** El webhook tiene 5 segundos para devolver 2xx. Persistir inline (INSERT) y hacer el trabajo pesado después con `after()`.
3. **Entrega at-least-once:** Reclama cada evento por su id en una tabla webhook_events con INSERT ON CONFLICT DO NOTHING RETURNING antes de procesarlo. Si no insertó, es un reintento: 200 y cortar.
4. **Idempotencia de mensajes:** El índice único va sobre external_id SOLO, no sobre (provider, external_id).
5. **Un solo camino de salida:** Una función deliverMessage(conversationId, ...) que envía Y persiste.
6. **Un agente por canal:** Prompt, herramientas y interruptor en una tabla agent_configs. Canales nuevos arrancan APAGADOS.
7. **Ventana 24h de Meta:** La decide el SERVIDOR. Guardar last_inbound_at en la conversación.
8. **Firma HMAC:** Verificar la firma HMAC del webhook sobre el body CRUDO. Sin secreto configurado, rechazar todo.
9. **Evento sin `id` se rechaza:** sin id no hay idempotencia posible. Reclamarlo con `id=""` hacía que el primero pasara y **todos los siguientes se perdieran en silencio** como duplicados. Devolver 400.

## Tablas de base de datos

### contacts
- id: uuid PK
- name: text nullable
- avatar_url: text nullable
- phone: text nullable (E.164)
- email: text nullable
- company: text nullable
- tags: text[] default '{}'
- notes: text nullable
- metadata: jsonb default '{}'
- created_at: timestamptz
- updated_at: timestamptz

### contact_identities
- id: uuid PK
- contact_id: uuid FK → contacts (ON DELETE CASCADE)
- channel: text (whatsapp | instagram | facebook)
- provider: text default 'zernio'
- external_id: text NOT NULL
- provider_username: text nullable
- provider_name: text nullable
- provider_avatar: text nullable
- created_at: timestamptz
- updated_at: timestamptz
- UNIQUE (channel, external_id) — NO composite con provider

### conversations
- id: uuid PK
- contact_id: uuid FK → contacts (ON DELETE CASCADE)
- channel: text
- provider: text default 'zernio'
- zernio_conversation_id: text NOT NULL
- zernio_account_id: text
- platform_conversation_id: text
- status: text default 'active' (active | archived)
- last_inbound_at: timestamptz
- unread_count: integer default 0
- created_at: timestamptz
- updated_at: timestamptz
- UNIQUE (channel, zernio_conversation_id)

### messages
- id: uuid PK
- conversation_id: uuid FK → conversations (ON DELETE CASCADE)
- external_id: text NOT NULL
- direction: text (incoming | outgoing)
- text: text nullable
- attachments: jsonb default '[]'
- sender_type: text (contact | agent | system)
- sender_contact_id: uuid nullable FK → contacts
- platform_message_id: text
- status: text default 'sent' (sent | delivered | read | failed)
- metadata: jsonb default '{}'
- sent_at: timestamptz
- created_at: timestamptz
- UNIQUE (external_id) — SOLO external_id, no composite

### webhook_events
- id: uuid PK
- event_id: text NOT NULL
- event_type: text NOT NULL
- payload: jsonb NOT NULL
- processed: boolean default false
- created_at: timestamptz
- UNIQUE (event_id)

### agent_configs
- id: uuid PK
- channel: text NOT NULL
- enabled: boolean default false
- model: text default 'openai/gpt-4o-mini'
- system_prompt: text nullable
- temperature: numeric default 0.7
- max_tokens: integer default 1024
- tools: jsonb default '[]'
- created_at: timestamptz
- updated_at: timestamptz
- UNIQUE (channel)

### Storage bucket
- Bucket: `attachments` (público)
- Usado para: imágenes, videos, archivos adjuntos subidos desde el frontend
- Path: `YYYY-MM/uuid.ext`

Seed: INSERT INTO agent_configs (channel, enabled) VALUES ('whatsapp', false), ('instagram', false), ('facebook', false);

## Flujo webhook (message.received)

```
webhook event
  → INSERT webhook_events (event_id) ON CONFLICT DO NOTHING RETURNING ...
  → si no insertó: 200 OK (es reintento)
  → buscar conversation por zernio_conversation_id
  → si no existe: crear contact → contact_identity → conversation
  → INSERT message
  → actualizar conversation.last_inbound_at = message.sent_at
  → sseHub.Broadcast() → tiempo real al frontend
  → after(): invocar agente si enabled
```

## API Endpoints

### Backend (Railway)

**Públicos:** `/health`, `POST /api/auth/login`, `POST /api/auth/logout`, `POST /webhook/zernio`.
**Todo el resto de `/api/*` exige sesión (cookie JWT httpOnly).** Los marcados 🔒 admin exigen rol admin.

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/health` | Health check (verifica DB) |
| POST | `/webhook/zernio` | Webhook entrante de Zernio |
| POST | `/api/auth/login` | Login (setea cookie sesión 7 días) |
| POST | `/api/auth/logout` | Logout (expira cookie) |
| GET | `/api/auth/me` | Usuario autenticado actual |
| GET | `/api/events` | SSE: actualizaciones en tiempo real |
| GET | `/api/inbox/conversations` | Listar conversaciones (?channel=&status=) |
| GET | `/api/inbox/conversations/:id` | Detalle + mensajes (resetea unread) |
| PATCH | `/api/inbox/conversations/:id` | Archivar/desarchivar {status} (404 si no existe) |
| POST | `/api/inbox/conversations/:id/messages` | Enviar mensaje (+ adjunto opcional) |
| PATCH | `/api/inbox/contacts/:id` | Guardar notas del contacto {notes} |
| POST | `/api/upload` | Subir archivo a Supabase Storage |
| GET | `/api/media?url=...` | Proxy de medios (bypass CORS/auth de Zernio) |
| GET | `/api/agents` | Configuración de agentes IA (JSON lowercase) |
| PATCH | `/api/agents/:channel` | Editar agente {enabled,model,system_prompt,temperature,max_tokens} |
| GET 🔒 | `/api/users` | Listar usuarios |
| POST 🔒 | `/api/users` | Crear usuario {email,name,password,role} |
| PUT 🔒 | `/api/users/:id` | Editar {name?,role?,password?} |
| DELETE 🔒 | `/api/users/:id` | Eliminar (protege último admin y auto-borrado) |
| GET | `/api/stats/overview?period=24h\|7d\|30d` | KPIs del dashboard (mensajes, Δ%, canales, IA vs humano, 1ª respuesta) |
| GET | `/api/stats/reports?from&to` | Series diarias por canal + tiempos de respuesta (máx 92 días) |
| GET | `/api/channels/status` | Estado real por canal + webhook URL |
| GET | `/api/templates?search=&category=` | Listar plantillas |
| POST | `/api/templates` | Crear plantilla {name,category,content,language?} |
| PUT | `/api/templates/:id` | Editar plantilla (parcial) |
| DELETE | `/api/templates/:id` | Eliminar plantilla |
| PATCH | `/api/auth/profile` | Perfil propio {name?,current_password+new_password?} |
| PATCH | `/api/inbox/conversations/:id` | también acepta {assigned_to?: uuid\|null} |
| GET | `/api/inbox/unread?limit=8` | Feed no leídos + total (campanita) |
| GET | `/api/inbox/search?q=` | Búsqueda global (nombre/teléfono/texto) |
| GET 🔒 | `/api/system/info` | Versión, DB, Zernio/OpenRouter configurados |

### Env vars (Railway)

| Variable | Descripción |
|---|---|
| `DATABASE_URL` | Supavisor pooler (transaction mode, puerto 6543) |
| `ZERNIO_API_KEY` | API key de Zernio |
| `ZERNIO_WEBHOOK_SECRET` | (opcional) HMAC secret |
| `SUPABASE_URL` | URL del proyecto Supabase |
| `SUPABASE_SERVICE_KEY` | Service role key de Supabase |
| `AUTH_JWT_SECRET` | Secreto HS256 para cookies de sesión (**requerido para login**) |
| `OPENROUTER_API_KEY` | **Pendiente** — sin ella los agentes no responden (log claro, sin crash) |
| `PORT` | Puerto del servidor (default: 8080) |

## Fases de desarrollo

1. **Fase 1** — Modelo de datos y migración ✅
2. **Fase 2** — Cliente de la API de Zernio, tipado contra el OpenAPI ✅
3. **Fase 3** — Webhook entrante y persistencia ✅
4. **Fase 4** — Bandeja: listar, abrir y responder ✅
5. **Fase 5** — Agente por canal ✅
6. **Fase 6** — Deploy y conexión de cuentas ✅
7. **Fase 7** — Tiempo real (SSE) ✅
8. **Fase 8** — Archivos adjuntos (upload + display + send) ✅
9. **Fase 9** — UI/UX redesign "SocialCRM" ✅ (design system Kinetic en docs/design-spec.md)
10. **Fase 10** — Agente IA: **construido y apagado** — falta solo cargar `OPENROUTER_API_KEY` y activar por canal
11. **Fase 11** — Conectar más cuentas (Instagram, Facebook) — PENDIENTE
12. **Fase 12** — Auth + Usuarios ✅ (login cookie JWT, CRUD usuarios admin-only, usuario default riontdev@gmail.com)
13. **Fase 13** — Dashboard KPIs ✅ (home post-login, /api/stats/overview, gráficos SVG sin dependencias)
14. **Fase 14** — Plantillas ✅ (migración 000009 templates + CRUD + picker en composer)
15. **Fase 15** — Canales ✅ (/api/channels/status estado real + webhook URL + guía IG/FB)
16. **Fase 16** — Reportes ✅ (/api/stats/reports + barras apiladas SVG + export CSV)
17. **Fase 17** — Configuración ✅ (perfil/password propio + /api/system/info admin)
18. **Fase 18** — Producto inbox ✅ (búsqueda global TopBar, notificaciones SSE, asignación de conversaciones, paginación/infinite scroll)
19. **Fase 19** — Análisis de pedidos con IA local ✅ (Ollama + Whisper en Docker, migraciones 12-14, API, SSE, catálogo 45 productos, UI)
20. **Fase 20** — UI del análisis ✅ (badge en inbox, OrderCard en el contacto, transcripción en burbujas, vistas Pedidos/Catálogo, config IA en Agentes)

---

## Análisis de pedidos (IA local)

Lee las conversaciones y arma el pedido. **No responde nunca al cliente**: el
agente de OpenRouter sigue siendo el único que contesta, y este módulo no toca
ese camino.

Todo corre en la máquina: Ollama y Whisper son servicios de Docker **sin
puertos publicados**. Ningún texto del cliente sale del servidor.

### Reglas duras de este módulo

9. **Read-only para la IA, writable para el humano.** `PATCH /api/orders/:id`
   sella `edited=true` y desde ese momento el consolidado NUNCA se vuelve a
   escribir. Gana el operador, siempre.
13. **Pedido sellado ≠ pedido resuelto.** Cuando el cliente pide algo más, el
    analysis nuevo se guarda pero no se consolida (el pedido está `edited`). Sin
    una salida visible, el operador mira un pedido viejo y cree que ya lo vio
    todo. Por eso existe `order_pending`: el pedido pendiente NO se pierde, se
    muestra. Resolverlo es una decisión del operador, por dos caminos:
    `accept-pending` (revisión nueva, la anterior queda en el historial) o
    `reopen` (la IA actualiza la misma revisión y se saca el sello).
14. **El pendiente se mide contra `applied_analysis_at`, no contra `updated_at`.**
    El status del embudo (`nuevo`→`entregado`) cambia `updated_at` y con una
    regla basada en timestamps el pendiente desaparecía solo. `applied_analysis_at`
    solo lo mueve el pipeline de IA, nunca un cambio de estado ni una corrección
    del operador.
10. **Sin confianza no hay pedido.** `needs_review` sale de reglas
    deterministas (`NeedsReview`), no de la confianza que se pogó el modelo.
11. **La IA no responde.** No hay función de envío en `internal/insight`. Si
    alguna vez se agrega, es otro módulo.
12. **Guardarraíl sobre el texto crudo.** `saneaCantidades` saca de
    `cantidades` los números que en el mensaje iban pegados a una unidad
    ("2 stickers de 5cm" → 2, medida 5 cm) y fuerza `needs_review`. Es
    determinista a propósito: con un modelo de 2B, pedir precisión por prompt
    no alcanza.
15. **Solo un `pedido` toca el pedido consolidado.** `Consolida()` en
    `persistOrder`: un reclamo o una pregunta pueden nombrar productos, y si
    se consolidaran sumarían líneas que el cliente no pidió. Los analysis
    no-`pedido` no se pierden: quedan en el historial y en la burbuja.
16. **Una pregunta no es un pedido.** `desambiguaPregunta` baja a `info` un
    analysis `pedido` cuyo texto es una pregunta sin verbo de pedido
    ("¿ustedes hacen una figura de kratos?"). Sin esto el hilo de Jorge Mujica
    arrancó con una línea fantasma de 1 figura y todos los deltas de después se
    sumaron sobre una base que nadie pidió. Con verbo explícito ("quiero 30
    stickers, ¿los tienen?") la `?` es decorativa y el pedido manda.
17. **Nunca un número que el cliente no dijo.** `alineado` completa con `1` las
    piezas sin cantidad para que los índices de productos y cantidades no se
    corran, y ese `1` se marca como relleno (`Order.CantidadesContadas`). Una
    línea sin cantidad contada **no entra al pedido**: no incrementa, no
    reemplaza y no crea línea. Es la diferencia entre "queda el pedido intacto +
    revisión" (correcto) y "Llavero 1" cuando el cliente pidió 12 (corrupto).
    El item queda en el historial del analysis, con "Traer al pedido".
18. **No hay variante por línea, y no se simula.** `productos` es un array de
    strings y `detalles` es del pedido entero, así que "figura de Kratos 15cm" y
    "figura de Mario 23cm" son la misma línea. Se probó etiquetar la línea
    (`"Miniatura / figura (Kratos 15cm)"`) y partir por variante: parte pedidos
    que son el mismo item ("suma 1 figura de kratos", que no repite la medida,
    abre una línea nueva) y tampoco agarra el caso que lo motivó, porque
    `granite3.3:2b` no pone "Mario" en ningún campo estructurado (deja
    `medidas: "23 cm, 15 cm"` y `personalizacion` vacía). Separar de verdad
    necesita un campo de variante por producto en el schema y en el prompt; con
    heurísticas sobre texto se reparte el pedido al azar.

### Pipeline

```
webhook / backfill
  → cola de TEXTO (buffer 128, 1 worker) o de AUDIO (buffer 24, 1 worker)
  → texto: contexto (10 mensajes) + catálogo → Ollama /api/chat → JSON
  → Enrich determinista: canonicaliza productos, busca material, sanea cantidades
  → message_analyses (status ok) + SSE insight.updated
  → consolidate en conversation_orders (MergeOrder sobre la fila is_current,
    skip si edited) + applied_analysis_at = now()
  → audio: descarga de Zernio → Whisper /asr → SetTranscript + SSE
     insight.transcript → reencola TEXTO con Job.ASRID (misma fila, sin doble Claim)
```

### Versiones de pedido (migración 000015)

Un hilo tiene una cadena de pedidos, no uno solo. La fila con `is_current=true`
es la que ve el operador; el índice único parcial `uniq_conversation_order_current`
impide que haya dos vigentes.

```
v1 (edited,archived) ──superseded_by──> v2 (is_current)
```

| Columna | Para qué |
|---|---|
| `revision` | 1, 2, 3… correlativo por hilo |
| `is_current` | Una sola fila `true` por hilo (índice parcial) |
| `superseded_at` / `superseded_by` | Cuándo y contra qué revisión se reemplazó |
| `applied_analysis_at` | Último analysis que el pipeline consolidó. **Movido solo por la IA** |

- `UpsertOrder` actualiza la fila `is_current` solo si `NOT edited`; si está
  sellada devuelve `false` y el analysis queda pendiente.
- `accept-pending` copia el analysis almacenado a una revisión nueva, archiva la
  anterior y enlaza `superseded_by`. La corrección del humano sobrevive porque
  vive en la fila que se archiva, no en el pendiente.
- `reopen` exige `edited=true`: una revisión que creó la IA no está "fijada" y no
  hay nada que reabrir (400). Aplica el pendiente sobre la misma fila, quita el
  sello y mueve `applied_analysis_at`.
- Toda lectura operativa (`/api/orders`, `/api/insights/conversation/:id`,
  `PATCH`, status) mira **solo** `is_current`. El historial se pide explícito
  vía `revisions`.
- `down` de la 000015 es **destructivo**: borra las revisiones no-current.

### Delta vs total (migración 000016)

"How many more" is not the same as "how many in total". If both are treated as
the total, the merge **replaces** 30 with 2, and nobody notices: the order
silently loses everything the customer already ordered.

`message_analyses.tipo_cantidad` (`delta` | `total` | NULL) records which one
the model understood. It is stored on the analysis, NOT on the order, because a
pending analysis gets reapplied days later and has to mean the same thing it
meant when it was written.

| `tipo_cantidad` | Producto ya en el pedido | Producto nuevo |
|---|---|---|
| `delta` | **suma** la cantidad | agrega la línea con esa cantidad |
| `total` | **reemplaza** la cantidad | agrega la línea con esa cantidad |
| `NULL` + cantidad | conserva la anterior + `needs_review` | agrega la línea |
| `NULL` sin cantidad | conserva | conserva |

- `MergeOrder(prev, next, tipo)` is deterministic: the model only *classifies*,
  the merge decides. A wrong classification is visible (`needs_review`), never
  silent.
- One type per analysis, not per product. A message that both adds and resets in
  a single sentence is not representable; the model picks one and the operator
  sees the doubt.
- The model **frequently returns a computed quantity** for a delta ("y sumale 2 más"
  → 32, already added). `saneaTipoCantidad` vetoes any delta whose quantity
  does not appear literally in that message's text, sets the type to NULL and
  drops confidence below 0.5, so the merge preserves the order. It cannot catch
  the inverse: a *total* misread as delta whose number does appear in the text.
  That one needs the operator.
- `AcceptPending` and `ReopenOrder` walk **all** pending analyses in
  `created_at` order, not just the latest. Two pending deltas are +5 then +2;
  applying only the last one loses the first.

### Traer productos al pedido (frontend)

`OrderCard.vue` has "Traer al pedido" on the pending banner, on every archived
revision and on every `pedido` analysis in the history. It **sums** into the
draft if the product is already there and appends if it is not — the same rule
as `delta`, so the button and the AI cannot disagree. It only fills the form:
saving is the human's decision and seals the order (`edited`).

### Env vars (Docker local)

| Variable | Default | Descripción |
|---|---|---|
| `INSIGHT_ENABLED` | `true` | Si es `false` no se registran las rutas `/api/insights*` |
| `OLLAMA_BASE_URL` | `http://ollama:11434` | API local de Ollama |
| `INSIGHT_MODEL` | `granite3.3:2b` | Modelo de análisis |
| `INSIGHT_TEXT_WORKERS` | `1` | Workers de texto |
| `ASR_BASE_URL` | `http://whisper:9000` | Whisper asr webservice |
| `ASR_MODEL` | `small` | Modelo de transcripción |
| `ASR_MAX_SECONDS` | `300` | Corta audios más largos |
| `ASR_MAX_BYTES` | `20MB` | Tope de descarga |

### Endpoints del módulo

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/api/insights/status` | Estado global, modelo, colas, contadores, errores recientes |
| GET | `/api/insights/messages/:id` | Análisis de un mensaje |
| GET | `/api/insights/conversation/:id` | Pedido consolidado + analyses + `revisions` (si hay >1) + `pending` |
| PATCH | `/api/insights/settings/:key` | `insight.master_enabled` \| `insight.asr_enabled` — cuerpo `{"enabled":bool}` |
| PATCH | `/api/insights/config/:channel` | Config por canal |
| POST | `/api/insights/backfill` | Encola mensajes sin análisis (`{conversation_id?,limit?,force?}`) |
| GET/POST | `/api/catalog` | Listar (`?all=true` ve los inactivos) / crear |
| PATCH/DELETE | `/api/catalog/:id` | Editar / borrar |
| GET | `/api/orders` | Lista paginada (`status,intent,search,limit,offset`), solo vigentes |
| GET/PATCH | `/api/orders/:id` | Detalle / corrección manual (sella `edited`) |
| PATCH | `/api/orders/:id/status` | `nuevo\|confirmado\|entregado\|descartado` |
| POST | `/api/orders/:id/accept-pending` | Aplica el pendiente como revisión nueva (409 si no hay) |
| POST | `/api/orders/:id/reopen` | Saca el sello y aplica el pendiente en la misma revisión (400 si no está editada) |
| PATCH | `/api/inbox/conversations/:id` | acepta `insight_enabled` (interruptor por hilo) |

### SSE

| Evento | Payload | Cuándo |
|---|---|---|
| `insight.updated` | `{conversation_id,message_id,channel,order}` o `{...,analysis,asr_text}` | La IA terminó |
| `insight.transcript` | `{conversation_id,message_id,channel,text,asr_ms,asr_model}` | Whisper terminó |
| `insight.health` | `{ready}` | Ollama cargó el modelo |

`insight.transcript` se pinta **sin refetch**: el texto viene en el evento. El
backend guarda la transcripción ANTES de analizar, así que si Ollama se cae el
operador igual puede leer lo que dijo el cliente.

### Frontend

- `stores/insights.ts`: estado + SSE. Cachea por conversación, indexa analyses
  por `message_id` y refresca solo lo que está en pantalla.
- `OrderBadge.vue`: badge de intención en la lista del inbox. `reclamo` en rojo
  a propósito: es el único estado que hay que mirar ya. `pending` (punto ámbar +
  "· nuevo pedido") le gana a `needsReview`: uno es una decisión del operador
  que se pierde si no se ve, el otro es una duda de la IA.
- `OrderCard.vue`: lectura + edición del pedido en el panel de contacto. Con
  `pending` muestra el pedido nuevo arriba, en ámbar, con **Aplicar** (revisión
  nueva) y **Reabrir** (misma revisión). El formulario solo edita `material`,
  así que al guardar hace merge sobre los `detalles` existentes: mandar solo
  `{material}` borraba medidas, personalización, fecha de entrega y envío.
- `MessageBubble.vue`: transcripción y resumen debajo del audio.
- `OrdersView.vue` (`/orders`) y `CatalogView.vue` (`/catalog`).

> Las vistas esperan `await fetchStatus()` antes de mirar `insights.enabled`.
> Con `void fetchStatus()` el `enabled` da `false` (status es `null`) y la vista
> queda en "cargando" para siempre al entrar directo o tras recargar.

## Pendiente

- Fase 11: conectar cuentas Instagram/Facebook en Zernio
- Grabar mensajes de audio (botón mic + MediaRecorder) — postergado
- Métricas del diseño sin datos reales hoy: CSAT y Resolution Rate (requerirían encuestas/tickets)
- **Modelo de 2B**: el bench da ~80% de acierto en intención y productos. Con
  `granite3.3:2b` el caso "2 stickers de 5cm" salía con cantidad 5 hasta que
  se agregó `saneaCantidades`. Subir a un modelo de 7-8B en la misma máquina es
  la vía obvia si el modelo chico molesta.

### templates
- id: uuid PK
- name: text NOT NULL (2..80)
- category: text default 'general' (marketing | utility | soporte | general)
- content: text NOT NULL (variables {{1}}, {{2}}...)
- language: text default 'es'
- created_at / updated_at: timestamptz

Seed: Bienvenida, Horarios, Seguimiento, Promo.

### users
- id: uuid PK
- email: text NOT NULL UNIQUE
- name: text NOT NULL
- password_hash: text NOT NULL (bcrypt)
- role: text default 'agente' (admin | agente)
- created_at / updated_at: timestamptz

Seed default: riontdev@gmail.com / 123456 (rol admin, hash via pgcrypto).

## Cómo trabajar
Por fases. Al terminar cada una, `go build ./...` y `vue-tsc --noEmit` + `npm run build` deben pasar limpios antes de seguir. No avanzar si la fase anterior no compila.

## Stack técnico

### Backend (Go)
- Echo v4 (framework HTTP)
- pgx/v5 (driver Postgres)
- pgx/v5/pgxpool (connection pool)
- golang-migrate/migrate/v4 (migraciones SQL)
- go:embed para archivos de migración
- SSE (Server-Sent Events) para tiempo real

### Frontend (Vue + TypeScript)
- Vue 3 (Composition API, `<script setup>`)
- Pinia (state manager)
- TypeScript
- Tailwind CSS 4 (con @theme)
- Vite (bundler)
- SSE via EventSource API

### Database
- Supabase Postgres
- Conexión via Supavisor transaction mode (puerto 6543, `DefaultQueryExecModeSimpleProtocol`)
- sslmode=require
- Migraciones SQL manuales (no ORM)
- Supabase Storage para archivos adjuntos

### Operación — troubleshooting pooler
- Si el backend local se cuelga tras "Migrations applied" o Railway no termina un deploy:
  probar `echo > /dev/tcp/aws-0-us-west-2.pooler.supabase.com/6543` (transaction mode).
  El 5432 (session mode) suele seguir disponible como fallback de diagnóstico.
- Si Railway quedó con deploy fallido por una caída del pooler, el contenedor viejo sigue vivo:
  disparar redeploy con un nuevo push a `main`.

### Deploy
- Vercel (frontend SPA, rewrite rules para SPA routing + proxy API)
- Railway (backend Go, Dockerfile multi-stage)
- Auto-deploy desde GitHub push a `main`

## Deploy local (Docker + Cloudflare Tunnel) — PRODUCCIÓN ACTUAL

El stack local `docker-compose.yml` es el despliegue activo (los free tiers de
Vercel/Railway se agotaron). Postgres y MinIO corren en Docker; el acceso público
va por Cloudflare Tunnel (patrón igual al de Dozzle).

| Servicio | URL |
|---|---|
| Webhook | `POST https://<tunnel>.trycloudflare.com/webhook/zernio` |
| Health | `GET /health` |
| API | `GET /api/inbox/conversations` |
| SSE | `GET /api/events` |
| Upload | `POST /api/upload` |
| Files | `GET /api/files/:bucket/:key` (MinIO) |

### Comandos

```bash
docker compose up -d --build          # levantar todo
docker compose up -d postgres minio   # solo infra (sin app)
docker compose logs -f tunnel         # ver la URL .trycloudflare.com actual
docker compose ps                     # estado
docker compose down                   # bajar (mantiene volumes pgdata/miniodata)
```

### Servicios

- `postgres` (postgres:16-alpine, sin puerto host — el 5432 host está ocupado),
  init corre `CREATE EXTENSION pgcrypto`. Init de datos: `docker/postgres/init/`.
- `minio` + `minio-init` (bucket `attachments`), sin puerto host (9000/9001 host ocupados).
- `backend` (Go, build `backend/Dockerfile`, `env_file .env`).
- `frontend` (nginx multi-stage: `vue-tsc` + `vite build` → nginx:alpine). Proxea
  `/api`, `/webhook`, `/health` → `backend:8080`. SSE con buffering desactivado.
- `tunnel` (cloudflared quick tunnel → `http://frontend:80`).

> **Quick tunnel:** la URL `*.trycloudflare.com` CAMBIA en cada reinicio del
> contenedor `tunnel`. El webhook de Zernio hay que reconfigurarlo con la URL
> nueva cada vez. Pendiente: migrar a **named tunnel** con subdominio fijo
> (`cloudflared tunnel login` → `create crm` → `route dns crm crm.<dominio>` →
> montar `credentials-file` + `config.yml` con ingress).

### Env vars (Docker local, en `.env`)

`DATABASE_URL` apunta a `postgres:5432/crm` (local). Se mantienen
`ZERNIO_API_KEY`, `ZERNIO_WEBHOOK_SECRET`, `OPENROUTER_API_KEY`,
`AUTH_JWT_SECRET`, `PORT`. Nuevas:

| Variable | Descripción |
|---|---|
| `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` | Postgres local (`crm`/`crm`/`crm`) |
| `MINIO_ENDPOINT` | `minio:9000` (dns interno) |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | Credenciales MinIO |
| `MINIO_BUCKET` | `attachments` |
| `MINIO_USE_SSL` | `false` (red interna docker) |
| `SUPABASE_LEGACY_DB_URL` | Solo para dump/restore inicial, borrar después |
| `INSIGHT_ENABLED` | `true` — apaga todo el módulo de análisis si es `false` |
| `OLLAMA_BASE_URL` | `http://ollama:11434` (nombre de servicio, sin puerto host) |
| `INSIGHT_MODEL` | `granite3.3:2b` |
| `ASR_BASE_URL` | `http://whisper:9000` |
| `ASR_MODEL` | `small` |
| `LOG_FORMAT` | `json` (o `text` para desarrollo) |

### Storage de adjuntos

`backend/internal/handlers/upload.go` escribe a MinIO y devuelve URL relativa
`/api/files/<bucket>/<key>`; `GET /api/files/*` (protegido) lo sirve. Los adjuntos
viejos con URL pública de Supabase siguen resolviendo igual (bucket público).

### Migración de datos desde Supabase (una vez)

La migración `000007` (storage de Supabase) es tolerante: en Postgres plano es un
no-op, y migrations corren solas al arrancar el backend. Para importar datos
legacy: `pg_dump --schema=public --no-owner --no-privileges` desde la URL session
mode (puerto 5432, el 6543 no soporta pg_dump) y restaurar con `psql` antes del
primer arranque del backend (así `schema_migrations` queda completo).
