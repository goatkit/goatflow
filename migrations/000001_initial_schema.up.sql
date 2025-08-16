-- GOTRS-CE Initial Database Schema
-- 
-- This is an original schema design for GOTRS-CE
-- Designed for compatibility with industry-standard ticketing systems
-- All SQL is originally written and not copied from other projects

-- Users table (agents and customers)
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    login VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    title VARCHAR(50),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone VARCHAR(50),
    mobile VARCHAR(50),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT users_valid_id_check CHECK (valid_id IN (1, 2, 3)) -- 1=valid, 2=invalid, 3=temp invalid
);

-- Groups (departments/queues)
CREATE TABLE groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    comment TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT groups_valid_id_check CHECK (valid_id IN (1, 2, 3))
);

-- Roles for RBAC
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    comment TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT roles_valid_id_check CHECK (valid_id IN (1, 2, 3))
);

-- Queues (ticket queues)
CREATE TABLE queues (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    group_id INTEGER NOT NULL REFERENCES groups(id),
    comment TEXT,
    system_address_id INTEGER, -- will reference system_addresses table later
    salutation_id INTEGER,     -- will reference salutations table later
    signature_id INTEGER,      -- will reference signatures table later
    unlock_timeout INTEGER DEFAULT 0,
    follow_up_id SMALLINT DEFAULT 1, -- 1=possible, 2=reject, 3=new ticket
    follow_up_lock SMALLINT DEFAULT 0, -- 0=no, 1=yes
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT queues_valid_id_check CHECK (valid_id IN (1, 2, 3)),
    CONSTRAINT queues_follow_up_id_check CHECK (follow_up_id IN (1, 2, 3)),
    CONSTRAINT queues_follow_up_lock_check CHECK (follow_up_lock IN (0, 1))
);

-- Ticket states
CREATE TABLE ticket_states (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    type_id SMALLINT NOT NULL DEFAULT 1, -- 1=new, 2=open, 3=closed, 4=removed, 5=pending
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT ticket_states_valid_id_check CHECK (valid_id IN (1, 2, 3)),
    CONSTRAINT ticket_states_type_id_check CHECK (type_id IN (1, 2, 3, 4, 5, 6))
);

-- Ticket priorities
CREATE TABLE ticket_priorities (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT ticket_priorities_valid_id_check CHECK (valid_id IN (1, 2, 3))
);

-- Main tickets table
CREATE TABLE tickets (
    id SERIAL PRIMARY KEY,
    tn VARCHAR(50) NOT NULL UNIQUE, -- ticket number
    title VARCHAR(255) NOT NULL,
    queue_id INTEGER NOT NULL REFERENCES queues(id),
    ticket_lock_id INTEGER DEFAULT 1, -- 1=unlock, 2=lock, 3=tmp_lock
    type_id INTEGER DEFAULT 1,
    service_id INTEGER,
    sla_id INTEGER,
    user_id INTEGER REFERENCES users(id), -- owner
    responsible_user_id INTEGER REFERENCES users(id),
    customer_id INTEGER REFERENCES users(id),
    customer_user_id VARCHAR(100), -- customer user login
    ticket_state_id INTEGER NOT NULL REFERENCES ticket_states(id),
    ticket_priority_id INTEGER NOT NULL REFERENCES ticket_priorities(id),
    until_time INTEGER DEFAULT 0,
    escalation_time INTEGER DEFAULT 0,
    escalation_update_time INTEGER DEFAULT 0,
    escalation_response_time INTEGER DEFAULT 0,
    escalation_solution_time INTEGER DEFAULT 0,
    archive_flag SMALLINT DEFAULT 0, -- 0=not archived, 1=archived
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT tickets_lock_id_check CHECK (ticket_lock_id IN (1, 2, 3)),
    CONSTRAINT tickets_archive_flag_check CHECK (archive_flag IN (0, 1))
);

-- Articles (messages within tickets)
CREATE TABLE articles (
    id SERIAL PRIMARY KEY,
    ticket_id INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    article_type_id SMALLINT NOT NULL DEFAULT 1, -- 1=email-external, 2=email-internal, 3=phone, 4=fax, 5=sms, 6=webrequest, 7=note-internal, 8=note-external
    sender_type_id SMALLINT NOT NULL DEFAULT 1,  -- 1=agent, 2=system, 3=customer
    communication_channel_id SMALLINT DEFAULT 1, -- 1=email, 2=phone, 3=chat, 4=internal
    is_visible_for_customer SMALLINT DEFAULT 1,  -- 0=no, 1=yes
    subject VARCHAR(255),
    body TEXT,
    body_type VARCHAR(50) DEFAULT 'text/plain', -- text/plain, text/html
    charset VARCHAR(50) DEFAULT 'utf-8',
    mime_type VARCHAR(100) DEFAULT 'text/plain',
    content_path TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT articles_valid_id_check CHECK (valid_id IN (1, 2, 3)),
    CONSTRAINT articles_type_check CHECK (article_type_id IN (1, 2, 3, 4, 5, 6, 7, 8)),
    CONSTRAINT articles_sender_check CHECK (sender_type_id IN (1, 2, 3)),
    CONSTRAINT articles_visible_check CHECK (is_visible_for_customer IN (0, 1))
);

