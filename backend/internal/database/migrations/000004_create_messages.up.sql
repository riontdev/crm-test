CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    direction TEXT NOT NULL,
    text TEXT,
    attachments JSONB DEFAULT '[]',
    sender_type TEXT NOT NULL,
    sender_contact_id UUID REFERENCES contacts(id),
    platform_message_id TEXT,
    status TEXT DEFAULT 'sent',
    metadata JSONB DEFAULT '{}',
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (external_id)
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id, created_at DESC);
