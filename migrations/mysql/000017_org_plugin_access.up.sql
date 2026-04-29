-- Per-org plugin access control. A row here means: for `org_id`, members
-- of `group_id` (customer side via group_customer_user) are entitled to
-- use `plugin_name`. Absence of any row for (org_id, plugin_name) means
-- the plugin is not enabled for that org — customers from that org are
-- denied at the middleware layer.
--
-- Agent access is NOT gated by this table; agents use the convention
-- `<plugin>-users` in `group_user` for cross-org plugin access (support
-- staff work across tenants by design). See RequirePluginAccess in
-- internal/api/plugin_handlers.go for the full auth flow.
CREATE TABLE IF NOT EXISTS gk_org_plugin_access (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    org_id      BIGINT NOT NULL,
    plugin_name VARCHAR(100) NOT NULL,
    group_id    INT NOT NULL,
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_org_plugin_group (org_id, plugin_name, group_id),
    KEY idx_org_plugin (org_id, plugin_name),
    KEY idx_plugin (plugin_name),
    KEY idx_group (group_id),

    CONSTRAINT fk_opa_org FOREIGN KEY (org_id) REFERENCES gk_organisation(id) ON DELETE CASCADE,
    CONSTRAINT fk_opa_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE,
    CONSTRAINT fk_opa_create_by FOREIGN KEY (create_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
