-- Admin action type table (follows ticket_history_type pattern)
CREATE TABLE IF NOT EXISTS admin_action_type (
    id SMALLINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250) NULL,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time DATETIME NOT NULL,
    create_by INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by INT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY admin_action_type_name (name),
    KEY FK_admin_action_type_create_by (create_by),
    KEY FK_admin_action_type_change_by (change_by),
    KEY FK_admin_action_type_valid_id (valid_id),
    CONSTRAINT FK_admin_action_type_create_by FOREIGN KEY (create_by) REFERENCES users (id),
    CONSTRAINT FK_admin_action_type_change_by FOREIGN KEY (change_by) REFERENCES users (id),
    CONSTRAINT FK_admin_action_type_valid_id FOREIGN KEY (valid_id) REFERENCES valid (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Admin action log table (follows ticket_history pattern)
CREATE TABLE IF NOT EXISTS admin_action_log (
    id BIGINT NOT NULL AUTO_INCREMENT,
    action_type_id SMALLINT NOT NULL,
    target_type VARCHAR(50) NOT NULL,         -- 'user', 'customer', 'system'
    target_id INT NULL,                        -- ID of affected entity
    target_identifier VARCHAR(255) NULL,       -- Username/email for display
    reason TEXT NULL,                          -- Admin's reason for action
    details JSON NULL,                         -- Additional context (old values, etc.)
    create_time DATETIME NOT NULL,
    create_by INT NOT NULL,                    -- Admin who performed action
    PRIMARY KEY (id),
    KEY FK_admin_action_log_action_type_id (action_type_id),
    KEY FK_admin_action_log_create_by (create_by),
    KEY admin_action_log_target (target_type, target_id),
    KEY admin_action_log_create_time (create_time),
    CONSTRAINT FK_admin_action_log_action_type_id FOREIGN KEY (action_type_id) REFERENCES admin_action_type (id),
    CONSTRAINT FK_admin_action_log_create_by FOREIGN KEY (create_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insert initial action types
INSERT INTO admin_action_type (name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES
    ('2FADisable', 'Administrator disabled 2FA for user', 1, NOW(), 1, NOW(), 1),
    ('2FAReset', 'Administrator reset 2FA for user', 1, NOW(), 1, NOW(), 1),
    ('UserUnlock', 'Administrator unlocked user account', 1, NOW(), 1, NOW(), 1),
    ('UserLock', 'Administrator locked user account', 1, NOW(), 1, NOW(), 1),
    ('PasswordReset', 'Administrator reset user password', 1, NOW(), 1, NOW(), 1),
    ('RoleChange', 'Administrator changed user role', 1, NOW(), 1, NOW(), 1),
    ('PermissionChange', 'Administrator changed user permissions', 1, NOW(), 1, NOW(), 1);
