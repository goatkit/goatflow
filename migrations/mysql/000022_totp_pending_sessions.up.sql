-- Short-lived TOTP login sessions shared across app instances.
CREATE TABLE IF NOT EXISTS gk_totp_pending_session (
    session_key  VARCHAR(64) NOT NULL,
    user_id      BIGINT NOT NULL DEFAULT 0,
    user_login   VARCHAR(255) NOT NULL DEFAULT '',
    username     VARCHAR(255) NOT NULL DEFAULT '',
    is_customer  TINYINT(1) NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL,
    expires_at   DATETIME NOT NULL,
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    client_ip    VARCHAR(255) NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL,

    PRIMARY KEY (session_key),
    KEY idx_totp_pending_session_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
