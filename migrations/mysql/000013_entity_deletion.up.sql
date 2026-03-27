-- Recycle bin: tracks soft-deleted entities (GoatKit PaaS Core — Entity Deletion)
CREATE TABLE IF NOT EXISTS gk_recycle_bin (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    entity_type VARCHAR(50) NOT NULL,
    entity_id   BIGINT NOT NULL,
    entity_name VARCHAR(255) DEFAULT NULL,
    deleted_by  INT NOT NULL,
    deleted_at  DATETIME NOT NULL,
    expires_at  DATETIME DEFAULT NULL,
    org_id      BIGINT DEFAULT NULL,

    PRIMARY KEY (id),
    KEY idx_entity (entity_type, entity_id),
    KEY idx_deleted_at (deleted_at),
    KEY idx_expires (expires_at),
    CONSTRAINT fk_rb_deleted_by FOREIGN KEY (deleted_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Tombstone log: immutable record that deletion happened
CREATE TABLE IF NOT EXISTS gk_deletion_log (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    entity_type VARCHAR(50) NOT NULL,
    entity_id   BIGINT NOT NULL,
    action      VARCHAR(20) NOT NULL,
    deleted_by  INT NOT NULL,
    deleted_at  DATETIME NOT NULL,
    org_id      BIGINT DEFAULT NULL,
    reason      VARCHAR(500) DEFAULT NULL,

    PRIMARY KEY (id),
    KEY idx_entity (entity_type, entity_id),
    KEY idx_deleted_at (deleted_at),
    CONSTRAINT fk_dl_deleted_by FOREIGN KEY (deleted_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
