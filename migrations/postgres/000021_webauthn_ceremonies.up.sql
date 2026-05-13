-- Short-lived WebAuthn ceremony state shared across app instances.
CREATE TABLE IF NOT EXISTS gk_webauthn_ceremony (
    ceremony_key VARCHAR(64) PRIMARY KEY,
    user_type    VARCHAR(10) NOT NULL
        CHECK (user_type IN ('agent', 'customer')),
    user_key     VARCHAR(255) NOT NULL,
    purpose      VARCHAR(32) NOT NULL
        CHECK (purpose IN ('registration', 'login', 'passkey-login')),
    session_json JSONB NOT NULL,
    expires_at   TIMESTAMP NOT NULL,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webauthn_ceremony_expiry ON gk_webauthn_ceremony(expires_at);
