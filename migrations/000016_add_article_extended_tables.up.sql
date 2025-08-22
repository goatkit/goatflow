-- Batch 7: Additional Article and Ticket related tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Article plain text storage
CREATE TABLE IF NOT EXISTS article_data_mime_plain (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL,
    body TEXT NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_article_data_mime_plain_article_id ON article_data_mime_plain(article_id);

-- Article send errors
CREATE TABLE IF NOT EXISTS article_data_mime_send_error (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL,
    message_id VARCHAR(200),
    message TEXT NOT NULL,
    log_message TEXT,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_article_data_mime_send_error_article_id ON article_data_mime_send_error(article_id);
CREATE INDEX idx_article_data_mime_send_error_message_id ON article_data_mime_send_error(message_id);

-- Article chat data
CREATE TABLE IF NOT EXISTS article_data_otrs_chat (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL,
    chat_participant_id VARCHAR(255) NOT NULL,
    chat_participant_name VARCHAR(255) NOT NULL,
    chat_participant_type VARCHAR(255) NOT NULL,
    message_text TEXT NOT NULL,
    system_generated SMALLINT NOT NULL DEFAULT 0,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_article_data_otrs_chat_article_id ON article_data_otrs_chat(article_id);

-- Article flags
CREATE TABLE IF NOT EXISTS article_flag (
    article_id BIGINT NOT NULL,
    article_flag VARCHAR(50) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    PRIMARY KEY (article_id, create_by, article_flag)
);

CREATE INDEX idx_article_flag_article_id ON article_flag(article_id);
CREATE INDEX idx_article_flag_create_by ON article_flag(create_by);

-- Article search index
CREATE TABLE IF NOT EXISTS article_search_index (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL,
    article_type_id INTEGER NOT NULL,
    article_sender_type_id INTEGER NOT NULL,
    from_address TEXT,
    to_address TEXT,
    cc_address TEXT,
    subject TEXT,
    message_id VARCHAR(200),
    body TEXT,
    incoming_time INTEGER NOT NULL
);

CREATE INDEX idx_article_search_index_ticket_id ON article_search_index(ticket_id);
CREATE INDEX idx_article_search_index_article_type_id ON article_search_index(article_type_id);
CREATE INDEX idx_article_search_index_article_sender_type_id ON article_search_index(article_sender_type_id);
CREATE INDEX idx_article_search_index_message_id ON article_search_index(message_id);

-- Ticket flags
CREATE TABLE IF NOT EXISTS ticket_flag (
    ticket_id BIGINT NOT NULL,
    ticket_flag VARCHAR(50) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    PRIMARY KEY (ticket_id, create_by, ticket_flag)
);

CREATE INDEX idx_ticket_flag_ticket_id ON ticket_flag(ticket_id);
CREATE INDEX idx_ticket_flag_create_by ON ticket_flag(create_by);

-- Ticket index for performance
CREATE TABLE IF NOT EXISTS ticket_index (
    ticket_id BIGINT NOT NULL,
    queue_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    s_lock VARCHAR(50) NOT NULL,
    s_state VARCHAR(50) NOT NULL,
    create_time_unix BIGINT NOT NULL,
    PRIMARY KEY (ticket_id)
);

CREATE INDEX idx_ticket_index_queue_id ON ticket_index(queue_id);
CREATE INDEX idx_ticket_index_group_id ON ticket_index(group_id);
CREATE INDEX idx_ticket_index_s_lock ON ticket_index(s_lock);
CREATE INDEX idx_ticket_index_s_state ON ticket_index(s_state);
CREATE INDEX idx_ticket_index_create_time_unix ON ticket_index(create_time_unix);

-- Ticket lock index
CREATE TABLE IF NOT EXISTS ticket_lock_index (
    ticket_id BIGINT PRIMARY KEY
);

-- Ticket loop protection
CREATE TABLE IF NOT EXISTS ticket_loop_protection (
    sent_to VARCHAR(250) NOT NULL,
    sent_date VARCHAR(150) NOT NULL,
    PRIMARY KEY (sent_to, sent_date)
);

-- Ticket number counter
CREATE TABLE IF NOT EXISTS ticket_number_counter (
    id SERIAL PRIMARY KEY,
    counter BIGINT NOT NULL,
    counter_uid VARCHAR(32),
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_ticket_number_counter_uid ON ticket_number_counter(counter_uid);

-- Ticket watcher
CREATE TABLE IF NOT EXISTS ticket_watcher (
    ticket_id BIGINT NOT NULL,
    user_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (ticket_id, user_id)
);

CREATE INDEX idx_ticket_watcher_ticket_id ON ticket_watcher(ticket_id);
CREATE INDEX idx_ticket_watcher_user_id ON ticket_watcher(user_id);

-- Time accounting
CREATE TABLE IF NOT EXISTS time_accounting (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL,
    article_id BIGINT,
    time_unit DECIMAL(10,2) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_time_accounting_ticket_id ON time_accounting(ticket_id);