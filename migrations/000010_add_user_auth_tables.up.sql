-- Batch 1: User and Authentication related tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Roles table for role-based access control
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_roles_valid_id ON roles(valid_id);
CREATE INDEX idx_roles_name ON roles(name);

-- Role to User mapping
CREATE TABLE IF NOT EXISTS role_user (
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);

CREATE INDEX idx_role_user_role_id ON role_user(role_id);
CREATE INDEX idx_role_user_user_id ON role_user(user_id);

-- Group to Role mapping  
CREATE TABLE IF NOT EXISTS group_role (
    role_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    permission_key VARCHAR(20) NOT NULL,
    permission_value SMALLINT NOT NULL DEFAULT 0,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (role_id, group_id, permission_key),
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

CREATE INDEX idx_group_role_group_id ON group_role(group_id);
CREATE INDEX idx_group_role_role_id ON group_role(role_id);

-- User preferences storage
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id INTEGER NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value TEXT,
    PRIMARY KEY (user_id, preferences_key),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_user_preferences_user_id ON user_preferences(user_id);

-- Personal queues for users
CREATE TABLE IF NOT EXISTS personal_queues (
    user_id INTEGER NOT NULL,
    queue_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, queue_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (queue_id) REFERENCES queue(id) ON DELETE CASCADE
);

CREATE INDEX idx_personal_queues_user_id ON personal_queues(user_id);
CREATE INDEX idx_personal_queues_queue_id ON personal_queues(queue_id);

-- Personal services for users
CREATE TABLE IF NOT EXISTS personal_services (
    user_id INTEGER NOT NULL,
    service_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, service_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (service_id) REFERENCES service(id) ON DELETE CASCADE
);

CREATE INDEX idx_personal_services_user_id ON personal_services(user_id);
CREATE INDEX idx_personal_services_service_id ON personal_services(service_id);

-- Customer preferences
CREATE TABLE IF NOT EXISTS customer_preferences (
    customer_id VARCHAR(150) NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value TEXT,
    PRIMARY KEY (customer_id, preferences_key)
);

CREATE INDEX idx_customer_preferences_customer_id ON customer_preferences(customer_id);

-- Valid states lookup table
CREATE TABLE IF NOT EXISTS valid (
    id SMALLINT PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- Insert default valid states
INSERT INTO valid (id, name, create_by, change_by) VALUES 
    (1, 'valid', 1, 1),
    (2, 'invalid', 1, 1),
    (3, 'invalid-temporarily', 1, 1)
ON CONFLICT (id) DO NOTHING;

-- Group to User mapping (rename existing table for consistency)
-- Note: This might already exist as user_groups, adding if not exists
CREATE TABLE IF NOT EXISTS group_user (
    user_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    permission_key VARCHAR(20) NOT NULL,
    permission_value SMALLINT NOT NULL DEFAULT 0,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (user_id, group_id, permission_key),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

CREATE INDEX idx_group_user_group_id ON group_user(group_id);
CREATE INDEX idx_group_user_user_id ON group_user(user_id);