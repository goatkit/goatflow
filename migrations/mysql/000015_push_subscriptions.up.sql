CREATE TABLE IF NOT EXISTS gk_push_subscription (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    user_id     INT NOT NULL,
    user_type   VARCHAR(10) NOT NULL DEFAULT 'agent',
    endpoint    VARCHAR(500) NOT NULL,
    p256dh      VARCHAR(200) NOT NULL,
    auth        VARCHAR(100) NOT NULL,
    created_at  DATETIME NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_push_endpoint (endpoint),
    KEY idx_push_user (user_id, user_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
