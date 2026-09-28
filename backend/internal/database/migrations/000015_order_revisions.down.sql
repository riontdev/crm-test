-- ---------------------------------------------------------------------------
-- 000015 down: deshace el historial de revisiones
-- ---------------------------------------------------------------------------
-- ADVERTENCIA: es destructivo y a proposito. Si un hilo llego a tener mas de
-- una revision, volver al UNIQUE (conversation_id) es imposible sin borrar
-- algo, asi que se conservan solo las vigentes. Lo que se pierde es el
-- historial de pedidos reemplazados: eso no se puede recuperar.
-- ---------------------------------------------------------------------------

DELETE FROM conversation_orders WHERE NOT is_current;

DROP INDEX IF EXISTS idx_conv_orders_revisions;
DROP INDEX IF EXISTS uniq_conversation_order_current;

ALTER TABLE conversation_orders DROP COLUMN IF EXISTS applied_analysis_at;
ALTER TABLE conversation_orders DROP COLUMN IF EXISTS superseded_by;
ALTER TABLE conversation_orders DROP COLUMN IF EXISTS superseded_at;
ALTER TABLE conversation_orders DROP COLUMN IF EXISTS is_current;
ALTER TABLE conversation_orders DROP COLUMN IF EXISTS revision;

ALTER TABLE conversation_orders ADD CONSTRAINT conversation_orders_conversation_id_key
    UNIQUE (conversation_id);