-- Attachments
CREATE TABLE article_attachments (
    id SERIAL PRIMARY KEY,
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    content_size INTEGER NOT NULL DEFAULT 0,
    content_id VARCHAR(255),
    content_alternative VARCHAR(50),
    disposition VARCHAR(50) DEFAULT 'attachment',
    content TEXT, -- base64 encoded or file path
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- User-Group relationships (for permissions)
CREATE TABLE user_groups (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    permission_key VARCHAR(50) NOT NULL DEFAULT 'ro', -- ro=read-only, move_into, create, note, owner, priority, rw=read-write
    permission_value SMALLINT NOT NULL DEFAULT 1,     -- 0=no, 1=yes
    permission_context VARCHAR(50) DEFAULT 'Ticket',
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (user_id, group_id, permission_key)
);

-- User-Role relationships
CREATE TABLE user_roles (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (user_id, role_id)
);

-- Role-Group relationships (what groups each role can access)
CREATE TABLE role_groups (
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    permission_key VARCHAR(50) NOT NULL DEFAULT 'ro',
    permission_value SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (role_id, group_id, permission_key)
);

-- System configuration
CREATE TABLE system_config (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    value TEXT,
    description TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    CONSTRAINT system_config_valid_id_check CHECK (valid_id IN (1, 2, 3))
);

-- Session storage
CREATE TABLE sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    data TEXT,
    ip_address INET,
    user_agent TEXT,
    last_activity TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_tickets_tn ON tickets(tn);
CREATE INDEX idx_tickets_queue_id ON tickets(queue_id);
CREATE INDEX idx_tickets_state_id ON tickets(ticket_state_id);
CREATE INDEX idx_tickets_priority_id ON tickets(ticket_priority_id);
CREATE INDEX idx_tickets_user_id ON tickets(user_id);
CREATE INDEX idx_tickets_customer_id ON tickets(customer_id);
CREATE INDEX idx_tickets_create_time ON tickets(create_time);
CREATE INDEX idx_tickets_archive_flag ON tickets(archive_flag);

CREATE INDEX idx_articles_ticket_id ON articles(ticket_id);
CREATE INDEX idx_articles_create_time ON articles(create_time);
CREATE INDEX idx_articles_type_id ON articles(article_type_id);

CREATE INDEX idx_article_attachments_article_id ON article_attachments(article_id);

CREATE INDEX idx_users_login ON users(login);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_valid_id ON users(valid_id);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_last_activity ON sessions(last_activity);

CREATE INDEX idx_user_groups_user_id ON user_groups(user_id);
CREATE INDEX idx_user_groups_group_id ON user_groups(group_id);

CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);

-- Add triggers for automatic change_time updates
CREATE OR REPLACE FUNCTION update_change_time()
RETURNS TRIGGER AS $$
BEGIN
    NEW.change_time = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_users_change_time
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_change_time();

CREATE TRIGGER trigger_groups_change_time
    BEFORE UPDATE ON groups
    FOR EACH ROW
    EXECUTE FUNCTION update_change_time();

CREATE TRIGGER trigger_roles_change_time
    BEFORE UPDATE ON roles
    FOR EACH ROW
    EXECUTE FUNCTION update_change_time();

CREATE TRIGGER trigger_queues_change_time
    BEFORE UPDATE ON queues
    FOR EACH ROW
    EXECUTE FUNCTION update_change_time();

CREATE TRIGGER trigger_tickets_change_time
    BEFORE UPDATE ON tickets
    FOR EACH ROW
    EXECUTE FUNCTION update_change_time();

CREATE TRIGGER trigger_articles_change_time
    BEFORE UPDATE ON articles
    FOR EACH ROW
    EXECUTE FUNCTION update_change_time();

CREATE TRIGGER trigger_system_config_change_time
    BEFORE UPDATE ON system_config
    FOR EACH ROW
    EXECUTE FUNCTION update_change_time();