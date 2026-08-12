-- Remove SAML2-specific fields from identity providers table.
ALTER TABLE gk_identity_provider
    DROP COLUMN IF EXISTS signing_cert,
    DROP COLUMN IF EXISTS private_key,
    DROP COLUMN IF EXISTS entity_id,
    DROP COLUMN IF EXISTS acs_url;
