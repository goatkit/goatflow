-- Short-lived TOTP login sessions shared across app instances.
CREATE TABLE IF NOT EXISTS gk_totp_pending_session (
    session_key  VARCHAR(64) PRIMARY KEY,
    user_id      BIGINT NOT NULL DEFAULT 0,
    user_login   VARCHAR(255) NOT NULL DEFAULT '',
    username     VARCHAR(255) NOT NULL DEFAULT '',
    is_customer  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMP NOT NULL,
    expires_at   TIMESTAMP NOT NULL,
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    client_ip    VARCHAR(255) NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_totp_pending_session_expiry ON gk_totp_pending_session(expires_at);
