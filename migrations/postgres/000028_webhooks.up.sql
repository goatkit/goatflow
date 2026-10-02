-- Outbound webhooks: endpoint configuration, delivery log, and the event-source
-- cursors the dispatcher uses to turn ticket/article/history rows into events.
CREATE TABLE IF NOT EXISTS gk_webhook (
    id               BIGSERIAL PRIMARY KEY,
    name             VARCHAR(200) NOT NULL,
    url              VARCHAR(2000) NOT NULL,
    secret_encrypted BYTEA NULL,
    secret_hint      VARCHAR(10) NULL,
    events           TEXT NOT NULL,
    headers          TEXT NULL,
    retry_count      INT NOT NULL DEFAULT 3,
    timeout_seconds  INT NOT NULL DEFAULT 10,
    valid_id         SMALLINT NOT NULL DEFAULT 1 REFERENCES valid(id),
    create_time      TIMESTAMP NOT NULL,
    create_by        INT NOT NULL REFERENCES users(id),
    change_time      TIMESTAMP NOT NULL,
    change_by        INT NOT NULL REFERENCES users(id),

    CONSTRAINT uk_gk_webhook_name UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS gk_webhook_delivery (
    id                BIGSERIAL PRIMARY KEY,
    webhook_id        BIGINT NOT NULL REFERENCES gk_webhook(id) ON DELETE CASCADE,
    event_type        VARCHAR(100) NOT NULL,
    payload           TEXT NOT NULL,
    status            VARCHAR(20) NOT NULL,
    attempts          INT NOT NULL DEFAULT 0,
    status_code       INT NULL,
    response_body     TEXT NULL,
    error_message     VARCHAR(1000) NULL,
    duration_ms       INT NULL,
    next_attempt_time TIMESTAMP NULL,
    delivered_time    TIMESTAMP NULL,
    create_time       TIMESTAMP NOT NULL,
    change_time       TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_gk_webhook_delivery_webhook ON gk_webhook_delivery(webhook_id, create_time);
CREATE INDEX IF NOT EXISTS idx_gk_webhook_delivery_due ON gk_webhook_delivery(status, next_attempt_time);

CREATE TABLE IF NOT EXISTS gk_webhook_event_cursor (
    source      VARCHAR(50) PRIMARY KEY,
    last_id     BIGINT NOT NULL,
    change_time TIMESTAMP NOT NULL
);
