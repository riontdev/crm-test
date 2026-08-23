CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel TEXT NOT NULL,
    enabled BOOLEAN DEFAULT false,
    model TEXT DEFAULT 'openai/gpt-4o-mini',
    system_prompt TEXT,
    temperature NUMERIC DEFAULT 0.7,
    max_tokens INTEGER DEFAULT 1024,
    tools JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (channel)
);

-- Seed: canales nuevos arrancan APAGADOS
INSERT INTO agent_configs (channel, enabled) VALUES
    ('whatsapp', false),
    ('instagram', false),
    ('facebook', false);
