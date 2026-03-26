-- Plugin UI registrations (GoatKit PaaS Core — Plugin UI System)
CREATE TABLE IF NOT EXISTS gk_plugin_ui (
    id          BIGSERIAL PRIMARY KEY,
    plugin_name VARCHAR(100) NOT NULL,
    ui_id       VARCHAR(100) NOT NULL,
    full_id     VARCHAR(200) NOT NULL UNIQUE,
    name        VARCHAR(200) NOT NULL,
    description VARCHAR(500) DEFAULT NULL,
    ui_type     VARCHAR(20) NOT NULL
        CHECK (ui_type IN ('admin_page','agent_app','customer_app','public_page','kiosk')),
    shell       VARCHAR(20) NOT NULL DEFAULT 'standard'
        CHECK (shell IN ('none','minimal','standard')),
    icon        VARCHAR(100) DEFAULT NULL,
    config      JSONB,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    custom_domain VARCHAR(255) DEFAULT NULL,
    valid_id    SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL,
    create_by   INT NOT NULL REFERENCES users(id),
    change_time TIMESTAMP NOT NULL,
    change_by   INT NOT NULL REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_pui_plugin ON gk_plugin_ui(plugin_name);
CREATE INDEX IF NOT EXISTS idx_pui_type ON gk_plugin_ui(ui_type);
