-- SAML2 identity provider columns
ALTER TABLE gk_identity_provider
  ADD COLUMN signing_cert TEXT,
  ADD COLUMN private_key TEXT,
  ADD COLUMN entity_id VARCHAR(500),
  ADD COLUMN acs_url VARCHAR(500),
  ADD COLUMN idp_metadata_xml TEXT;
