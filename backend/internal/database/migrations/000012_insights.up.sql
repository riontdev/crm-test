-- Fase 19+20: analisis de pedidos con IA local (Ollama) y transcripcion de
-- notas de voz (Whisper), ambos en segundo plano.
--
-- Tres ideas gobiernan este esquema:
--   1. Una fila por MENSAJE (message_analyses) para trazabilidad: siempre se
--      puede ver por que el modelo dijo lo que dijo (columna raw).
--   2. Una fila por CONVERSACION (conversation_orders) que es lo que ve el
--      operador: el pedido se acumula a lo largo del hilo.
--   3. edited=true significa "dato humano": la IA no lo pisa nunca mas.

-- ---------------------------------------------------------------------------
-- Configuracion por canal del analizador
-- ---------------------------------------------------------------------------
-- Nace enabled=true, excepcion JUSTIFICADA a la regla dura 6 del AGENTS.md.
-- Esa regla protege al cliente de RESPUESTAS automaticas, que siguen apagadas
-- en agent_configs. El analizador es read-only: el paquete internal/insight no
-- importa internal/zernio, no puede enviar nada a nadie.
CREATE TABLE insight_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    asr_enabled boolean NOT NULL DEFAULT true,
    model text NOT NULL DEFAULT '',
    system_prompt text,
    temperature numeric NOT NULL DEFAULT 0,
    context_messages integer NOT NULL DEFAULT 10,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (channel)
);

INSERT INTO insight_configs (channel, enabled, asr_enabled) VALUES
('whatsapp', true, true),
('instagram', true, true),
('facebook', true, true);

-- ---------------------------------------------------------------------------
-- Ajustes globales (interruptores maestros)
-- ---------------------------------------------------------------------------
CREATE TABLE app_settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL DEFAULT 'true'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL
);

INSERT INTO app_settings (key, value) VALUES
('insight.master_enabled', 'true'::jsonb),
('insight.asr_enabled', 'true'::jsonb);

