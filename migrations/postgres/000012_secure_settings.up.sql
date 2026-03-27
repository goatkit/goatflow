-- Secure encrypted configuration storage (GoatKit PaaS Core)
CREATE TABLE IF NOT EXISTS gk_secure_config (
    id              BIGSERIAL PRIMARY KEY,
    plugin_name     VARCHAR(100) NOT NULL,
    name            VARCHAR(250) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    value_hint      VARCHAR(10) DEFAULT NULL,
    org_id          BIGINT DEFAULT NULL,
    create_time     TIMESTAMP NOT NULL,
    create_by       INT NOT NULL REFERENCES users(id),
    change_time     TIMESTAMP NOT NULL,
    change_by       INT NOT NULL REFERENCES users(id),

    UNIQUE (plugin_name, name, org_id)
);

CREATE INDEX IF NOT EXISTS idx_sc_plugin ON gk_secure_config(plugin_name);
