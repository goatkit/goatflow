-- Plugin UI registrations (GoatKit PaaS Core — Plugin UI System)
CREATE TABLE IF NOT EXISTS gk_plugin_ui (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    plugin_name VARCHAR(100) NOT NULL,
    ui_id       VARCHAR(100) NOT NULL,
    full_id     VARCHAR(200) NOT NULL,
    name        VARCHAR(200) NOT NULL,
    description VARCHAR(500) DEFAULT NULL,
    ui_type     VARCHAR(20) NOT NULL,
    shell       VARCHAR(20) NOT NULL DEFAULT 'standard',
    icon        VARCHAR(100) DEFAULT NULL,
    config      JSON,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    custom_domain VARCHAR(255) DEFAULT NULL,
    valid_id    SMALLINT NOT NULL DEFAULT 1,
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by   INT NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_full_id (full_id),
    KEY idx_plugin (plugin_name),
    KEY idx_type (ui_type),
    CONSTRAINT fk_pui_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_pui_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
