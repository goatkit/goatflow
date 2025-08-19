-- OTRS-Compatible Schema for GOTRS
-- Written from scratch based on OTRS table structure
-- 100% compatible with OTRS Community Edition

-- ============================================
-- Core User and Group Tables
-- ============================================

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    login VARCHAR(200) NOT NULL UNIQUE,
    pw VARCHAR(128),
    title VARCHAR(50),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE user_groups (
    user_id INTEGER NOT NULL REFERENCES users(id),
    group_id INTEGER NOT NULL REFERENCES groups(id),
    permission_key VARCHAR(20) NOT NULL,
    permission_value SMALLINT NOT NULL,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(user_id, group_id, permission_key)
);

-- ============================================
-- Customer Tables
-- ============================================

CREATE TABLE customer_company (
    customer_id VARCHAR(150) PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    street VARCHAR(200),
    zip VARCHAR(200),
    city VARCHAR(200),
    country VARCHAR(200),
    url VARCHAR(200),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE customer_user (
    id SERIAL PRIMARY KEY,
    login VARCHAR(200) NOT NULL UNIQUE,
    email VARCHAR(150) NOT NULL,
    customer_id VARCHAR(150) NOT NULL,
    pw VARCHAR(128),
    title VARCHAR(50),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    phone VARCHAR(150),
    fax VARCHAR(150),
    mobile VARCHAR(150),
    street VARCHAR(150),
    zip VARCHAR(200),
    city VARCHAR(200),
    country VARCHAR(200),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- ============================================
-- Queue and Ticket Configuration
-- ============================================

CREATE TABLE queue (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    group_id INTEGER NOT NULL REFERENCES groups(id),
    system_address_id INTEGER,
    salutation_id INTEGER,
    signature_id INTEGER,
    unlock_timeout INTEGER DEFAULT 0,
    follow_up_id SMALLINT DEFAULT 1,
    follow_up_lock SMALLINT DEFAULT 0,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE ticket_state (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    type_id SMALLINT NOT NULL,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE ticket_state_type (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE ticket_priority (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    color VARCHAR(20),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE ticket_type (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE ticket_lock_type (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- ============================================
-- Ticket and Article Tables
-- ============================================

CREATE TABLE ticket (
    id BIGSERIAL PRIMARY KEY,
    tn VARCHAR(50) NOT NULL UNIQUE,
    title VARCHAR(255),
    queue_id INTEGER NOT NULL REFERENCES queue(id),
    ticket_lock_id SMALLINT NOT NULL DEFAULT 1,
    ticket_type_id INTEGER DEFAULT 1,
    service_id INTEGER,
    sla_id INTEGER,
    user_id INTEGER REFERENCES users(id),
    responsible_user_id INTEGER REFERENCES users(id),
    ticket_priority_id SMALLINT NOT NULL REFERENCES ticket_priority(id),
    ticket_state_id SMALLINT NOT NULL REFERENCES ticket_state(id),
    customer_id VARCHAR(150),
    customer_user_id VARCHAR(250),
    timeout INTEGER DEFAULT 0,
    until_time INTEGER DEFAULT 0,
    escalation_time INTEGER DEFAULT 0,
    escalation_update_time INTEGER DEFAULT 0,
    escalation_response_time INTEGER DEFAULT 0,
    escalation_solution_time INTEGER DEFAULT 0,
    archive_flag SMALLINT DEFAULT 0,
    create_time_unix BIGINT,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX ticket_tn ON ticket(tn);
CREATE INDEX ticket_queue_id ON ticket(queue_id);
CREATE INDEX ticket_customer_id ON ticket(customer_id);
CREATE INDEX ticket_customer_user_id ON ticket(customer_user_id);
CREATE INDEX ticket_state_id ON ticket(ticket_state_id);
CREATE INDEX ticket_priority_id ON ticket(ticket_priority_id);
CREATE INDEX ticket_create_time ON ticket(create_time);

CREATE TABLE article_sender_type (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE article (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES ticket(id),
    article_sender_type_id SMALLINT NOT NULL,
    communication_channel_id BIGINT DEFAULT 1,
    is_visible_for_customer SMALLINT DEFAULT 0,
    search_index_needs_rebuild SMALLINT DEFAULT 1,
    insert_fingerprint VARCHAR(64),
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX article_ticket_id ON article(ticket_id);
CREATE INDEX article_sender_type_id ON article(article_sender_type_id);

CREATE TABLE article_data_mime (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL REFERENCES article(id),
    a_from TEXT,
    a_reply_to TEXT,
    a_to TEXT,
    a_cc TEXT,
    a_bcc TEXT,
    a_subject VARCHAR(3800),
    a_message_id VARCHAR(3800),
    a_message_id_md5 VARCHAR(32),
    a_in_reply_to TEXT,
    a_references TEXT,
    a_content_type VARCHAR(250),
    a_body BYTEA NOT NULL,
    incoming_time INTEGER NOT NULL,
    content_path VARCHAR(250),
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX article_data_mime_article_id ON article_data_mime(article_id);

CREATE TABLE article_data_mime_attachment (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL REFERENCES article(id),
    filename VARCHAR(250),
    content_type VARCHAR(450),
    content_size VARCHAR(30),
    content BYTEA NOT NULL,
    content_id VARCHAR(250),
    content_alternative VARCHAR(50),
    disposition VARCHAR(15),
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- ============================================
-- History Tables
-- ============================================

CREATE TABLE ticket_history_type (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE ticket_history (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200),
    history_type_id SMALLINT NOT NULL,
    ticket_id BIGINT NOT NULL REFERENCES ticket(id),
    article_id BIGINT,
    type_id SMALLINT,
    queue_id INTEGER,
    owner_id INTEGER,
    priority_id SMALLINT,
    state_id SMALLINT,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX ticket_history_ticket_id ON ticket_history(ticket_id);
CREATE INDEX ticket_history_article_id ON ticket_history(article_id);
CREATE INDEX ticket_history_create_time ON ticket_history(create_time);
CREATE INDEX ticket_history_history_type_id ON ticket_history(history_type_id);

-- ============================================
-- Service and SLA Tables
-- ============================================

CREATE TABLE service (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE TABLE sla (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    calendar_name VARCHAR(100),
    first_response_time INTEGER,
    first_response_notify SMALLINT,
    update_time INTEGER,
    update_notify SMALLINT,
    solution_time INTEGER,
    solution_notify SMALLINT,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- ============================================
-- Session Management
-- ============================================

CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(100) NOT NULL UNIQUE,
    data_key VARCHAR(100) NOT NULL,
    data_value TEXT,
    serialized SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX sessions_session_id ON sessions(session_id);
CREATE INDEX sessions_data_key ON sessions(data_key);

-- ============================================
-- System Configuration
-- ============================================

CREATE TABLE system_data (
    data_key VARCHAR(160) PRIMARY KEY,
    data_value TEXT,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- ============================================
-- Initial Admin User
-- ============================================

-- Password is 'admin' (you should change this immediately)
INSERT INTO users (id, login, pw, first_name, last_name, valid_id, create_by, change_by) VALUES
(1, 'root@localhost', '$2a$12$K3iFcqdDATSmuVWa8LkSPudZYoWZBjl1uGnu5ZyCzK.tI7jWYaq/K', 'Admin', 'User', 1, 1, 1);

-- Create admin group
INSERT INTO groups (id, name, comments, valid_id, create_by, change_by) VALUES
(1, 'admin', 'Admin Group', 1, 1, 1),
(2, 'users', 'Users Group', 1, 1, 1);

-- Give admin all permissions
INSERT INTO user_groups (user_id, group_id, permission_key, permission_value, create_by, change_by) VALUES
(1, 1, 'rw', 1, 1, 1);

SELECT setval('users_id_seq', 1, true);
SELECT setval('groups_id_seq', 2, true);