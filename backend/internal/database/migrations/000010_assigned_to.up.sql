ALTER TABLE conversations ADD COLUMN assigned_to uuid REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_conversations_assigned_to ON conversations(assigned_to);
