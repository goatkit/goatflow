-- WebAuthn / FIDO2 hardware-key credentials for MFA.
CREATE TABLE IF NOT EXISTS gk_webauthn_credential (
    id              BIGSERIAL PRIMARY KEY,
    user_type       VARCHAR(10) NOT NULL
        CHECK (user_type IN ('agent', 'customer')),
    user_key        VARCHAR(255) NOT NULL,
    credential_id   VARCHAR(500) NOT NULL UNIQUE,
    credential_json JSONB NOT NULL,
    name            VARCHAR(200) NOT NULL DEFAULT 'Security key',
    sign_count      BIGINT NOT NULL DEFAULT 0,
    last_used_at    TIMESTAMP NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webauthn_user ON gk_webauthn_credential(user_type, user_key);
