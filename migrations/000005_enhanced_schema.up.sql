-- GOTRS-CE Enhanced Schema Migration
-- This migration enhances our schema for better interoperability
-- All SQL is originally written for GOTRS-CE

-- Modify tickets table for broader compatibility
-- Note: Keeping customer_id as INTEGER to maintain foreign key relationships
-- ALTER TABLE tickets 
-- ALTER COLUMN customer_id TYPE VARCHAR(150) USING customer_id::VARCHAR;

-- Enhanced ticket state types for comprehensive workflow support
ALTER TABLE ticket_states 
DROP CONSTRAINT IF EXISTS ticket_states_type_id_check;

ALTER TABLE ticket_states 
ADD CONSTRAINT ticket_states_type_id_check 
CHECK (type_id BETWEEN 1 AND 10);

-- Add visual indicators to priorities (common in modern ticketing systems)
ALTER TABLE ticket_priorities 
ADD COLUMN IF NOT EXISTS display_color VARCHAR(20);

-- Extended ticket tracking fields for SLA management
ALTER TABLE tickets
ADD COLUMN IF NOT EXISTS category_id INTEGER,
ADD COLUMN IF NOT EXISTS response_deadline TIMESTAMP,
ADD COLUMN IF NOT EXISTS resolution_deadline TIMESTAMP,
ADD COLUMN IF NOT EXISTS last_customer_contact TIMESTAMP,
ADD COLUMN IF NOT EXISTS last_agent_contact TIMESTAMP;

-- Add missing columns to users table for organization support
ALTER TABLE users ADD COLUMN IF NOT EXISTS organization_id INTEGER;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_customer BOOLEAN DEFAULT false;

-- Organization management for B2B support
CREATE TABLE IF NOT EXISTS organizations (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    domain VARCHAR(100),
    support_level VARCHAR(50),
    industry VARCHAR(100),
    size VARCHAR(50),
    address_line1 VARCHAR(200),
    address_line2 VARCHAR(200),
    city VARCHAR(100),
    state_province VARCHAR(100),
    postal_code VARCHAR(20),
    country VARCHAR(100),
    website VARCHAR(200),
    notes TEXT,
    active BOOLEAN DEFAULT true,
    create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- Add foreign key constraint to users table now that organizations table exists
ALTER TABLE users ADD CONSTRAINT users_organization_id_fkey 
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE SET NULL;

-- Customer accounts linked to organizations
CREATE TABLE IF NOT EXISTS customer_accounts (
    id SERIAL PRIMARY KEY,
    username VARCHAR(200) NOT NULL UNIQUE,
    email VARCHAR(150) NOT NULL,
    organization_id INTEGER REFERENCES organizations(id),
    password_hash VARCHAR(255),
    full_name VARCHAR(200),
    phone_number VARCHAR(50),
    mobile_number VARCHAR(50),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by INTEGER NOT NULL
);

-- Email configuration for automated ticket creation
CREATE TABLE IF NOT EXISTS email_accounts (
    id SERIAL PRIMARY KEY,
    email_address VARCHAR(200) NOT NULL UNIQUE,
    display_name VARCHAR(200),
    smtp_host VARCHAR(200),
    smtp_port INTEGER,
    smtp_username VARCHAR(200),
    smtp_password_encrypted TEXT,
    imap_host VARCHAR(200),
    imap_port INTEGER,
    imap_username VARCHAR(200),
    imap_password_encrypted TEXT,
    queue_id INTEGER REFERENCES queues(id),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by INTEGER NOT NULL
);

-- Template system for automated responses
CREATE TABLE IF NOT EXISTS email_templates (
    id SERIAL PRIMARY KEY,
    template_name VARCHAR(200) NOT NULL UNIQUE,
    subject_template TEXT,
    body_template TEXT,
    template_type VARCHAR(50), -- 'greeting', 'signature', 'auto_reply', etc.
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by INTEGER NOT NULL
);

-- Ticket categories for classification
CREATE TABLE IF NOT EXISTS ticket_categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    description TEXT,
    parent_category_id INTEGER REFERENCES ticket_categories(id),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by INTEGER NOT NULL
);

-- Workflow states with clear business meaning
DELETE FROM ticket_states WHERE id <= 10;
INSERT INTO ticket_states (id, name, type_id, valid_id, create_by, change_by) VALUES
-- Initial states
(1, 'new', 1, 1, 1, 1),
(2, 'open', 2, 1, 1, 1),
-- Resolution states  
(3, 'resolved', 3, 1, 1, 1),
(4, 'closed', 3, 1, 1, 1),
-- Special states
(5, 'waiting-customer', 4, 1, 1, 1),
(6, 'waiting-approval', 4, 1, 1, 1),
(7, 'scheduled', 5, 1, 1, 1),
-- Archive states
(8, 'archived', 6, 1, 1, 1),
(9, 'deleted', 6, 1, 1, 1);

-- Priority levels with visual indicators
DELETE FROM ticket_priorities WHERE id <= 5;
INSERT INTO ticket_priorities (id, name, display_color, valid_id, create_by, change_by) VALUES
(1, 'lowest', '#0099CC', 1, 1, 1),
(2, 'low', '#66CCFF', 1, 1, 1),
(3, 'medium', '#FFCC00', 1, 1, 1),
(4, 'high', '#FF6600', 1, 1, 1),
(5, 'critical', '#CC0000', 1, 1, 1);

-- Default ticket category
INSERT INTO ticket_categories (id, name, description, created_by, updated_by) VALUES
(1, 'General', 'Default category for unclassified tickets', 1, 1)
ON CONFLICT (id) DO NOTHING;

-- Reset sequences
SELECT setval('ticket_states_id_seq', GREATEST(10, (SELECT MAX(id) FROM ticket_states)), true);
SELECT setval('ticket_priorities_id_seq', GREATEST(5, (SELECT MAX(id) FROM ticket_priorities)), true);
SELECT setval('ticket_categories_id_seq', GREATEST(1, (SELECT MAX(id) FROM ticket_categories)), true);

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_organizations_name ON organizations(name);
CREATE INDEX IF NOT EXISTS idx_customer_accounts_email ON customer_accounts(email);
CREATE INDEX IF NOT EXISTS idx_customer_accounts_org ON customer_accounts(organization_id);
CREATE INDEX IF NOT EXISTS idx_tickets_category ON tickets(category_id);
CREATE INDEX IF NOT EXISTS idx_email_accounts_queue ON email_accounts(queue_id);