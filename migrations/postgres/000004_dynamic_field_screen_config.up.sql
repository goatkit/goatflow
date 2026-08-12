-- Dynamic Field Screen Configuration
-- Maps dynamic fields to screens with visibility settings (0=disabled, 1=enabled, 2=required)
-- Ported from migrations/mysql/000004_dynamic_field_screen_config.up.sql

CREATE TABLE IF NOT EXISTS dynamic_field_screen_config (
    id SERIAL PRIMARY KEY,
    field_id INT NOT NULL,
    screen_key VARCHAR(200) NOT NULL,
    config_value SMALLINT NOT NULL DEFAULT 0,
    create_time TIMESTAMPTZ NOT NULL,
    create_by INT NOT NULL,
    change_time TIMESTAMPTZ NOT NULL,
    change_by INT NOT NULL,
    CONSTRAINT uq_dfsc_field_screen UNIQUE (field_id, screen_key),
    CONSTRAINT fk_dfsc_field FOREIGN KEY (field_id) REFERENCES dynamic_field(id) ON DELETE CASCADE,
    CONSTRAINT fk_dfsc_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_dfsc_change_by FOREIGN KEY (change_by) REFERENCES users(id)
);

CREATE INDEX idx_dfsc_screen_key ON dynamic_field_screen_config (screen_key);
