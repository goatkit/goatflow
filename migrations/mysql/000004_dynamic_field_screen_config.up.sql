-- Dynamic Field Screen Configuration
-- Maps dynamic fields to screens with visibility settings (0=disabled, 1=enabled, 2=required)

CREATE TABLE IF NOT EXISTS dynamic_field_screen_config (
    id INT AUTO_INCREMENT PRIMARY KEY,
    field_id INT NOT NULL,
    screen_key VARCHAR(200) NOT NULL,
    config_value TINYINT NOT NULL DEFAULT 0,
    create_time DATETIME NOT NULL,
    create_by INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by INT NOT NULL,
    UNIQUE KEY idx_field_screen (field_id, screen_key),
    KEY idx_screen_key (screen_key),
    CONSTRAINT fk_dfsc_field FOREIGN KEY (field_id) REFERENCES dynamic_field(id) ON DELETE CASCADE,
    CONSTRAINT fk_dfsc_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_dfsc_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
