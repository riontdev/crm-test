-- Revertir: quitar la marca de no entregado y restaurar el estado 'sent'.
UPDATE messages m
SET status = 'sent',
    metadata = (m.metadata - 'undelivered_window_closed')
FROM conversations c
WHERE m.conversation_id = c.id
  AND c.channel = 'whatsapp'
  AND m.direction = 'outgoing'
  AND (m.metadata ? 'undelivered_window_closed')
  AND COALESCE((m.metadata -> 'template')::boolean, false) = false;
