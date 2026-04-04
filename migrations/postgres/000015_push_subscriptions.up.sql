CREATE TABLE IF NOT EXISTS gk_push_subscription (
    id          BIGSERIAL PRIMARY KEY,
    user_id     INT NOT NULL,
    user_type   VARCHAR(10) NOT NULL DEFAULT 'agent'
        CHECK (user_type IN ('agent', 'customer')),
    endpoint    VARCHAR(500) NOT NULL UNIQUE,
    p256dh      VARCHAR(200) NOT NULL,
    auth        VARCHAR(100) NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_push_user ON gk_push_subscription(user_id, user_type);
