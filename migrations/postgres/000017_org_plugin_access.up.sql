-- Per-org plugin access control — mirrors the MySQL migration. See the
-- MySQL copy (../mysql/000017_org_plugin_access.up.sql) for the design
-- note; duplicated here so each driver gets its own canonical DDL.
CREATE TABLE IF NOT EXISTS gk_org_plugin_access (
    id          BIGSERIAL PRIMARY KEY,
    org_id      BIGINT NOT NULL REFERENCES gk_organisation(id) ON DELETE CASCADE,
    plugin_name VARCHAR(100) NOT NULL,
    group_id    INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    create_time TIMESTAMP NOT NULL,
    create_by   INTEGER NOT NULL REFERENCES users(id),

    CONSTRAINT uk_org_plugin_group UNIQUE (org_id, plugin_name, group_id)
);

CREATE INDEX IF NOT EXISTS idx_org_plugin ON gk_org_plugin_access (org_id, plugin_name);
CREATE INDEX IF NOT EXISTS idx_plugin     ON gk_org_plugin_access (plugin_name);
CREATE INDEX IF NOT EXISTS idx_group      ON gk_org_plugin_access (group_id);
