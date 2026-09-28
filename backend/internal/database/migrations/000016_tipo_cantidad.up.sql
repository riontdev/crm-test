-- ---------------------------------------------------------------------------
-- 000016: tipo_cantidad (delta vs total) en el analysis
-- ---------------------------------------------------------------------------
-- Por que: el merge consolidaba SIEMPRE como si la cantidad del analysis fuera
-- el total del pedido. El modelo, en cambio, es inconsistente: ante "sumale 2
-- stickers mas" con un pedido de 30 devuelve a veces 32 (total) y a veces 2
-- (delta). Medido sobre granite3.3:2b: 5 de 8 redacciones naturales de "sumale
-- N mas" devuelven el total, las otras 3 devuelven el incremento.
--
-- Con MergeOrder reemplazando siempre, esas 3BORRABAN 28 stickers: el pedido pasaba de 30
-- stickers a 2 y no habia error, ni warning, ni needs_review (la IA contestaba
-- confianza 0.9). El pedido se perdia en silencio, que es exactamente lo que
-- las reglas duras del modulo prohiben.
--
-- Como se resuelve: el analysis DECLARA que numero empejo. tipo_cantidad dice
-- si el numero se suma ("delta") o si es el total ("total"), y MergeOrder
-- obedece esa declaracion con codigo determinista. La decision deja de
-- depender de que el modelo la ejecute bien.
--
-- Por que va en message_analyses y no solo en la llamada a Ollama: reopen y
-- accept-pending re-aplican un analysis ALMACENADO, a veces dias despues. Si el
-- flag viviera solo en memoria, reaplicar un analysis viejo caeria en el caso
-- ambiguo y el operador veria el pedido sin cambio, con la duda de por que.
--
-- NULL en las filas viejas = "no sabemos": se aplica la regla conservadora (no
-- tocar el pedido y marcar needs_review). Es el default correcto para una
-- columna que decide si se pisa informacion ya consolidada.
-- ---------------------------------------------------------------------------

ALTER TABLE message_analyses ADD COLUMN IF NOT EXISTS tipo_cantidad text;

-- delta: el numero se SUMA a lo ya pedido.
-- total: el numero REEMPLAZA la cantidad de esa linea.
-- NULL: desconocido, no se toca el pedido (queda para revision del operador).
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'message_analyses_tipo_cantidad_check'
  ) THEN
    ALTER TABLE message_analyses
      ADD CONSTRAINT message_analyses_tipo_cantidad_check
      CHECK (tipo_cantidad IS NULL OR tipo_cantidad IN ('delta', 'total'));
  END IF;
END $$;
