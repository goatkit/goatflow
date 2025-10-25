-- Initial database setup for GOTRS
-- This script runs automatically when PostgreSQL container starts

-- Create Temporal databases (required for workflow engine)
CREATE DATABASE temporal;
CREATE DATABASE temporal_visibility;

-- Grant permissions to gotrs_user for Temporal databases
GRANT ALL PRIVILEGES ON DATABASE temporal TO gotrs_user;
GRANT ALL PRIVILEGES ON DATABASE temporal_visibility TO gotrs_user;

-- Switch to main database for GOTRS tables
\c gotrs_db;

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create initial schema version table
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Basic users table for MVP
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('admin', 'agent', 'customer')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Basic tickets table for MVP
CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number BIGSERIAL UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) DEFAULT 'new' CHECK (status IN ('new', 'open', 'pending', 'resolved', 'closed')),
    priority VARCHAR(50) DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'critical')),
    customer_id UUID REFERENCES users(id),
    agent_id UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ticket messages
CREATE TABLE IF NOT EXISTS ticket_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    message TEXT NOT NULL,
    is_internal BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX idx_tickets_status ON tickets(status);
CREATE INDEX idx_tickets_customer ON tickets(customer_id);
CREATE INDEX idx_tickets_agent ON tickets(agent_id);
CREATE INDEX idx_ticket_messages_ticket ON ticket_messages(ticket_id);

-- Insert default admin user
INSERT INTO users (email, password_hash, name, role) 
VALUES ('root@localhost', encode(digest(gen_random_bytes(32), 'sha256'), 'hex'), 'Admin User', 'admin')
ON CONFLICT (email) DO NOTHING;

-- Insert demo agent
INSERT INTO users (email, password_hash, name, role)
VALUES ('agent@localhost', encode(digest(gen_random_bytes(32), 'sha256'), 'hex'), 'Demo Agent', 'agent')
ON CONFLICT (email) DO NOTHING;

-- Insert demo customer
INSERT INTO users (email, password_hash, name, role)
VALUES ('customer@localhost', encode(digest(gen_random_bytes(32), 'sha256'), 'hex'), 'Demo Customer', 'customer')
ON CONFLICT (email) DO NOTHING;
