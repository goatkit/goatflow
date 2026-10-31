-- One audit log for security-relevant events (internal/platform/audit).
-- Replaces admin_action_log, whose rows are copied across.
CREATE TABLE IF NOT EXISTS gk_audit_event (
    id BIGSERIAL PRIMARY KEY,
    create_time TIMESTAMP NOT NULL,
    category VARCHAR(20) NOT NULL,
    action VARCHAR(100) NOT NULL,
    outcome VARCHAR(20) NOT NULL,
    actor_type VARCHAR(20) NOT NULL,
    actor_id BIGINT,
    actor_login VARCHAR(255),
    ip_address VARCHAR(45),
    user_agent VARCHAR(255),
    target_type VARCHAR(100),
    target_id VARCHAR(100),
    target_label VARCHAR(255),
    details TEXT
);

CREATE INDEX IF NOT EXISTS gk_audit_event_create_time ON gk_audit_event (create_time);
CREATE INDEX IF NOT EXISTS gk_audit_event_actor ON gk_audit_event (actor_type, actor_id);
CREATE INDEX IF NOT EXISTS gk_audit_event_target ON gk_audit_event (target_type, target_id);
CREATE INDEX IF NOT EXISTS gk_audit_event_action ON gk_audit_event (category, action);

INSERT INTO gk_audit_event (
    create_time, category, action, outcome, actor_type, actor_id, actor_login,
    target_type, target_id, target_label, details
)
SELECT al.create_time, 'admin',
       CASE aat.name
           WHEN '2FADisable' THEN 'admin.2fa.disable'
           ELSE 'admin.legacy.' || LOWER(aat.name)
       END,
       'success', 'agent', al.create_by, u.login,
       al.target_type, CAST(al.target_id AS VARCHAR), al.target_identifier,
       CASE WHEN al.reason IS NULL THEN NULL ELSE json_build_object('reason', al.reason)::text END
  FROM admin_action_log al
  JOIN admin_action_type aat ON aat.id = al.action_type_id
  LEFT JOIN users u ON u.id = al.create_by
 ORDER BY al.id;

DROP TABLE IF EXISTS admin_action_log;
DROP TABLE IF EXISTS admin_action_type;
