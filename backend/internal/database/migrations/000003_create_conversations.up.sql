CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT 'zernio',
    zernio_conversation_id TEXT NOT NULL,
    zernio_account_id TEXT,
    platform_conversation_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    last_inbound_at TIMESTAMPTZ,
    unread_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (channel, zernio_conversation_id)
);

CREATE INDEX idx_conversations_contact ON conversations(contact_id);
CREATE INDEX idx_conversations_status ON conversations(status, updated_at DESC);
