-- Webhook custom header values are encrypted at rest like the signing secret
-- (AES-256-GCM with GOATFLOW_SECURE_KEY, applied by the application):
-- headers_encrypted holds the encrypted JSON object of name -> value and
-- header_hints the JSON object of name -> last characters shown to admins.
-- The plaintext headers column is dropped. It only existed in pre-release
-- 0.10.0 builds; header values saved there must be entered again.
-- The create_time index serves delivery log retention (pruning old rows).
ALTER TABLE gk_webhook ADD COLUMN IF NOT EXISTS headers_encrypted VARBINARY(16384) NULL;
ALTER TABLE gk_webhook ADD COLUMN IF NOT EXISTS header_hints TEXT NULL;
ALTER TABLE gk_webhook DROP COLUMN IF EXISTS headers;
CREATE INDEX IF NOT EXISTS idx_gk_webhook_delivery_create ON gk_webhook_delivery (create_time);
