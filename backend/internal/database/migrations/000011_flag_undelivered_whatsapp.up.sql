-- Retrofirma mensajes de WhatsApp enviados fuera de la ventana de 24h.
-- WhatsApp rechaza en silencio los mensajes libres fuera de la ventana: Zernio
-- devolvía wamid y se guardaban como 'sent' aunque el destinatario nunca los
-- recibió. Aquí marcamos como 'failed' (icono de error en el frontend) los
-- mensajes salientes que se enviaron cuando la ventana ya estaba vencida.
--
-- Criterio: mensaje sales (no plantilla) de una conversación whatsapp cuyo
-- último inbound ANTERIOR al envío está a más de 24h (o no existía inbound).

UPDATE messages m
SET status = 'failed',
    metadata = COALESCE(m.metadata, '{}'::jsonb) || '{"undelivered_window_closed": true}'::jsonb
FROM conversations c
WHERE m.conversation_id = c.id
  AND c.channel = 'whatsapp'
  AND m.direction = 'outgoing'
  AND m.status <> 'failed'
  AND COALESCE((m.metadata -> 'template')::boolean, false) = false
  AND (
    NOT EXISTS (
      SELECT 1 FROM messages inbound
      WHERE inbound.conversation_id = m.conversation_id
        AND inbound.direction = 'incoming'
        AND inbound.sent_at <= m.sent_at
    )
    OR (m.sent_at - (
        SELECT MAX(inbound2.sent_at) FROM messages inbound2
        WHERE inbound2.conversation_id = m.conversation_id
          AND inbound2.direction = 'incoming'
          AND inbound2.sent_at <= m.sent_at
      )) > INTERVAL '24 hours'
  );
