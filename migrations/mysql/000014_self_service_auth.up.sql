-- Password reset tokens and email verification (Self-Service Authentication)
CREATE TABLE IF NOT EXISTS gk_auth_token (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    token       VARCHAR(64) NOT NULL,
    token_type  VARCHAR(30) NOT NULL,
    user_type   VARCHAR(10) NOT NULL,
    user_id     INT DEFAULT NULL,
    customer_login VARCHAR(200) DEFAULT NULL,
    email       VARCHAR(255) NOT NULL,
    expires_at  DATETIME NOT NULL,
    used_at     DATETIME DEFAULT NULL,
    created_at  DATETIME NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_token (token),
    KEY idx_email (email),
    KEY idx_expires (expires_at),
    KEY idx_type (token_type, user_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Customer registration requests (approval workflow)
CREATE TABLE IF NOT EXISTS gk_registration_request (
    id              BIGINT NOT NULL AUTO_INCREMENT,
    email           VARCHAR(255) NOT NULL,
    first_name      VARCHAR(100) NOT NULL,
    last_name       VARCHAR(100) NOT NULL,
    customer_id     VARCHAR(150) DEFAULT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    approval_token  VARCHAR(64) DEFAULT NULL,
    approved_by     INT DEFAULT NULL,
    approved_at     DATETIME DEFAULT NULL,
    rejected_reason VARCHAR(500) DEFAULT NULL,
    created_at      DATETIME NOT NULL,

    PRIMARY KEY (id),
    KEY idx_email (email),
    KEY idx_status (status),
    KEY idx_approval_token (approval_token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
