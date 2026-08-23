CREATE TABLE IF NOT EXISTS contact_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT 'zernio',
    external_id TEXT NOT NULL,
    provider_username TEXT,
    provider_name TEXT,
    provider_avatar TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (channel, external_id)
);

CREATE INDEX idx_contact_identities_contact ON contact_identities(contact_id);
