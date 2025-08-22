-- Batch 8: Calendar, ACL and Link Management tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Calendar
CREATE TABLE IF NOT EXISTS calendar (
    id SERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL,
    name VARCHAR(200) NOT NULL,
    salt_string VARCHAR(64) NOT NULL,
    color VARCHAR(7) NOT NULL,
    ticket_appointments TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (group_id) REFERENCES groups(id)
);

CREATE UNIQUE INDEX idx_calendar_name ON calendar(name);
CREATE INDEX idx_calendar_group_id ON calendar(group_id);
CREATE INDEX idx_calendar_valid_id ON calendar(valid_id);

-- Calendar appointments
CREATE TABLE IF NOT EXISTS calendar_appointment (
    id BIGSERIAL PRIMARY KEY,
    parent_id BIGINT,
    calendar_id INTEGER NOT NULL,
    unique_id VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    location VARCHAR(255),
    start_time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    end_time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    all_day SMALLINT,
    notify_time TIMESTAMP WITHOUT TIME ZONE,
    notify_template VARCHAR(255),
    notify_custom VARCHAR(255),
    notify_custom_unit_count BIGINT,
    notify_custom_unit VARCHAR(255),
    notify_custom_unit_point VARCHAR(255),
    notify_custom_date TIMESTAMP WITHOUT TIME ZONE,
    team_id TEXT,
    resource_id TEXT,
    recurring SMALLINT,
    recur_type VARCHAR(20),
    recur_freq VARCHAR(255),
    recur_count INTEGER,
    recur_interval INTEGER,
    recur_until TIMESTAMP WITHOUT TIME ZONE,
    recur_id TIMESTAMP WITHOUT TIME ZONE,
    recur_exclude TEXT,
    ticket_appointment_rule_id VARCHAR(32),
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (calendar_id) REFERENCES calendar(id) ON DELETE CASCADE
);

CREATE INDEX idx_calendar_appointment_calendar_id ON calendar_appointment(calendar_id);
CREATE INDEX idx_calendar_appointment_unique_id ON calendar_appointment(unique_id);
CREATE INDEX idx_calendar_appointment_start_time ON calendar_appointment(start_time);
CREATE INDEX idx_calendar_appointment_end_time ON calendar_appointment(end_time);

-- Calendar appointment to ticket mapping
CREATE TABLE IF NOT EXISTS calendar_appointment_ticket (
    calendar_id INTEGER NOT NULL,
    ticket_id BIGINT NOT NULL,
    appointment_id BIGINT NOT NULL,
    PRIMARY KEY (calendar_id, ticket_id, appointment_id),
    FOREIGN KEY (appointment_id) REFERENCES calendar_appointment(id) ON DELETE CASCADE,
    FOREIGN KEY (calendar_id) REFERENCES calendar(id) ON DELETE CASCADE
);

CREATE INDEX idx_calendar_appointment_ticket_appointment_id ON calendar_appointment_ticket(appointment_id);
CREATE INDEX idx_calendar_appointment_ticket_ticket_id ON calendar_appointment_ticket(ticket_id);

-- Access Control Lists (ACL)
CREATE TABLE IF NOT EXISTS acl (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    description VARCHAR(250),
    stop_after_match SMALLINT,
    config_match TEXT,
    config_change TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_acl_valid_id ON acl(valid_id);

-- ACL sync state
CREATE TABLE IF NOT EXISTS acl_sync (
    acl_id VARCHAR(200) NOT NULL,
    sync_state VARCHAR(30) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Link types
CREATE TABLE IF NOT EXISTS link_type (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_link_type_valid_id ON link_type(valid_id);

-- Link states
CREATE TABLE IF NOT EXISTS link_state (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_link_state_valid_id ON link_state(valid_id);

-- Link objects
CREATE TABLE IF NOT EXISTS link_object (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE
);

-- Link relations
CREATE TABLE IF NOT EXISTS link_relation (
    id SERIAL PRIMARY KEY,
    source_object_id INTEGER NOT NULL,
    source_key VARCHAR(50) NOT NULL,
    target_object_id INTEGER NOT NULL,
    target_key VARCHAR(50) NOT NULL,
    type_id INTEGER NOT NULL,
    state_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    FOREIGN KEY (source_object_id) REFERENCES link_object(id),
    FOREIGN KEY (target_object_id) REFERENCES link_object(id),
    FOREIGN KEY (type_id) REFERENCES link_type(id),
    FOREIGN KEY (state_id) REFERENCES link_state(id)
);

CREATE UNIQUE INDEX idx_link_relation_unique ON link_relation(source_object_id, source_key, target_object_id, target_key, type_id);
CREATE INDEX idx_link_relation_source ON link_relation(source_object_id, source_key);
CREATE INDEX idx_link_relation_target ON link_relation(target_object_id, target_key);
CREATE INDEX idx_link_relation_type_id ON link_relation(type_id);
CREATE INDEX idx_link_relation_state_id ON link_relation(state_id);