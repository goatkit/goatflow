-- Admin action type table (follows ticket_history_type pattern)
CREATE TABLE IF NOT EXISTS admin_action_type (
    id SMALLSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1 REFERENCES valid(id),
    create_time TIMESTAMP NOT NULL,
    create_by INT NOT NULL REFERENCES users(id),
    change_time TIMESTAMP NOT NULL,
    change_by INT NOT NULL REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_admin_action_type_create_by ON admin_action_type(create_by);
CREATE INDEX IF NOT EXISTS idx_admin_action_type_change_by ON admin_action_type(change_by);
CREATE INDEX IF NOT EXISTS idx_admin_action_type_valid_id ON admin_action_type(valid_id);

-- Admin action log table (follows ticket_history pattern)
CREATE TABLE IF NOT EXISTS admin_action_log (
    id BIGSERIAL PRIMARY KEY,
    action_type_id SMALLINT NOT NULL REFERENCES admin_action_type(id),
    target_type VARCHAR(50) NOT NULL,          -- 'user', 'customer', 'system'
    target_id INT,                              -- ID of affected entity
    target_identifier VARCHAR(255),            -- Username/email for display
    reason TEXT,                                -- Admin's reason for action
    details JSONB,                              -- Additional context (old values, etc.)
    create_time TIMESTAMP NOT NULL,
    create_by INT NOT NULL REFERENCES users(id) -- Admin who performed action
);

CREATE INDEX IF NOT EXISTS idx_admin_action_log_action_type_id ON admin_action_log(action_type_id);
CREATE INDEX IF NOT EXISTS idx_admin_action_log_create_by ON admin_action_log(create_by);
CREATE INDEX IF NOT EXISTS idx_admin_action_log_target ON admin_action_log(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_admin_action_log_create_time ON admin_action_log(create_time);

-- Insert initial action types
INSERT INTO admin_action_type (name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES
    ('2FADisable', 'Administrator disabled 2FA for user', 1, NOW(), 1, NOW(), 1),
    ('2FAReset', 'Administrator reset 2FA for user', 1, NOW(), 1, NOW(), 1),
    ('UserUnlock', 'Administrator unlocked user account', 1, NOW(), 1, NOW(), 1),
    ('UserLock', 'Administrator locked user account', 1, NOW(), 1, NOW(), 1),
    ('PasswordReset', 'Administrator reset user password', 1, NOW(), 1, NOW(), 1),
    ('RoleChange', 'Administrator changed user role', 1, NOW(), 1, NOW(), 1),
    ('PermissionChange', 'Administrator changed user permissions', 1, NOW(), 1, NOW(), 1)
ON CONFLICT (name) DO NOTHING;
