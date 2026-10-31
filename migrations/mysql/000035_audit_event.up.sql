-- One audit log for security-relevant events (internal/platform/audit).
-- Replaces admin_action_log, whose rows are copied across.
CREATE TABLE IF NOT EXISTS gk_audit_event (
    id BIGINT NOT NULL AUTO_INCREMENT,
    create_time DATETIME NOT NULL,
    category VARCHAR(20) NOT NULL,
    action VARCHAR(100) NOT NULL,
    outcome VARCHAR(20) NOT NULL,
    actor_type VARCHAR(20) NOT NULL,
    actor_id BIGINT NULL,
    actor_login VARCHAR(255) NULL,
    ip_address VARCHAR(45) NULL,
    user_agent VARCHAR(255) NULL,
    target_type VARCHAR(100) NULL,
    target_id VARCHAR(100) NULL,
    target_label VARCHAR(255) NULL,
    details TEXT NULL,
    PRIMARY KEY (id),
    KEY gk_audit_event_create_time (create_time),
    KEY gk_audit_event_actor (actor_type, actor_id),
    KEY gk_audit_event_target (target_type, target_id),
    KEY gk_audit_event_action (category, action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO gk_audit_event (
    create_time, category, action, outcome, actor_type, actor_id, actor_login,
    target_type, target_id, target_label, details
)
SELECT al.create_time, 'admin',
       CASE aat.name
           WHEN '2FADisable' THEN 'admin.2fa.disable'
           ELSE CONCAT('admin.legacy.', LOWER(aat.name))
       END,
       'success', 'agent', al.create_by, u.login,
       al.target_type, CAST(al.target_id AS CHAR), al.target_identifier,
       CASE WHEN al.reason IS NULL THEN NULL ELSE JSON_OBJECT('reason', al.reason) END
  FROM admin_action_log al
  JOIN admin_action_type aat ON aat.id = al.action_type_id
  LEFT JOIN users u ON u.id = al.create_by
 ORDER BY al.id;

DROP TABLE IF EXISTS admin_action_log;
DROP TABLE IF EXISTS admin_action_type;
