-- Short-lived WebAuthn ceremony state shared across app instances.
CREATE TABLE IF NOT EXISTS gk_webauthn_ceremony (
    ceremony_key VARCHAR(64) NOT NULL,
    user_type    VARCHAR(10) NOT NULL,
    user_key     VARCHAR(255) NOT NULL,
    purpose      VARCHAR(32) NOT NULL,
    session_json JSON NOT NULL,
    expires_at   DATETIME NOT NULL,
    created_at   DATETIME NOT NULL,

    PRIMARY KEY (ceremony_key),
    KEY idx_webauthn_ceremony_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
