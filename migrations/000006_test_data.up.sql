-- Simplified Test Data Migration for Development and Testing
-- This migration provides minimal test data that works with the schema

-- Check environment (will fail in production)
DO $$
BEGIN
    IF current_setting('app.env', true) = 'production' THEN
        RAISE EXCEPTION 'Test data migration cannot be run in production environment';
    END IF;
END $$;

-- Organizations
INSERT INTO organizations (id, name, domain, support_level, industry, size, active, create_by, change_by) VALUES
(1, 'Acme Corporation', 'acme.com', 'platinum', 'Technology', 'enterprise', true, 1, 1),
(2, 'TechStart Inc', 'techstart.io', 'gold', 'Technology', 'startup', true, 1, 1),
(3, 'CloudScale Systems', 'cloudscale.net', 'gold', 'Cloud Services', 'medium', true, 1, 1);
SELECT setval('organizations_id_seq', 3, true);

-- Test users (agents and customers)
INSERT INTO users (id, login, password_hash, first_name, last_name, email, organization_id, is_customer, create_by, change_by) VALUES
(3, 'agent.smith', '$2a$12$K3iFcqdDATSmuVWa8LkSPudZYoWZBjl1uGnu5ZyCzK.tI7jWYaq/K', 'Agent', 'Smith', 'agent.smith@support.local', NULL, false, 1, 1),
(4, 'agent.jones', '$2a$12$K3iFcqdDATSmuVWa8LkSPudZYoWZBjl1uGnu5ZyCzK.tI7jWYaq/K', 'Agent', 'Jones', 'agent.jones@support.local', NULL, false, 1, 1),
(5, 'john.customer', '$2a$12$K3iFcqdDATSmuVWa8LkSPudZYoWZBjl1uGnu5ZyCzK.tI7jWYaq/K', 'John', 'Customer', 'john@acme.com', 1, true, 1, 1),
(6, 'jane.customer', '$2a$12$K3iFcqdDATSmuVWa8LkSPudZYoWZBjl1uGnu5ZyCzK.tI7jWYaq/K', 'Jane', 'Customer', 'jane@techstart.io', 2, true, 1, 1);
SELECT setval('users_id_seq', 6, true);

-- Groups first (queues need them)
INSERT INTO groups (id, name, valid_id, create_by, change_by) VALUES
(1, 'Support Group', 1, 1, 1),
(2, 'Sales Group', 1, 1, 1),
(3, 'Development Group', 1, 1, 1)
ON CONFLICT (id) DO NOTHING;
SELECT setval('groups_id_seq', 3, true);

-- Queues
INSERT INTO queues (id, name, group_id, valid_id, create_by, change_by) VALUES
(1, 'Support', 1, 1, 1, 1),
(2, 'Sales', 2, 1, 1, 1),
(3, 'Development', 3, 1, 1, 1);
SELECT setval('queues_id_seq', 3, true);

-- Ticket priorities (if not already present)
INSERT INTO ticket_priorities (id, name, valid_id, create_by, change_by) VALUES
(1, 'Low', 1, 1, 1),
(2, 'Normal', 1, 1, 1),
(3, 'High', 1, 1, 1),
(4, 'Critical', 1, 1, 1)
ON CONFLICT (id) DO NOTHING;
SELECT setval('ticket_priorities_id_seq', 4, true);

-- Ticket states (if not already present)
INSERT INTO ticket_states (id, name, type_id, valid_id, create_by, change_by) VALUES
(1, 'new', 1, 1, 1, 1),
(2, 'open', 2, 1, 1, 1),
(3, 'closed', 3, 1, 1, 1),
(4, 'pending', 4, 1, 1, 1)
ON CONFLICT (id) DO NOTHING;
SELECT setval('ticket_states_id_seq', 4, true);

-- Sample tickets
INSERT INTO tickets (ticket_number, title, queue_id, user_id, customer_id, ticket_state_id, ticket_priority_id, create_by, change_by) VALUES
('2025081900001', 'Cannot login to application', 1, 3, 5, 1, 2, 1, 1),
('2025081900002', 'Request for new feature', 3, 4, 6, 2, 1, 1, 1),
('2025081900003', 'System is running slow', 1, 3, 5, 2, 3, 1, 1),
('2025081900004', 'Database backup failed', 1, 4, 5, 1, 4, 1, 1),
('2025081900005', 'Need help with API integration', 3, 3, 6, 3, 2, 1, 1);

-- Sample articles (ticket messages)
INSERT INTO articles (ticket_id, article_type_id, sender_type_id, subject, body, create_by, change_by) VALUES
(1, 1, 3, 'Cannot login to application', 'I am unable to login to the application. Getting error: Invalid credentials', 5, 5),
(1, 2, 1, 'Re: Cannot login to application', 'We are looking into this issue. Can you please try resetting your password?', 3, 3),
(2, 1, 3, 'Request for new feature', 'We would like to have a dashboard showing real-time metrics', 6, 6),
(3, 1, 3, 'System is running slow', 'The system has been very slow since this morning. Pages take 30+ seconds to load', 5, 5),
(4, 1, 1, 'Database backup failed', 'Automated backup failed with error: Insufficient disk space', 4, 4),
(5, 1, 3, 'Need help with API integration', 'We are trying to integrate with your API but getting 401 errors', 6, 6);

-- Ticket categories
INSERT INTO ticket_categories (id, name, description, created_by, updated_by) VALUES
(1, 'Incident', 'Service disruptions and outages', 1, 1),
(2, 'Service Request', 'Standard service requests', 1, 1),
(3, 'Problem', 'Root cause analysis needed', 1, 1)
ON CONFLICT (id) DO NOTHING;
SELECT setval('ticket_categories_id_seq', 3, true);

-- Knowledge base articles
INSERT INTO system_config (name, value, create_by, change_by) VALUES
('KB::Article::Password::Reset::Title', 'How to Reset Your Password', 1, 1),
('KB::Article::Password::Reset::Body', 'To reset your password: 1. Click Forgot Password 2. Enter your email 3. Check your email 4. Follow the reset link', 1, 1),
('KB::Article::API::Guide::Title', 'API Integration Guide', 1, 1),
('KB::Article::API::Guide::Body', 'Our API uses OAuth 2.0 for authentication. See documentation at /api/docs', 1, 1);

-- Email templates
INSERT INTO email_templates (template_name, subject_template, body_template, template_type, created_by, updated_by) VALUES
('Ticket Created', 'Ticket #[TICKET_NUMBER] has been created', 'Your ticket has been created and will be processed soon.', 'ticket_create', 1, 1),
('Ticket Updated', 'Update on Ticket #[TICKET_NUMBER]', 'Your ticket has been updated. Please check the portal for details.', 'ticket_update', 1, 1);

DO $$
DECLARE
    ticket_count INTEGER;
    user_count INTEGER;
    org_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO ticket_count FROM tickets;
    SELECT COUNT(*) INTO user_count FROM users;
    SELECT COUNT(*) INTO org_count FROM organizations;
    
    RAISE NOTICE 'Test Data Migration Complete!';
    RAISE NOTICE '  - % Organizations', org_count;
    RAISE NOTICE '  - % Users', user_count;
    RAISE NOTICE '  - % Tickets', ticket_count;
END $$;