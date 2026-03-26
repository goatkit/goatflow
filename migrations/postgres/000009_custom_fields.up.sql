-- Custom field definitions (GoatKit PaaS Core — universal custom fields)
CREATE TABLE IF NOT EXISTS gk_custom_field_def (
    id BIGSERIAL PRIMARY KEY,

    -- Identity
    name VARCHAR(200) NOT NULL,
    label VARCHAR(200) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    field_type VARCHAR(50) NOT NULL,

    -- Ownership
    owner_type VARCHAR(10) NOT NULL DEFAULT 'admin'
        CHECK (owner_type IN ('plugin','admin','legacy')),
    owner_name VARCHAR(100) DEFAULT NULL,
    migrated_from BIGINT DEFAULT NULL,

    -- Display
    section VARCHAR(100) NOT NULL DEFAULT 'custom',
    field_order INT NOT NULL DEFAULT 0,
    description VARCHAR(500) DEFAULT NULL,
    placeholder VARCHAR(200) DEFAULT NULL,

    -- Validation & config
    required BOOLEAN NOT NULL DEFAULT FALSE,
    config JSONB,

    -- Lifecycle
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL,
    create_by INT NOT NULL REFERENCES users(id),
    change_time TIMESTAMP NOT NULL,
    change_by INT NOT NULL REFERENCES users(id),

    UNIQUE (entity_type, name)
);

CREATE INDEX IF NOT EXISTS idx_cfdef_entity_type ON gk_custom_field_def(entity_type, valid_id);
CREATE INDEX IF NOT EXISTS idx_cfdef_owner ON gk_custom_field_def(owner_type, owner_name);
CREATE INDEX IF NOT EXISTS idx_cfdef_migrated ON gk_custom_field_def(migrated_from);

-- Custom field values (EAV with denormalised typed columns)
CREATE TABLE IF NOT EXISTS gk_custom_field_value (
    id BIGSERIAL PRIMARY KEY,
    field_id BIGINT NOT NULL REFERENCES gk_custom_field_def(id) ON DELETE CASCADE,
    object_id BIGINT NOT NULL,

    -- Denormalised typed columns (only one populated per row)
    val_text VARCHAR(4000) DEFAULT NULL,
    val_int BIGINT DEFAULT NULL,
    val_decimal DECIMAL(18,8) DEFAULT NULL,
    val_decimal2 DECIMAL(18,8) DEFAULT NULL,
    val_date DATE DEFAULT NULL,
    val_datetime TIMESTAMP DEFAULT NULL,
    val_json JSONB DEFAULT NULL,

    UNIQUE (field_id, object_id)
);

CREATE INDEX IF NOT EXISTS idx_cfval_object ON gk_custom_field_value(object_id);
CREATE INDEX IF NOT EXISTS idx_cfval_text ON gk_custom_field_value(field_id, val_text);
CREATE INDEX IF NOT EXISTS idx_cfval_int ON gk_custom_field_value(field_id, val_int);
CREATE INDEX IF NOT EXISTS idx_cfval_decimal ON gk_custom_field_value(field_id, val_decimal);
CREATE INDEX IF NOT EXISTS idx_cfval_date ON gk_custom_field_value(field_id, val_date);
CREATE INDEX IF NOT EXISTS idx_cfval_datetime ON gk_custom_field_value(field_id, val_datetime);
