-- Vuelve a jsonb. Solo para rollback; la 000012 original sigue siendo la
-- fuente de verdad del tipo jsonb.
ALTER TABLE conversation_orders DROP INDEX IF EXISTS idx_conv_orders_productos;

-- El mismo truco que en la 000013: en USING no puede haber subconsultas.
CREATE OR REPLACE FUNCTION text_array_to_jsonb(a text[]) RETURNS jsonb
    LANGUAGE sql IMMUTABLE STRICT AS $$ SELECT to_jsonb(a) $$;

CREATE OR REPLACE FUNCTION int_array_to_jsonb(a integer[]) RETURNS jsonb
    LANGUAGE sql IMMUTABLE STRICT AS $$ SELECT to_jsonb(a) $$;

ALTER TABLE conversation_orders ALTER COLUMN productos DROP DEFAULT;
ALTER TABLE conversation_orders ALTER COLUMN productos TYPE jsonb USING text_array_to_jsonb(productos);
ALTER TABLE conversation_orders ALTER COLUMN productos SET DEFAULT '[]'::jsonb;

ALTER TABLE conversation_orders ALTER COLUMN cantidades DROP DEFAULT;
ALTER TABLE conversation_orders ALTER COLUMN cantidades TYPE jsonb USING int_array_to_jsonb(cantidades);
ALTER TABLE conversation_orders ALTER COLUMN cantidades SET DEFAULT '[]'::jsonb;

ALTER TABLE message_analyses ALTER COLUMN productos DROP DEFAULT;
ALTER TABLE message_analyses ALTER COLUMN productos TYPE jsonb USING text_array_to_jsonb(productos);
ALTER TABLE message_analyses ALTER COLUMN productos SET DEFAULT '[]'::jsonb;

ALTER TABLE message_analyses ALTER COLUMN cantidades DROP DEFAULT;
ALTER TABLE message_analyses ALTER COLUMN cantidades TYPE jsonb USING int_array_to_jsonb(cantidades);
ALTER TABLE message_analyses ALTER COLUMN cantidades SET DEFAULT '[]'::jsonb;

DROP FUNCTION text_array_to_jsonb(text[]);
DROP FUNCTION int_array_to_jsonb(integer[]);
