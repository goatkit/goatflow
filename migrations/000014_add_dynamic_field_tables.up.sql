-- Batch 5: Dynamic Fields tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Dynamic fields configuration
CREATE TABLE IF NOT EXISTS dynamic_field (
    id SERIAL PRIMARY KEY,
    internal_field SMALLINT NOT NULL DEFAULT 0,
    name VARCHAR(200) NOT NULL UNIQUE,
    label VARCHAR(200) NOT NULL,
    field_order INTEGER NOT NULL,
    field_type VARCHAR(200) NOT NULL,
    object_type VARCHAR(100) NOT NULL,
    config TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_dynamic_field_field_type ON dynamic_field(field_type);
CREATE INDEX idx_dynamic_field_object_type ON dynamic_field(object_type);
CREATE INDEX idx_dynamic_field_name ON dynamic_field(name);
CREATE INDEX idx_dynamic_field_valid_id ON dynamic_field(valid_id);

-- Dynamic field values storage
CREATE TABLE IF NOT EXISTS dynamic_field_value (
    id BIGSERIAL PRIMARY KEY,
    field_id INTEGER NOT NULL,
    object_id BIGINT NOT NULL,
    value_text TEXT,
    value_date TIMESTAMP WITHOUT TIME ZONE,
    value_int BIGINT,
    FOREIGN KEY (field_id) REFERENCES dynamic_field(id) ON DELETE CASCADE
);

CREATE INDEX idx_dynamic_field_value_field_id ON dynamic_field_value(field_id);
CREATE INDEX idx_dynamic_field_value_object_id ON dynamic_field_value(object_id);
CREATE INDEX idx_dynamic_field_value_field_id_object_id ON dynamic_field_value(field_id, object_id);

-- Dynamic field object ID to name mapping
CREATE TABLE IF NOT EXISTS dynamic_field_obj_id_name (
    object_id BIGSERIAL PRIMARY KEY,
    object_name VARCHAR(200) NOT NULL UNIQUE,
    object_type VARCHAR(100) NOT NULL
);

CREATE INDEX idx_dynamic_field_obj_id_name_object_name ON dynamic_field_obj_id_name(object_name);
CREATE INDEX idx_dynamic_field_obj_id_name_object_type ON dynamic_field_obj_id_name(object_type);