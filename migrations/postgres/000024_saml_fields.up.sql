-- Add SAML2-specific fields to identity providers table.
ALTER TABLE gk_identity_provider
    ADD COLUMN signing_cert TEXT DEFAULT '',
    ADD COLUMN private_key  TEXT DEFAULT '',
    ADD COLUMN entity_id    VARCHAR(500) DEFAULT '',
    ADD COLUMN acs_url      VARCHAR(500) DEFAULT '';
