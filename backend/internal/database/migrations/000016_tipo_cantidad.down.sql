-- ---------------------------------------------------------------------------
-- 000016 down: saca tipo_cantidad
-- ---------------------------------------------------------------------------
-- Perder la columna es seguro: los analysis vuelven al estado previo, donde
-- MergeOrder asumia que toda cantidad era un total. No se borra ningun analysis
-- ni ningun pedido.
-- ---------------------------------------------------------------------------

ALTER TABLE message_analyses DROP CONSTRAINT IF EXISTS message_analyses_tipo_cantidad_check;
ALTER TABLE message_analyses DROP COLUMN IF EXISTS tipo_cantidad;
