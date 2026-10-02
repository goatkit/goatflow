-- Outbound webhooks: endpoint configuration, delivery log, and the event-source
-- cursors the dispatcher uses to turn ticket/article/history rows into events.
CREATE TABLE IF NOT EXISTS gk_webhook (
    id               BIGINT NOT NULL AUTO_INCREMENT,
    name             VARCHAR(200) NOT NULL,
    url              VARCHAR(2000) NOT NULL,
    secret_encrypted VARBINARY(1024) NULL,
    secret_hint      VARCHAR(10) NULL,
    events           TEXT NOT NULL,
    headers          TEXT NULL,
    retry_count      INT NOT NULL DEFAULT 3,
    timeout_seconds  INT NOT NULL DEFAULT 10,
    valid_id         SMALLINT NOT NULL DEFAULT 1,
    create_time      DATETIME NOT NULL,
    create_by        INT NOT NULL,
    change_time      DATETIME NOT NULL,
    change_by        INT NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_gk_webhook_name (name),
    CONSTRAINT fk_gk_webhook_valid FOREIGN KEY (valid_id) REFERENCES valid(id),
    CONSTRAINT fk_gk_webhook_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_gk_webhook_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS gk_webhook_delivery (
    id                BIGINT NOT NULL AUTO_INCREMENT,
    webhook_id        BIGINT NOT NULL,
    event_type        VARCHAR(100) NOT NULL,
    payload           MEDIUMTEXT NOT NULL,
    status            VARCHAR(20) NOT NULL,
    attempts          INT NOT NULL DEFAULT 0,
    status_code       INT NULL,
    response_body     TEXT NULL,
    error_message     VARCHAR(1000) NULL,
    duration_ms       INT NULL,
    next_attempt_time DATETIME NULL,
    delivered_time    DATETIME NULL,
    create_time       DATETIME NOT NULL,
    change_time       DATETIME NOT NULL,

    PRIMARY KEY (id),
    KEY idx_gk_webhook_delivery_webhook (webhook_id, create_time),
    KEY idx_gk_webhook_delivery_due (status, next_attempt_time),
    CONSTRAINT fk_gk_webhook_delivery_webhook FOREIGN KEY (webhook_id) REFERENCES gk_webhook(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS gk_webhook_event_cursor (
    source      VARCHAR(50) NOT NULL,
    last_id     BIGINT NOT NULL,
    change_time DATETIME NOT NULL,

    PRIMARY KEY (source)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
