-- ---------------------------------------------------------------------------
-- 000013: productos y cantidades pasan de jsonb a arrays nativos
-- ---------------------------------------------------------------------------
-- Por que: la 000012 los creo como jsonb y el codigo Go los trata como
-- text[] / int[]. Con jsonb, pgx manda un []string de Go como text[] (que
-- Postgres no puede castear a jsonb) y el UPDATE muere con
-- "invalid input syntax for type json" al PRIMER mensaje analizado.
--
-- Arrays nativos ademas son lo correcto para el dominio: productos es una
-- lista de nombres para comparar contra el catalogo, y con GIN se puede
-- indexar y filtrar en SQL. jsonb obliga a parsear en cada fila.
--
-- Nota: productos[i] y cantidades[i] son el mismo producto, en ese orden.
--
-- DETALLES DE IMPLEMENTACION (los dos_costaron una corrida):
--
-- 1. Un USING de ALTER TABLE ... TYPE no admite subconsultas ("cannot use
--    subquery in transform expression"). Por eso la conversion va por una
--    funcion IMMUTABLE, que si es una expresion valida.
-- 2. El DEFAULT tiene que caerse ANTES de cambiar el tipo: si no, Postgres
--    intenta castear '[]'::jsonb a text[] en el mismo ALTER y falla con
--    "default for column productos cannot be cast automatically". Por eso
--    DROP DEFAULT, TYPE, SET DEFAULT van en sentencias separadas y en ese
--    orden.
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION jsonb_to_text_array(j jsonb) RETURNS text[]
    LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT COALESCE(ARRAY(SELECT jsonb_array_elements_text(j)), '{}'::text[])
$$;

CREATE OR REPLACE FUNCTION jsonb_to_int_array(j jsonb) RETURNS integer[]
    LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT COALESCE(ARRAY(SELECT jsonb_array_elements(j)::int), '{}'::integer[])
$$;

ALTER TABLE message_analyses ALTER COLUMN productos DROP DEFAULT;
ALTER TABLE message_analyses
    ALTER COLUMN productos TYPE text[] USING jsonb_to_text_array(productos);
ALTER TABLE message_analyses ALTER COLUMN productos SET DEFAULT '{}'::text[];

ALTER TABLE message_analyses ALTER COLUMN cantidades DROP DEFAULT;
ALTER TABLE message_analyses
    ALTER COLUMN cantidades TYPE integer[] USING jsonb_to_int_array(cantidades);
ALTER TABLE message_analyses ALTER COLUMN cantidades SET DEFAULT '{}'::integer[];

ALTER TABLE conversation_orders ALTER COLUMN productos DROP DEFAULT;
ALTER TABLE conversation_orders
    ALTER COLUMN productos TYPE text[] USING jsonb_to_text_array(productos);
ALTER TABLE conversation_orders ALTER COLUMN productos SET DEFAULT '{}'::text[];

ALTER TABLE conversation_orders ALTER COLUMN cantidades DROP DEFAULT;
ALTER TABLE conversation_orders
    ALTER COLUMN cantidades TYPE integer[] USING jsonb_to_int_array(cantidades);
ALTER TABLE conversation_orders ALTER COLUMN cantidades SET DEFAULT '{}'::integer[];

-- Indice GIN para buscar "de que conversationes hablan de este producto".
-- Es lo que va a usar la vista de Pedidos al filtrar por producto.
CREATE INDEX IF NOT EXISTS idx_conv_orders_productos
    ON conversation_orders USING gin (productos);

DROP FUNCTION jsonb_to_text_array(jsonb);
DROP FUNCTION jsonb_to_int_array(jsonb);
