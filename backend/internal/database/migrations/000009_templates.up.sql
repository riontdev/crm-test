CREATE TABLE templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    category text NOT NULL DEFAULT 'general' CHECK (category IN ('marketing','utility','soporte','general')),
    content text NOT NULL,
    language text NOT NULL DEFAULT 'es',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO templates (name, category, content) VALUES
('Bienvenida', 'general', '¡Hola {{1}}! 👋 Gracias por escribirnos. ¿En qué podemos ayudarte hoy?'),
('Horarios', 'utility', 'Nuestros horarios de atención son de lunes a viernes de {{1}} a {{2}} hs. ¡Escribinos en cualquier momento y te respondemos!'),
('Seguimiento', 'soporte', 'Hola {{1}}, te comento que estamos revisando tu caso. Te vamos a responder a la brevedad. 🙌'),
('Promo', 'marketing', '🔥 {{1}}: aprovechá un {{2}}% de descuento esta semana. Escribí SI para que te lo apliquemos.');
