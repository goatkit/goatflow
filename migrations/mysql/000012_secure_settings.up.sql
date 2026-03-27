-- Secure encrypted configuration storage (GoatKit PaaS Core)
CREATE TABLE IF NOT EXISTS gk_secure_config (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    plugin_name VARCHAR(100) NOT NULL,
    name        VARCHAR(250) NOT NULL,
    encrypted_value VARBINARY(4096) NOT NULL,
    value_hint  VARCHAR(10) DEFAULT NULL,
    org_id      BIGINT DEFAULT NULL,
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by   INT NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_plugin_name_org (plugin_name, name, org_id),
    KEY idx_plugin (plugin_name),
    CONSTRAINT fk_sc_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_sc_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
