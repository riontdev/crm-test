CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    name text NOT NULL,
    password_hash text NOT NULL,
    role text NOT NULL DEFAULT 'agente' CHECK (role IN ('admin', 'agente')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO users (email, name, password_hash, role)
VALUES ('riontdev@gmail.com', 'Riont Admin', crypt('123456', gen_salt('bf')), 'admin');
