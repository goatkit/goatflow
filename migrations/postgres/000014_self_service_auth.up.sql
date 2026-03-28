-- Password reset tokens and email verification (Self-Service Authentication)
CREATE TABLE IF NOT EXISTS gk_auth_token (
    id              BIGSERIAL PRIMARY KEY,
    token           VARCHAR(64) NOT NULL UNIQUE,
    token_type      VARCHAR(30) NOT NULL
        CHECK (token_type IN ('password_reset','email_verify','registration_approve')),
    user_type       VARCHAR(10) NOT NULL
        CHECK (user_type IN ('agent','customer')),
    user_id         INT DEFAULT NULL,
    customer_login  VARCHAR(200) DEFAULT NULL,
    email           VARCHAR(255) NOT NULL,
    expires_at      TIMESTAMP NOT NULL,
    used_at         TIMESTAMP DEFAULT NULL,
    created_at      TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_authtoken_email ON gk_auth_token(email);
CREATE INDEX IF NOT EXISTS idx_authtoken_expires ON gk_auth_token(expires_at);

-- Customer registration requests (approval workflow)
CREATE TABLE IF NOT EXISTS gk_registration_request (
    id              BIGSERIAL PRIMARY KEY,
    email           VARCHAR(255) NOT NULL,
    first_name      VARCHAR(100) NOT NULL,
    last_name       VARCHAR(100) NOT NULL,
    customer_id     VARCHAR(150) DEFAULT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','approved','rejected')),
    approval_token  VARCHAR(64) DEFAULT NULL,
    approved_by     INT DEFAULT NULL,
    approved_at     TIMESTAMP DEFAULT NULL,
    rejected_reason VARCHAR(500) DEFAULT NULL,
    created_at      TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_regrq_email ON gk_registration_request(email);
CREATE INDEX IF NOT EXISTS idx_regrq_status ON gk_registration_request(status);