-- ---------------------------------------------------------------------------
-- Catalogo de productos: se inyecta en el system prompt en cada llamada
-- ---------------------------------------------------------------------------
CREATE TABLE product_catalog (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    category text NOT NULL DEFAULT 'general',
    aliases text[] NOT NULL DEFAULT '{}',
    active boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_product_catalog_active ON product_catalog (active, category, sort_order);

-- Semilla: impresion 3D. Los alias existen para que el modelo normalice
-- "filamento", "pla" o "3d" al nombre canonico del catalogo.
INSERT INTO product_catalog (name, category, aliases, sort_order) VALUES
('Impresión 3D FDM (filamento)', 'servicio', ARRAY['impresion','impresion 3d','filamento','fdm','imprimir en 3d'], 10),
('Impresión 3D SLA en resina', 'servicio', ARRAY['sla','resina','impresion en resina','impresion sla'], 11),
('Modelado 3D / diseño CAD', 'servicio', ARRAY['diseno','modelado','cad','archivo stl','archivo 3mf','diseño 3d'], 12),
('Escaneo 3D', 'servicio', ARRAY['escaneo','digitalizacion','metrologia','escaneo 3d'], 13),
('Prototipo rápido', 'servicio', ARRAY['prototipo','prototipo rapido','prueba de impresion'], 14),

('PLA', 'material', ARRAY['filamento pla','plastico','pla mate'], 20),
('PLA+', 'material', ARRAY['pla plus','pla reforzado'], 21),
('PETG', 'material', ARRAY['petg','filamento petg'], 22),
('ABS', 'material', ARRAY['abs','filamento abs'], 23),
('ASA', 'material', ARRAY['asa','filamento asa'], 24),
('TPU (flexible)', 'material', ARRAY['tpu','flexible','elastico','goma'], 25),
('Nylon (PA)', 'material', ARRAY['nylon','pa6','pa12','nylon pa'], 26),
('Policarbonato (PC)', 'material', ARRAY['policarbonato','pc'], 27),
('Fibra de carbono', 'material', ARRAY['cf','carbono','carbon'], 28),
('Fibra de vidrio', 'material', ARRAY['gf','vidio','fibra de vidrio'], 29),

('Resina estándar', 'resina', ARRAY['resina estandar','resina normal'], 30),
('Resina flexible', 'resina', ARRAY['resina flexible','goma de resina'], 31),
('Resina resistente (alta temperatura)', 'resina', ARRAY['resina ht','resina alta temperatura'], 32),
('Resina transparente', 'resina', ARRAY['resina transparente','resina crystal','clear'], 33),
('Resina de detalle fino', 'resina', ARRAY['resina detalle','resina miniaturas','alta definicion'], 34),

('Post-proceso (lijado/alisado)', 'acabado', ARRAY['post proceso','lijado','alisado','acabado'], 40),
('Pintura / pintado', 'acabado', ARRAY['pintura','pintado','pintura en spray','pintar'], 41),
('Impresión multicolor', 'acabado', ARRAY['multicolor','varios colores','doble color'], 42),
('Barniz epoxi', 'acabado', ARRAY['barniz','epoxi','barnizado','barnizar'], 43),
('Alisado con vapor', 'acabado', ARRAY['vapor','suavizado','smooth'], 44),

('Organizador', 'pieza', ARRAY['organizador','compartimento','cajonera'], 50),
('Maceta', 'pieza', ARRAY['maceta','jardinera','planta'], 51),
('Llavero', 'pieza', ARRAY['llavero','llaveros','porta llaves'], 52),
('Portaobjetos', 'pieza', ARRAY['portaobjetos','soporte celular','loader'], 53),
('Separador', 'pieza', ARRAY['separador','separadores','organizador separador'], 54),
('Soporte', 'pieza', ARRAY['soporte','holder','stand','base'], 55),
('Engranaje', 'pieza', ARRAY['engranaje','engranajes','gear'], 56),
('Tapa', 'pieza', ARRAY['tapa','tapon'], 57),
('Imanes', 'pieza', ARRAY['iman','imanes','magnetico','magnetico'], 58),
('Pieza a medida', 'pieza', ARRAY['a medida','medida','custom','personalizado'], 59),

('Miniatura / figura', 'uso', ARRAY['miniatura','miniaturas','figura','figuras','personaje'], 60),
('Joyería / bisutería', 'uso', ARRAY['joyeria','anillo','pendientes','pulsera','bijouterie'], 61),
('Maqueta / maquetería', 'uso', ARRAY['maqueta','maquetas','diorama','miniatura de arquitectura'], 62),
('Carcasa electrónica', 'uso', ARRAY['carcasa','enclosure','caja electronica','carcasa de circuito'], 63),
('Repuesto industrial', 'uso', ARRAY['repuesto','repuestos','pieza industrial','repuesto de fabrica'], 64),
('Prototipo funcional', 'uso', ARRAY['funcional','prueba fisica','que funcione'], 65),
('Prototipo de producto', 'uso', ARRAY['producto','producto nuevo','lanzamiento'], 66),

('Envío', 'complemento', ARRAY['envio','entrega','delivery','correo'], 70),
('Entrega express', 'complemento', ARRAY['express','entrega rapida','urgente','same day'], 71),
('Cotización', 'complemento', ARRAY['cotizacion','cotizar','precio','presupuesto','valor'], 72);

-- ---------------------------------------------------------------------------
-- Analisis por MENSAJE (trazabilidad)
-- ---------------------------------------------------------------------------
CREATE TABLE message_analyses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    scope text NOT NULL DEFAULT 'message' CHECK (scope IN ('message','conversation')),
    intent text CHECK (intent IN ('pedido','info','reclamo','otro')),
    resumen text,
    productos jsonb NOT NULL DEFAULT '[]',
    cantidades jsonb NOT NULL DEFAULT '[]',
    detalles jsonb NOT NULL DEFAULT '{}',
    confianza numeric,
    needs_review boolean NOT NULL DEFAULT false,
    asr_text text,
    asr_ms integer,
    asr_model text,
    model text,
    latency_ms integer,
    raw jsonb NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','processing','ok','error','skipped')),
    error text,
    skip_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (message_id)
);

CREATE INDEX idx_message_analyses_conv ON message_analyses (conversation_id, created_at DESC);
CREATE INDEX idx_message_analyses_status ON message_analyses (status);

-- ---------------------------------------------------------------------------
-- Pedido CONSOLIDADO por conversacion (lo que ve el operador)
-- ---------------------------------------------------------------------------
CREATE TABLE conversation_orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    intent text CHECK (intent IN ('pedido','info','reclamo','otro')),
    resumen text,
    productos jsonb NOT NULL DEFAULT '[]',
    cantidades jsonb NOT NULL DEFAULT '[]',
    detalles jsonb NOT NULL DEFAULT '{}',
    confianza numeric,
    needs_review boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'nuevo'
        CHECK (status IN ('nuevo','confirmado','entregado','descartado')),
    source_message_id uuid REFERENCES messages(id) ON DELETE SET NULL,
    model text,
    edited boolean NOT NULL DEFAULT false,
    edited_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (conversation_id)
);

CREATE INDEX idx_conversation_orders_status ON conversation_orders (status, updated_at DESC);
CREATE INDEX idx_conversation_orders_intent ON conversation_orders (intent);

-- ---------------------------------------------------------------------------
-- Interruptor por conversacion. DEFAULT true = analisis prendido.
-- Efectivo = app_settings('insight.master_enabled')
--         AND insight_configs[channel].enabled
--         AND conversations.insight_enabled
-- ---------------------------------------------------------------------------
ALTER TABLE conversations ADD COLUMN insight_enabled boolean NOT NULL DEFAULT true;
