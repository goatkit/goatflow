-- WebAuthn / FIDO2 hardware-key credentials for MFA.
CREATE TABLE IF NOT EXISTS gk_webauthn_credential (
    id              BIGINT NOT NULL AUTO_INCREMENT,
    user_type       VARCHAR(10) NOT NULL,
    user_key        VARCHAR(255) NOT NULL,
    credential_id   VARCHAR(500) NOT NULL,
    credential_json JSON NOT NULL,
    name            VARCHAR(200) NOT NULL DEFAULT 'Security key',
    sign_count      BIGINT NOT NULL DEFAULT 0,
    last_used_at    DATETIME NULL,
    created_at      DATETIME NOT NULL,
    updated_at      DATETIME NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_webauthn_credential_id (credential_id),
    KEY idx_webauthn_user (user_type, user_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
