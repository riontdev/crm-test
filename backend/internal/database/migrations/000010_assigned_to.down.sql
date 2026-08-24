DROP INDEX IF EXISTS idx_conversations_assigned_to;
ALTER TABLE conversations DROP COLUMN IF EXISTS assigned_to;
