-- Batch 2: Communication and Email related tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Auto response types
CREATE TABLE IF NOT EXISTS auto_response_type (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_auto_response_type_valid_id ON auto_response_type(valid_id);

-- Auto responses
CREATE TABLE IF NOT EXISTS auto_response (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    text0 TEXT,
    text1 TEXT, 
    type_id INTEGER NOT NULL,
    system_address_id INTEGER NOT NULL,
    content_type VARCHAR(250),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (type_id) REFERENCES auto_response_type(id)
);

CREATE INDEX idx_auto_response_valid_id ON auto_response(valid_id);
CREATE INDEX idx_auto_response_type_id ON auto_response(type_id);

-- Queue to auto response mapping
CREATE TABLE IF NOT EXISTS queue_auto_response (
    queue_id INTEGER NOT NULL,
    auto_response_id INTEGER NOT NULL,
    auto_response_type_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (queue_id, auto_response_type_id),
    FOREIGN KEY (queue_id) REFERENCES queue(id) ON DELETE CASCADE,
    FOREIGN KEY (auto_response_id) REFERENCES auto_response(id) ON DELETE CASCADE,
    FOREIGN KEY (auto_response_type_id) REFERENCES auto_response_type(id) ON DELETE CASCADE
);

CREATE INDEX idx_queue_auto_response_auto_response_id ON queue_auto_response(auto_response_id);

-- System email addresses
CREATE TABLE IF NOT EXISTS system_address (
    id SERIAL PRIMARY KEY,
    value0 VARCHAR(200) NOT NULL,
    value1 VARCHAR(200) NOT NULL,
    value2 VARCHAR(200),
    value3 VARCHAR(200),
    queue_id INTEGER NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (queue_id) REFERENCES queue(id)
);

CREATE INDEX idx_system_address_valid_id ON system_address(valid_id);
CREATE INDEX idx_system_address_queue_id ON system_address(queue_id);

-- Email signatures
CREATE TABLE IF NOT EXISTS signature (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    text TEXT NOT NULL,
    content_type VARCHAR(250),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_signature_valid_id ON signature(valid_id);

-- Email salutations
CREATE TABLE IF NOT EXISTS salutation (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    text TEXT NOT NULL,
    content_type VARCHAR(250),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_salutation_valid_id ON salutation(valid_id);

-- Follow up configuration
CREATE TABLE IF NOT EXISTS follow_up_possible (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_follow_up_possible_valid_id ON follow_up_possible(valid_id);

-- Mail accounts for fetching
CREATE TABLE IF NOT EXISTS mail_account (
    id SERIAL PRIMARY KEY,
    login VARCHAR(200) NOT NULL,
    pw VARCHAR(200) NOT NULL,
    host VARCHAR(200) NOT NULL,
    account_type VARCHAR(20) NOT NULL,
    queue_id INTEGER NOT NULL,
    trusted SMALLINT NOT NULL DEFAULT 0,
    imap_folder VARCHAR(250),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (queue_id) REFERENCES queue(id)
);

CREATE INDEX idx_mail_account_valid_id ON mail_account(valid_id);
CREATE INDEX idx_mail_account_queue_id ON mail_account(queue_id);

-- Postmaster filter
CREATE TABLE IF NOT EXISTS postmaster_filter (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    stop_after_match SMALLINT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_postmaster_filter_valid_id ON postmaster_filter(valid_id);

-- Communication channels
CREATE TABLE IF NOT EXISTS communication_channel (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    module VARCHAR(200) NOT NULL,
    package_name VARCHAR(200),
    channel_data TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_communication_channel_valid_id ON communication_channel(valid_id);

-- Mail queue for outgoing emails
CREATE TABLE IF NOT EXISTS mail_queue (
    id BIGSERIAL PRIMARY KEY,
    insert_fingerprint VARCHAR(64),
    article_id BIGINT,
    attempts INTEGER NOT NULL DEFAULT 0,
    sender VARCHAR(200),
    recipient TEXT NOT NULL,
    raw_message BYTEA NOT NULL,
    due_time TIMESTAMP WITHOUT TIME ZONE,
    last_smtp_code INTEGER,
    last_smtp_message TEXT,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mail_queue_article_id ON mail_queue(article_id);
CREATE INDEX idx_mail_queue_insert_fingerprint ON mail_queue(insert_fingerprint);
CREATE INDEX idx_mail_queue_due_time ON mail_queue(due_time);