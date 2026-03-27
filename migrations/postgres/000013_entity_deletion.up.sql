-- Recycle bin: tracks soft-deleted entities (GoatKit PaaS Core — Entity Deletion)
CREATE TABLE IF NOT EXISTS gk_recycle_bin (
    id          BIGSERIAL PRIMARY KEY,
    entity_type VARCHAR(50) NOT NULL,
    entity_id   BIGINT NOT NULL,
    entity_name VARCHAR(255) DEFAULT NULL,
    deleted_by  INT NOT NULL REFERENCES users(id),
    deleted_at  TIMESTAMP NOT NULL,
    expires_at  TIMESTAMP DEFAULT NULL,
    org_id      BIGINT DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_rb_entity ON gk_recycle_bin(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_rb_deleted_at ON gk_recycle_bin(deleted_at);
CREATE INDEX IF NOT EXISTS idx_rb_expires ON gk_recycle_bin(expires_at);

-- Tombstone log: immutable record that deletion happened
CREATE TABLE IF NOT EXISTS gk_deletion_log (
    id          BIGSERIAL PRIMARY KEY,
    entity_type VARCHAR(50) NOT NULL,
    entity_id   BIGINT NOT NULL,
    action      VARCHAR(20) NOT NULL
        CHECK (action IN ('soft_delete','restore','hard_delete')),
    deleted_by  INT NOT NULL REFERENCES users(id),
    deleted_at  TIMESTAMP NOT NULL,
    org_id      BIGINT DEFAULT NULL,
    reason      VARCHAR(500) DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_dl_entity ON gk_deletion_log(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_dl_deleted_at ON gk_deletion_log(deleted_at);
