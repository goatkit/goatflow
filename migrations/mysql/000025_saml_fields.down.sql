-- Remove SAML2-specific fields from identity providers table.
ALTER TABLE gk_identity_provider DROP COLUMN IF EXISTS signing_cert;
ALTER TABLE gk_identity_provider DROP COLUMN IF EXISTS private_key;
ALTER TABLE gk_identity_provider DROP COLUMN IF EXISTS entity_id;
ALTER TABLE gk_identity_provider DROP COLUMN IF EXISTS acs_url;
ALTER TABLE gk_identity_provider DROP COLUMN IF EXISTS idp_metadata_xml;
