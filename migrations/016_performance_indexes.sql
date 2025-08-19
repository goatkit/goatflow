-- Performance optimization indexes for GOTRS
-- These indexes improve query performance for common operations

-- Ticket table indexes
CREATE INDEX IF NOT EXISTS idx_ticket_queue_state ON ticket(queue_id, ticket_state_id);
CREATE INDEX IF NOT EXISTS idx_ticket_customer_state ON ticket(customer_id, ticket_state_id);
CREATE INDEX IF NOT EXISTS idx_ticket_responsible_state ON ticket(responsible_user_id, ticket_state_id);
CREATE INDEX IF NOT EXISTS idx_ticket_created_state ON ticket(create_time, ticket_state_id);
CREATE INDEX IF NOT EXISTS idx_ticket_changed ON ticket(change_time DESC);
CREATE INDEX IF NOT EXISTS idx_ticket_tn ON ticket(tn);
CREATE INDEX IF NOT EXISTS idx_ticket_title_search ON ticket USING gin(to_tsvector('english', title));

-- Article table indexes
CREATE INDEX IF NOT EXISTS idx_article_ticket_type ON article(ticket_id, article_type_id);
CREATE INDEX IF NOT EXISTS idx_article_ticket_created ON article(ticket_id, create_time);
CREATE INDEX IF NOT EXISTS idx_article_sender_type ON article(article_sender_type_id, create_time);
CREATE INDEX IF NOT EXISTS idx_article_subject_search ON article USING gin(to_tsvector('english', a_subject));
CREATE INDEX IF NOT EXISTS idx_article_body_search ON article USING gin(to_tsvector('english', a_body));

-- Queue table indexes
CREATE INDEX IF NOT EXISTS idx_queue_group ON queue(group_id);
CREATE INDEX IF NOT EXISTS idx_queue_valid ON queue(valid_id);

-- Users table indexes
CREATE INDEX IF NOT EXISTS idx_users_login ON users(login);
CREATE INDEX IF NOT EXISTS idx_users_valid ON users(valid_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Customer tables indexes
CREATE INDEX IF NOT EXISTS idx_customer_user_login ON customer_user(login);
CREATE INDEX IF NOT EXISTS idx_customer_user_email ON customer_user(email);
CREATE INDEX IF NOT EXISTS idx_customer_user_customer ON customer_user(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_company_name ON customer_company(name);
CREATE INDEX IF NOT EXISTS idx_customer_company_valid ON customer_company(valid_id);

-- Role and permission indexes
CREATE INDEX IF NOT EXISTS idx_role_user_role ON role_user(role_id, user_id);
CREATE INDEX IF NOT EXISTS idx_role_user_user ON role_user(user_id, role_id);
CREATE INDEX IF NOT EXISTS idx_group_role_role ON group_role(role_id, group_id);
CREATE INDEX IF NOT EXISTS idx_group_role_group ON group_role(group_id, role_id);

-- Session indexes
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);

-- History table indexes (for audit trail)
CREATE INDEX IF NOT EXISTS idx_ticket_history_ticket ON ticket_history(ticket_id, create_time);
CREATE INDEX IF NOT EXISTS idx_ticket_history_type ON ticket_history(history_type_id, create_time);
CREATE INDEX IF NOT EXISTS idx_ticket_history_user ON ticket_history(create_by, create_time);

-- System data indexes
CREATE INDEX IF NOT EXISTS idx_system_data_key ON system_data(data_key);

-- Composite indexes for common join patterns
CREATE INDEX IF NOT EXISTS idx_ticket_queue_priority_state 
    ON ticket(queue_id, ticket_priority_id, ticket_state_id);

CREATE INDEX IF NOT EXISTS idx_ticket_responsible_owner_state 
    ON ticket(responsible_user_id, user_id, ticket_state_id);

-- Partial indexes for specific queries
CREATE INDEX IF NOT EXISTS idx_ticket_open 
    ON ticket(queue_id, create_time) 
    WHERE ticket_state_id IN (1, 2, 3); -- Open states

CREATE INDEX IF NOT EXISTS idx_ticket_pending 
    ON ticket(queue_id, responsible_user_id) 
    WHERE ticket_state_id = 4; -- Pending state

CREATE INDEX IF NOT EXISTS idx_ticket_closed 
    ON ticket(queue_id, close_time) 
    WHERE ticket_state_id IN (5, 6); -- Closed states

-- Covering indexes for read-heavy queries
CREATE INDEX IF NOT EXISTS idx_ticket_dashboard_covering 
    ON ticket(queue_id, ticket_state_id, ticket_priority_id) 
    INCLUDE (tn, title, customer_id, create_time);

CREATE INDEX IF NOT EXISTS idx_article_list_covering 
    ON article(ticket_id, create_time) 
    INCLUDE (a_subject, article_type_id, article_sender_type_id, create_by);

-- Analyze tables to update statistics
ANALYZE ticket;
ANALYZE article;
ANALYZE queue;
ANALYZE users;
ANALYZE customer_user;
ANALYZE customer_company;
ANALYZE role_user;
ANALYZE group_role;
ANALYZE ticket_history;
ANALYZE sessions;
ANALYZE system_data;