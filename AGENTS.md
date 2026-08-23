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
| Frontend | `https://crm-test-xi-gilt.vercel.app` |
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

## Pendiente

- Cargar `OPENROUTER_API_KEY` en Railway → activar agentes desde UI (AgentsView editable ya operativa)
- Fase 11: conectar cuentas Instagram/Facebook en Zernio
- Grabar mensajes de audio (botón mic + MediaRecorder) — postergado
- Páginas del diseño Stitch no implementadas: Dashboard KPIs, Canales, Plantillas, Reportes, Configuración

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

### Deploy
- Vercel (frontend SPA, rewrite rules para SPA routing + proxy API)
- Railway (backend Go, Dockerfile multi-stage)
- Auto-deploy desde GitHub push a `main`
