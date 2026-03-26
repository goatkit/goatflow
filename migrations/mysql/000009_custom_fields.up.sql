-- Custom field definitions (GoatKit PaaS Core — universal custom fields)
CREATE TABLE IF NOT EXISTS gk_custom_field_def (
    id BIGINT NOT NULL AUTO_INCREMENT,

    -- Identity
    name VARCHAR(200) NOT NULL,
    label VARCHAR(200) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    field_type VARCHAR(50) NOT NULL,

    -- Ownership
    owner_type ENUM('plugin','admin','legacy') NOT NULL DEFAULT 'admin',
    owner_name VARCHAR(100) DEFAULT NULL,
    migrated_from BIGINT DEFAULT NULL,

    -- Display
    section VARCHAR(100) NOT NULL DEFAULT 'custom',
    field_order INT NOT NULL DEFAULT 0,
    description VARCHAR(500) DEFAULT NULL,
    placeholder VARCHAR(200) DEFAULT NULL,

    -- Validation & config
    required BOOLEAN NOT NULL DEFAULT FALSE,
    config JSON,

    -- Lifecycle
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time DATETIME NOT NULL,
    create_by INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by INT NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_entity_name (entity_type, name),
    KEY idx_entity_type (entity_type, valid_id),
    KEY idx_owner (owner_type, owner_name),
    KEY idx_migrated (migrated_from),
    CONSTRAINT fk_cfdef_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_cfdef_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Custom field values (EAV with denormalised typed columns)
CREATE TABLE IF NOT EXISTS gk_custom_field_value (
    id BIGINT NOT NULL AUTO_INCREMENT,
    field_id BIGINT NOT NULL,
    object_id BIGINT NOT NULL,

    -- Denormalised typed columns (only one populated per row)
    val_text VARCHAR(4000) DEFAULT NULL,
    val_int BIGINT DEFAULT NULL,
    val_decimal DECIMAL(18,8) DEFAULT NULL,
    val_decimal2 DECIMAL(18,8) DEFAULT NULL,
    val_date DATE DEFAULT NULL,
    val_datetime DATETIME DEFAULT NULL,
    val_json JSON DEFAULT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_field_object (field_id, object_id),
    KEY idx_object (object_id),
    KEY idx_text (field_id, val_text(191)),
    KEY idx_int (field_id, val_int),
    KEY idx_decimal (field_id, val_decimal),
    KEY idx_date (field_id, val_date),
    KEY idx_datetime (field_id, val_datetime),
    CONSTRAINT fk_cfval_field FOREIGN KEY (field_id) REFERENCES gk_custom_field_def(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
