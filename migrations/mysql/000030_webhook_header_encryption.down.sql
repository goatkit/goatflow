-- Restores the 000028 layout. Encrypted header values cannot be decrypted in
-- SQL, so custom headers are lost and must be entered again.
DROP INDEX IF EXISTS idx_gk_webhook_delivery_create ON gk_webhook_delivery;
ALTER TABLE gk_webhook ADD COLUMN IF NOT EXISTS headers TEXT NULL;
ALTER TABLE gk_webhook DROP COLUMN IF EXISTS header_hints;
ALTER TABLE gk_webhook DROP COLUMN IF EXISTS headers_encrypted;
