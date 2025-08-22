-- Batch 6: System Configuration tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- System configuration defaults
CREATE TABLE IF NOT EXISTS sysconfig_default (
    id SERIAL PRIMARY KEY,
    name VARCHAR(250) NOT NULL,
    description TEXT,
    navigation VARCHAR(200),
    is_invisible SMALLINT NOT NULL DEFAULT 0,
    is_readonly SMALLINT NOT NULL DEFAULT 0,
    is_required SMALLINT NOT NULL DEFAULT 0,
    is_valid SMALLINT NOT NULL DEFAULT 1,
    has_configlevel SMALLINT NOT NULL DEFAULT 0,
    user_modification_possible SMALLINT NOT NULL DEFAULT 0,
    user_modification_active SMALLINT NOT NULL DEFAULT 0,
    user_preferences_group VARCHAR(250),
    xml_content_raw TEXT NOT NULL,
    xml_content_parsed TEXT NOT NULL,
    xml_filename VARCHAR(250) NOT NULL,
    effective_value TEXT,
    is_dirty SMALLINT NOT NULL DEFAULT 0,
    exclusive_lock_guid VARCHAR(32),
    exclusive_lock_user_id INTEGER,
    exclusive_lock_expiry_time TIMESTAMP WITHOUT TIME ZONE,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_sysconfig_default_name ON sysconfig_default(name);
CREATE INDEX idx_sysconfig_default_exclusive_lock ON sysconfig_default(exclusive_lock_guid);

-- System configuration default versions
CREATE TABLE IF NOT EXISTS sysconfig_default_version (
    id SERIAL PRIMARY KEY,
    sysconfig_default_id INTEGER,
    name VARCHAR(250) NOT NULL,
    description TEXT,
    navigation VARCHAR(200),
    is_invisible SMALLINT NOT NULL DEFAULT 0,
    is_readonly SMALLINT NOT NULL DEFAULT 0,
    is_required SMALLINT NOT NULL DEFAULT 0,
    is_valid SMALLINT NOT NULL DEFAULT 1,
    has_configlevel SMALLINT NOT NULL DEFAULT 0,
    user_modification_possible SMALLINT NOT NULL DEFAULT 0,
    user_modification_active SMALLINT NOT NULL DEFAULT 0,
    user_preferences_group VARCHAR(250),
    xml_content_raw TEXT NOT NULL,
    xml_content_parsed TEXT NOT NULL,
    xml_filename VARCHAR(250) NOT NULL,
    effective_value TEXT,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_sysconfig_default_version_sysconfig_default_id ON sysconfig_default_version(sysconfig_default_id);
CREATE INDEX idx_sysconfig_default_version_name ON sysconfig_default_version(name);

-- System configuration modified settings
CREATE TABLE IF NOT EXISTS sysconfig_modified (
    id SERIAL PRIMARY KEY,
    sysconfig_default_id INTEGER NOT NULL,
    name VARCHAR(250) NOT NULL,
    user_id INTEGER,
    is_valid SMALLINT NOT NULL DEFAULT 1,
    user_modification_active SMALLINT NOT NULL DEFAULT 0,
    effective_value TEXT,
    is_dirty SMALLINT NOT NULL DEFAULT 0,
    reset_to_default SMALLINT NOT NULL DEFAULT 0,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (sysconfig_default_id) REFERENCES sysconfig_default(id)
);

CREATE UNIQUE INDEX idx_sysconfig_modified_per_user ON sysconfig_modified(sysconfig_default_id, COALESCE(user_id, 0));

-- System configuration modified versions
CREATE TABLE IF NOT EXISTS sysconfig_modified_version (
    id SERIAL PRIMARY KEY,
    sysconfig_default_version_id INTEGER NOT NULL,
    name VARCHAR(250) NOT NULL,
    user_id INTEGER,
    is_valid SMALLINT NOT NULL DEFAULT 1,
    user_modification_active SMALLINT NOT NULL DEFAULT 0,
    effective_value TEXT,
    reset_to_default SMALLINT NOT NULL DEFAULT 0,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (sysconfig_default_version_id) REFERENCES sysconfig_default_version(id)
);

CREATE INDEX idx_sysconfig_modified_version_default_version_id ON sysconfig_modified_version(sysconfig_default_version_id);

-- System configuration deployment
CREATE TABLE IF NOT EXISTS sysconfig_deployment (
    id SERIAL PRIMARY KEY,
    comments VARCHAR(250),
    user_id INTEGER,
    effective_value TEXT,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL
);

-- System configuration deployment lock
CREATE TABLE IF NOT EXISTS sysconfig_deployment_lock (
    id SERIAL PRIMARY KEY,
    exclusive_lock_guid VARCHAR(32),
    exclusive_lock_user_id INTEGER,
    exclusive_lock_expiry_time TIMESTAMP WITHOUT TIME ZONE
);

CREATE UNIQUE INDEX idx_sysconfig_deployment_lock_exclusive_lock ON sysconfig_deployment_lock(exclusive_lock_guid);

-- System maintenance schedule
CREATE TABLE IF NOT EXISTS system_maintenance (
    id SERIAL PRIMARY KEY,
    start_date INTEGER NOT NULL,
    stop_date INTEGER NOT NULL,
    comments VARCHAR(250) NOT NULL,
    login_message VARCHAR(250),
    show_login_message SMALLINT,
    notify_message VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_system_maintenance_valid_id ON system_maintenance(valid_id);