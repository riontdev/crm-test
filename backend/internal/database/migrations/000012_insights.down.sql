DROP TABLE IF EXISTS message_analyses;
DROP TABLE IF EXISTS conversation_orders;
DROP TABLE IF EXISTS product_catalog;
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS insight_configs;

ALTER TABLE conversations DROP COLUMN IF EXISTS insight_enabled;
