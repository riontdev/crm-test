# CRM Multicanal — AGENTS.md

## Qué es
Una bandeja única donde entran y se responden WhatsApp, Instagram DM y Facebook Messenger, con un agente de IA configurable por canal que contesta solo.

## Stack
- **Backend:** Golang (framework Echo) — SIN TypeScript en el backend
- **Frontend:** Vue.js with Pinia state manager + TypeScript + Tailwind 4 + shadcn/ui
- **Database:** Supabase (Postgres + Auth), migraciones SQL manuales con golang-migrate
- **Proveedor de canales:** Zernio — docs.zernio.com
- **Agente IA:** Un LLM vía OpenRouter
- **Deploy:** Vercel (frontend), backend por definir

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
  → after(): invocar agente si enabled
```

## Fases de desarrollo

1. **Fase 1** — Modelo de datos y migración ← ACTUAL
2. **Fase 2** — Cliente de la API de Zernio, tipado contra el OpenAPI
3. **Fase 3** — Webhook entrante y persistencia
4. **Fase 4** — Bandeja: listar, abrir y responder
5. **Fase 5** — Agente por canal
6. **Fase 6** — Deploy, conexión de cuentas y barridos

## Cómo trabajar
Por fases. Al terminar cada una, go build y go vet deben pasar limpios antes de seguir. No avanzar si la fase anterior no compila.

## Stack técnico

### Backend (Go)
- Echo v4 (framework HTTP)
- pgx/v5 (driver Postgres)
- pgx/v5/pgxpool (connection pool)
- golang-migrate/migrate/v4 (migraciones SQL)
- go:embed para archivos de migración

### Frontend (Vue + TypeScript)
- Vue 3
- Pinia (state manager)
- TypeScript
- Tailwind CSS 4
- shadcn/ui
- Vite (bundler)

### Database
- Supabase Postgres
- Conexión via Supavisor transaction mode (puerto 6543)
- sslmode=require
- Migraciones SQL manuales (no ORM)
