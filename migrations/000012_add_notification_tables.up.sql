-- Batch 3: Notification and Event related tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Notification events
CREATE TABLE IF NOT EXISTS notification_event (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    comments VARCHAR(250),
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_notification_event_valid_id ON notification_event(valid_id);

-- Notification event items (conditions and data)
CREATE TABLE IF NOT EXISTS notification_event_item (
    notification_id INTEGER NOT NULL,
    event_key VARCHAR(200) NOT NULL,
    event_value VARCHAR(200) NOT NULL,
    FOREIGN KEY (notification_id) REFERENCES notification_event(id) ON DELETE CASCADE
);

CREATE INDEX idx_notification_event_item_notification_id ON notification_event_item(notification_id);
CREATE INDEX idx_notification_event_item_event_key ON notification_event_item(event_key);
CREATE INDEX idx_notification_event_item_event_value ON notification_event_item(event_value);

-- Notification event messages
CREATE TABLE IF NOT EXISTS notification_event_message (
    id SERIAL PRIMARY KEY,
    notification_id INTEGER NOT NULL,
    subject VARCHAR(200) NOT NULL,
    text TEXT NOT NULL,
    content_type VARCHAR(250) NOT NULL,
    language VARCHAR(60) NOT NULL,
    FOREIGN KEY (notification_id) REFERENCES notification_event(id) ON DELETE CASCADE
);

CREATE INDEX idx_notification_event_message_notification_id ON notification_event_message(notification_id);
CREATE INDEX idx_notification_event_message_language ON notification_event_message(language);

-- Communication log
CREATE TABLE IF NOT EXISTS communication_log (
    id BIGSERIAL PRIMARY KEY,
    transport VARCHAR(200) NOT NULL,
    direction VARCHAR(200) NOT NULL,
    account_type VARCHAR(200),
    account_id VARCHAR(200),
    start_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    end_time TIMESTAMP WITHOUT TIME ZONE,
    result VARCHAR(50),
    websocket_debug_mode SMALLINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_communication_log_start_time ON communication_log(start_time);
CREATE INDEX idx_communication_log_account_type_id ON communication_log(account_type, account_id);

-- Communication log object lookup
CREATE TABLE IF NOT EXISTS communication_log_obj_lookup (
    id SERIAL PRIMARY KEY,
    communication_log_id BIGINT NOT NULL,
    object_type VARCHAR(200) NOT NULL,
    object_id BIGINT NOT NULL,
    FOREIGN KEY (communication_log_id) REFERENCES communication_log(id) ON DELETE CASCADE
);

CREATE INDEX idx_communication_log_obj_lookup_communication_log_id ON communication_log_obj_lookup(communication_log_id);
CREATE INDEX idx_communication_log_obj_lookup_object ON communication_log_obj_lookup(object_type, object_id);

-- Communication log objects
CREATE TABLE IF NOT EXISTS communication_log_object (
    id BIGSERIAL PRIMARY KEY,
    communication_log_id BIGINT NOT NULL,
    communication_id VARCHAR(200) NOT NULL,
    object_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    start_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    end_time TIMESTAMP WITHOUT TIME ZONE,
    FOREIGN KEY (communication_log_id) REFERENCES communication_log(id) ON DELETE CASCADE
);

CREATE INDEX idx_communication_log_object_communication_log_id ON communication_log_object(communication_log_id);
CREATE INDEX idx_communication_log_object_communication_id ON communication_log_object(communication_id);

-- Communication log object entries
CREATE TABLE IF NOT EXISTS communication_log_object_entry (
    id BIGSERIAL PRIMARY KEY,
    communication_log_object_id BIGINT NOT NULL,
    log_key VARCHAR(200) NOT NULL,
    log_value TEXT,
    priority VARCHAR(50) NOT NULL DEFAULT 'Info',
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (communication_log_object_id) REFERENCES communication_log_object(id) ON DELETE CASCADE
);

CREATE INDEX idx_communication_log_object_entry_object_id ON communication_log_object_entry(communication_log_object_id);
CREATE INDEX idx_communication_log_object_entry_create_time ON communication_log_object_entry(create_time);