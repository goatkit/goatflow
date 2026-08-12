-- Add SAML2-specific fields to identity providers table.
-- Idempotent: uses IF NOT EXISTS to prevent duplicate column errors
ALTER TABLE gk_identity_provider ADD COLUMN IF NOT EXISTS signing_cert TEXT DEFAULT '';
ALTER TABLE gk_identity_provider ADD COLUMN IF NOT EXISTS private_key  TEXT DEFAULT '';
ALTER TABLE gk_identity_provider ADD COLUMN IF NOT EXISTS entity_id    VARCHAR(500) DEFAULT '';
ALTER TABLE gk_identity_provider ADD COLUMN IF NOT EXISTS acs_url      VARCHAR(500) DEFAULT '';
ALTER TABLE gk_identity_provider ADD COLUMN IF NOT EXISTS idp_metadata_xml TEXT DEFAULT '';
