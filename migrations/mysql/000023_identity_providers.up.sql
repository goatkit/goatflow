-- Identity providers (SAML2, OIDC, Google, GitHub) per-tenant configuration.
CREATE TABLE IF NOT EXISTS gk_identity_provider (
    id                  BIGINT AUTO_INCREMENT PRIMARY KEY,
    org_id              BIGINT NULL,
    name                VARCHAR(100) NOT NULL,
    provider_type       ENUM('oidc','google','github','saml2') NOT NULL DEFAULT 'oidc',
    client_id           VARCHAR(255) NOT NULL,
    client_secret       TEXT DEFAULT '',
    discovery_url       VARCHAR(500) NULL,
    scopes              TEXT DEFAULT 'openid email profile',
    user_claim_email    VARCHAR(100) DEFAULT 'email',
    user_claim_name     VARCHAR(100) DEFAULT 'name',
    user_claim_groups   VARCHAR(100) NULL,
    enabled             TINYINT(1) DEFAULT 1,
    auto_provision      TINYINT(1) DEFAULT 1,
    auto_add_to_group   VARCHAR(100) NULL,
    user_table          VARCHAR(20) NOT NULL DEFAULT 'users',
    create_time         DATETIME NOT NULL,
    create_by           INT NOT NULL,
    change_time         DATETIME NOT NULL,
    change_by           INT NOT NULL,
    signing_cert        TEXT DEFAULT '',
    private_key         TEXT DEFAULT '',
    entity_id           VARCHAR(500) DEFAULT '',
    acs_url             VARCHAR(500) DEFAULT '',
    idp_metadata_xml    TEXT DEFAULT '',

    KEY idx_gk_ip_org (org_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Identity provider <-> organisation membership.
CREATE TABLE IF NOT EXISTS gk_identity_provider_org (
    provider_id BIGINT NOT NULL,
    org_id      BIGINT NOT NULL,

    PRIMARY KEY (provider_id, org_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
