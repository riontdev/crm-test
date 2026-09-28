-- ---------------------------------------------------------------------------
-- 000015: historial de revisiones del pedido consolidado
-- ---------------------------------------------------------------------------
-- Por que: conversation_orders tenia UNIQUE (conversation_id), o sea UNA sola
-- fila por hilo. La regla dura #9 (si un humano edito, la IA no vuelve a
-- escribir) estaba bien, pero dejaba un agujero: el cliente mandaba un pedido
-- NUEVO y la IA lo consolidaba, UpsertOrder rechazaba la escritura porque la
-- fila estaba editada, y el operador se quedaba viendo un pedido viejo con
-- needs_review=false y ninguna senal de que algo estaba pendiente. El sistema
-- era silencioso justo donde deberia gritar.
--
-- Como se resuelve: el pedido deja de ser una fila y pasa a ser una cadena de
-- revisiones. La vigente tiene is_current=true y hay un indice unico parcial
-- que garantiza como maximo una vigente por hilo (el mismo invariante que
-- daba el UNIQUE, pero sin impedir el historial).
--
-- La IA NO crea revisiones: escribe sobre la vigente mientras no este editada,
-- igual que antes. Las revisiones las crea el humano, y solo de dos formas
-- explicitas:
--   - aplicar: el pedido nuevo del cliente pasa a ser la nueva vigente y la
--     anterior queda en el historial.
--   - reabrir: el pedido editado por el humano vuelve a ser escribible por la
--     IA sobre la MISMA fila, sin perder la correccion.
--
-- superseded_by apunta a la revision que la reemplazo, para que el timeline de
-- la UI no tenga que deducirlo comparando timestamps.
-- ---------------------------------------------------------------------------

ALTER TABLE conversation_orders DROP CONSTRAINT IF EXISTS conversation_orders_conversation_id_key;

ALTER TABLE conversation_orders ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1;

ALTER TABLE conversation_orders ADD COLUMN IF NOT EXISTS is_current boolean NOT NULL DEFAULT true;

-- Solo una vigente por hilo. Es el invariante que daba el UNIQUE viejo.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_conversation_order_current
    ON conversation_orders (conversation_id) WHERE is_current;

-- Timeline: historial de un hilo, de la revision mas nueva a la mas vieja.
CREATE INDEX IF NOT EXISTS idx_conv_orders_revisions
    ON conversation_orders (conversation_id, revision DESC);

-- Cuando dejo de ser la vigente, y cual la reemplazo. NULL en la vigente.
ALTER TABLE conversation_orders ADD COLUMN IF NOT EXISTS superseded_at timestamptz;

-- applied_analysis_at es "hasta aca llego el pedido": el created_at del ultimo
-- analysis que ya esta consolidado en ESTA revision.
--
-- Por que una columna y no comparar contra updated_at: updated_at lo mueve
-- cualquier cosa, incluido SetOrderStatus, que solo mueve el embudo. Con ese
-- criterio, arrastrar el pedido a "descartado" un martes por la tarde hacia
-- pasar updated_at por delante del pedido nuevo del cliente y lo hacia
-- desaparecer sin que nadie lo hubiera tocado. Pasó de verdad: por eso existe
-- la columna.
--
-- NULL significa "no se sabe que llego hasta aca", que se trata como TODO
-- pendiente. Es el default correcto: mostrar de mas un pedido es un clic de
-- mas, tapar uno es un pedido que se pierde.
--
-- Quien lo mueve: la consolidacion de la IA, apply y reopen. NO lo mueve un
-- PATCH del operador, porque si lo moviera la propia edicion volveria a tapar
-- el pedido pendiente que el banner esta tratando de mostrar.
ALTER TABLE conversation_orders ADD COLUMN IF NOT EXISTS applied_analysis_at timestamptz;

ALTER TABLE conversation_orders
    ADD COLUMN IF NOT EXISTS superseded_by uuid REFERENCES conversation_orders(id);
