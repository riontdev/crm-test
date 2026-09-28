-- ---------------------------------------------------------------------------
-- 000014: attempts y updated_at en message_analyses
-- ---------------------------------------------------------------------------
-- Por que: la 000012 creo la tabla sin las dos, y el worker las usa.
--
-- attempts lleva la cuenta de reintentos. Sin ella, MaxAttempts y el backoff
-- solo viven en memoria: al reiniciar el backend un mensaje que fallaba 3
-- veces volvia a empezar de cero y podia reintentar para siempre.
--
-- updated_at es lo que permite ver en la UI cuando se toco por ultima vez un
-- analisis, y lo que usa la consulta de "trabajos processing liberados" para
-- distinguir un job abandonado de uno que esta corriendo ahora.
-- ---------------------------------------------------------------------------

ALTER TABLE message_analyses ADD COLUMN IF NOT EXISTS attempts integer NOT NULL DEFAULT 0;

ALTER TABLE message_analyses ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- Los jobs processing que quedaron de un proceso muerto se dejan como
-- 'error' con motivo explicito, en vez de figurar como trabajo en curso para
-- siempre. recover() los vuelve a encolar igual: esto es para que la UI no
-- muestre "procesando" para siempre, no para descartarlos.
UPDATE message_analyses
   SET status = 'error',
       error = COALESCE(error, 'proceso reiniciado con el trabajo a medias'),
       updated_at = now()
 WHERE status = 'processing';
