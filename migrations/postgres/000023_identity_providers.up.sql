-- Identity providers (SAML2, OIDC, Google, GitHub) per-tenant configuration.
CREATE TABLE gk_identity_provider (
    id              BIGSERIAL PRIMARY KEY,
    org_id          BIGINT,
    name            VARCHAR(100) NOT NULL,
    provider_type   VARCHAR(20) NOT NULL DEFAULT 'oidc',
    client_id       VARCHAR(255) NOT NULL,
    client_secret   TEXT DEFAULT '',
    discovery_url   VARCHAR(500),
    scopes          TEXT DEFAULT 'openid email profile',
    user_claim_email VARCHAR(100) DEFAULT 'email',
    user_claim_name VARCHAR(100) DEFAULT 'name',
    user_claim_groups VARCHAR(100),
    enabled         BOOLEAN DEFAULT TRUE,
    auto_provision  BOOLEAN DEFAULT TRUE,
    auto_add_to_group VARCHAR(100),
    create_time     TIMESTAMP NOT NULL,
    create_by       INT NOT NULL,
    change_time     TIMESTAMP NOT NULL,
    change_by       INT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_gk_ip_org ON gk_identity_provider(org_id);

-- Identity provider <-> organisation membership.
CREATE TABLE gk_identity_provider_org (
    provider_id BIGINT NOT NULL,
    org_id      BIGINT NOT NULL,

    PRIMARY KEY (provider_id, org_id)
);