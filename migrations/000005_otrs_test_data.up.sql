-- OTRS Test Data
-- This creates comprehensive test data that matches OTRS production patterns

-- ============================================
-- Test Users (Agents)
-- ============================================
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_by, change_by) VALUES
(2, 'jdoe', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'John', 'Doe', 1, 1, 1),
(3, 'jsmith', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Ms.', 'Jane', 'Smith', 1, 1, 1),
(4, 'bob', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', NULL, 'Bob', 'Johnson', 1, 1, 1),
(5, 'alice', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Dr.', 'Alice', 'Williams', 1, 1, 1),
(6, 'charlie', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', NULL, 'Charlie', 'Brown', 1, 1, 1),
(7, 'diana', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Ms.', 'Diana', 'Garcia', 1, 1, 1),
(8, 'edward', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'Edward', 'Martinez', 1, 1, 1),
(9, 'fiona', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mrs.', 'Fiona', 'Anderson', 1, 1, 1),
(10, 'george', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'George', 'Taylor', 1, 1, 1),
(11, 'helen', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Dr.', 'Helen', 'Thomas', 1, 1, 1),
(12, 'ivan', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'Ivan', 'Rodriguez', 2, 1, 1), -- Inactive user
(13, 'julia', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Ms.', 'Julia', 'Chen', 1, 1, 1)
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- Additional Groups
-- ============================================
INSERT INTO groups (id, name, comments, valid_id, create_by, change_by) VALUES
(3, 'Support', 'First level support team', 1, 1, 1),
(4, 'Sales', 'Sales team', 1, 1, 1),
(5, 'IT', 'IT department', 1, 1, 1),
(6, 'Management', 'Management group', 1, 1, 1),
(7, 'Quality', 'Quality assurance team', 1, 1, 1)
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- User Group Assignments
-- ============================================
INSERT INTO group_user (user_id, group_id, permission_key, permission_value, create_by, change_by) VALUES
-- Admin has access to all groups
(1, 1, 'rw', 1, 1, 1),
(1, 2, 'rw', 1, 1, 1),
(1, 3, 'rw', 1, 1, 1),
(1, 4, 'rw', 1, 1, 1),
(1, 5, 'rw', 1, 1, 1),
(1, 6, 'rw', 1, 1, 1),
-- John Doe - Support lead
(2, 3, 'rw', 1, 1, 1),
(2, 1, 'ro', 1, 1, 1),
-- Jane Smith - Sales lead
(3, 4, 'rw', 1, 1, 1),
(3, 1, 'ro', 1, 1, 1),
-- Bob - IT
(4, 5, 'rw', 1, 1, 1),
(4, 3, 'ro', 1, 1, 1),
-- Alice - Management
(5, 6, 'rw', 1, 1, 1),
(5, 1, 'ro', 1, 1, 1),
(5, 3, 'ro', 1, 1, 1),
(5, 4, 'ro', 1, 1, 1),
-- Charlie - Support
(6, 3, 'rw', 1, 1, 1),
-- Diana - Quality
(7, 7, 'rw', 1, 1, 1),
(7, 3, 'ro', 1, 1, 1),
-- Edward - Support & IT
(8, 3, 'rw', 1, 1, 1),
(8, 5, 'rw', 1, 1, 1),
-- Fiona - Sales
(9, 4, 'rw', 1, 1, 1),
-- George - Support
(10, 3, 'rw', 1, 1, 1),
-- Helen - Management & Quality
(11, 6, 'rw', 1, 1, 1),
(11, 7, 'rw', 1, 1, 1)
ON CONFLICT (user_id, group_id, permission_key) DO NOTHING;

-- ============================================
-- Additional Queues
-- ============================================
INSERT INTO queue (id, name, group_id, unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id, create_by, change_by) VALUES
(5, 'Support', 3, 900, 1, 0, 'General support queue', 1, 1, 1),
(6, 'Sales', 4, 0, 1, 0, 'Sales inquiries', 1, 1, 1),
(7, 'IT Helpdesk', 5, 1800, 1, 1, 'IT support requests', 1, 1, 1),
(8, 'Billing', 4, 0, 1, 0, 'Billing and invoicing', 1, 1, 1),
(9, 'Complaints', 6, 3600, 1, 1, 'Customer complaints', 1, 1, 1),
(10, 'Development', 5, 0, 1, 0, 'Development team queue', 1, 1, 1),
(11, 'Quality', 7, 0, 1, 0, 'Quality assurance', 1, 1, 1),
(12, 'Archive', 1, 0, 3, 0, 'Archived tickets', 2, 1, 1) -- Invalid queue for testing
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- Customer Companies
-- ============================================
INSERT INTO customer_company (customer_id, name, street, zip, city, country, url, comments, valid_id, create_by, change_by) VALUES
('ACME', 'ACME Corporation', '123 Main Street', '10001', 'New York', 'USA', 'https://www.acme.example.com', 'Primary test customer', 1, 1, 1),
('GLOBEX', 'Globex Industries', '456 Industrial Ave', '90001', 'Los Angeles', 'USA', 'https://www.globex.example.com', 'Enterprise customer', 1, 1, 1),
('INITECH', 'Initech Solutions', '789 Tech Park', '94102', 'San Francisco', 'USA', 'https://www.initech.example.com', 'Technology partner', 1, 1, 1),
('UMBRELLA', 'Umbrella Corp', '321 Research Blvd', '02139', 'Cambridge', 'USA', 'https://www.umbrella.example.com', 'Research client', 1, 1, 1),
('WAYNE', 'Wayne Enterprises', '1007 Mountain Drive', '10101', 'Gotham', 'USA', 'https://www.wayne.example.com', 'Premium customer', 1, 1, 1),
('STARK', 'Stark Industries', '10880 Malibu Point', '90265', 'Malibu', 'USA', 'https://www.stark.example.com', 'VIP customer', 1, 1, 1),
('OSCORP', 'Oscorp Industries', '42 Madison Ave', '10010', 'New York', 'USA', 'https://www.oscorp.example.com', 'Scientific research', 1, 1, 1),
('LEXCORP', 'LexCorp', '1000 Lex Tower', '10019', 'Metropolis', 'USA', 'https://www.lexcorp.example.com', 'Enterprise client', 1, 1, 1)
ON CONFLICT (customer_id) DO NOTHING;

-- ============================================
-- Customer Users
-- ============================================
INSERT INTO customer_user (id, login, email, customer_id, pw, title, first_name, last_name, phone, mobile, comments, valid_id, create_by, change_by) VALUES
(1, 'peter.parker', 'peter.parker@acme.example.com', 'ACME', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'Peter', 'Parker', '+1-555-0101', '+1-555-9101', 'Main contact', 1, 1, 1),
(2, 'mary.jane', 'mary.jane@acme.example.com', 'ACME', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Ms.', 'Mary', 'Jane', '+1-555-0102', '+1-555-9102', 'Secondary contact', 1, 1, 1),
(3, 'tony.stark', 'tony.stark@stark.example.com', 'STARK', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'Tony', 'Stark', '+1-555-0201', '+1-555-9201', 'CEO', 1, 1, 1),
(4, 'pepper.potts', 'pepper.potts@stark.example.com', 'STARK', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Ms.', 'Pepper', 'Potts', '+1-555-0202', '+1-555-9202', 'COO', 1, 1, 1),
(5, 'bruce.wayne', 'bruce.wayne@wayne.example.com', 'WAYNE', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'Bruce', 'Wayne', '+1-555-0301', '+1-555-9301', 'Owner', 1, 1, 1),
(6, 'alfred.pennyworth', 'alfred@wayne.example.com', 'WAYNE', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'Alfred', 'Pennyworth', '+1-555-0302', NULL, 'Butler/Admin', 1, 1, 1),
(7, 'norman.osborn', 'norman@oscorp.example.com', 'OSCORP', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Dr.', 'Norman', 'Osborn', '+1-555-0401', '+1-555-9401', 'CEO', 1, 1, 1),
(8, 'lex.luthor', 'lex@lexcorp.example.com', 'LEXCORP', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'Lex', 'Luthor', '+1-555-0501', '+1-555-9501', 'President', 1, 1, 1),
(9, 'john.globex', 'john@globex.example.com', 'GLOBEX', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Mr.', 'John', 'Customer', '+1-555-0601', NULL, 'Regular customer', 1, 1, 1),
(10, 'jane.initech', 'jane@initech.example.com', 'INITECH', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Ms.', 'Jane', 'Customer', '+1-555-0701', NULL, 'Support contact', 1, 1, 1),
(11, 'william.wesker', 'wesker@umbrella.example.com', 'UMBRELLA', '$2a$10$P/AuG3V7hK3K0kVu1kZjPu6W5VGdGP/IYU8uZ8Rv0XqKX.8XfVEvu', 'Dr.', 'William', 'Wesker', '+1-555-0801', '+1-555-9801', 'Research lead', 2, 1, 1) -- Inactive customer
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- Sample Tickets
-- ============================================
INSERT INTO ticket (
    id, tn, title, queue_id, ticket_lock_id, type_id,
    user_id, responsible_user_id, ticket_priority_id, ticket_state_id,
    customer_id, customer_user_id, create_time_unix, create_by, change_by
) VALUES
(1, '2024010000001', 'Cannot access email', 7, 1, 2, 4, 4, 3, 2, 'ACME', 'peter.parker@acme.example.com', 1704067200, 1, 1),
(2, '2024010000002', 'Request for new feature', 10, 1, 3, 2, 5, 2, 1, 'STARK', 'tony.stark@stark.example.com', 1704070800, 1, 1),
(3, '2024010000003', 'Billing discrepancy', 8, 2, 4, 9, 3, 4, 2, 'WAYNE', 'bruce.wayne@wayne.example.com', 1704074400, 1, 1),
(4, '2024010000004', 'System performance issues', 7, 1, 2, 4, 8, 5, 2, 'GLOBEX', 'john@globex.example.com', 1704078000, 1, 1),
(5, '2024010000005', 'Product inquiry', 6, 1, 3, 3, 9, 2, 1, 'INITECH', 'jane@initech.example.com', 1704081600, 1, 1),
(6, '2024010000006', 'Password reset request', 5, 1, 3, 6, 10, 3, 3, 'OSCORP', 'norman@oscorp.example.com', 1704085200, 1, 1),
(7, '2024010000007', 'Network connectivity problem', 7, 2, 2, 8, 4, 4, 2, 'LEXCORP', 'lex@lexcorp.example.com', 1704088800, 1, 1),
(8, '2024010000008', 'Software license renewal', 6, 1, 3, 3, 9, 3, 5, 'UMBRELLA', 'wesker@umbrella.example.com', 1704092400, 1, 1),
(9, '2024010000009', 'Database backup failure', 7, 1, 2, 4, 8, 5, 2, 'STARK', 'pepper.potts@stark.example.com', 1704096000, 1, 1),
(10, '2024010000010', 'Customer complaint', 9, 1, 4, 5, 11, 4, 2, 'ACME', 'mary.jane@acme.example.com', 1704099600, 1, 1),
(11, '2024010000011', 'Training request', 5, 1, 3, 2, 6, 2, 1, 'WAYNE', 'alfred@wayne.example.com', 1704103200, 1, 1),
(12, '2024010000012', 'Security incident', 7, 2, 2, 8, 4, 5, 2, 'OSCORP', 'norman@oscorp.example.com', 1704106800, 1, 1),
(13, '2024010000013', 'Contract renewal', 6, 1, 3, 3, 9, 3, 5, 'GLOBEX', 'john@globex.example.com', 1704110400, 1, 1),
(14, '2024010000014', 'API integration issue', 10, 1, 4, 4, 8, 4, 2, 'INITECH', 'jane@initech.example.com', 1704114000, 1, 1),
(15, '2024010000015', 'Quality control failure', 11, 1, 4, 7, 11, 5, 2, 'STARK', 'tony.stark@stark.example.com', 1704117600, 1, 1),
(16, '2024010000016', 'Printer not working', 5, 1, 2, 10, 6, 1, 3, 'ACME', 'peter.parker@acme.example.com', 1704121200, 1, 1),
(17, '2024010000017', 'Website is down', 7, 2, 2, 4, 8, 5, 2, 'WAYNE', 'bruce.wayne@wayne.example.com', 1704124800, 1, 1),
(18, '2024010000018', 'Invoice dispute', 8, 1, 4, 9, 3, 4, 2, 'LEXCORP', 'lex@lexcorp.example.com', 1704128400, 1, 1),
(19, '2024010000019', 'New employee onboarding', 5, 1, 3, 2, 10, 2, 1, 'UMBRELLA', 'wesker@umbrella.example.com', 1704132000, 1, 1),
(20, '2024010000020', 'System upgrade request', 10, 1, 5, 4, 8, 3, 6, 'STARK', 'pepper.potts@stark.example.com', 1704135600, 1, 1)
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- Sample Ticket Articles
-- ============================================
INSERT INTO article (
    id, ticket_id, article_sender_type_id, communication_channel_id,
    is_visible_for_customer, create_by, change_by
) VALUES
(1, 1, 3, 1, 1, 1, 1),
(2, 1, 1, 1, 1, 4, 4),
(3, 2, 3, 1, 1, 1, 1),
(4, 3, 3, 1, 1, 1, 1),
(5, 3, 1, 1, 1, 9, 9),
(6, 4, 3, 1, 1, 1, 1),
(7, 5, 3, 1, 1, 1, 1),
(8, 6, 3, 1, 1, 1, 1),
(9, 6, 1, 1, 1, 6, 6),
(10, 6, 2, 1, 0, 1, 1)
ON CONFLICT (id) DO NOTHING;

-- Article content in article_data_mime table
INSERT INTO article_data_mime (
    id, article_id, a_from, a_to, a_subject, a_body,
    incoming_time, create_by, change_by
) VALUES
(1, 1, 'peter.parker@acme.example.com', 'support@gotrs.example.com', 'Cannot access email',
    'Hello, I cannot access my company email since this morning. Please help!'::bytea, 1704067200, 1, 1),
(2, 2, 'support@gotrs.example.com', 'peter.parker@acme.example.com', 'RE: Cannot access email',
    'Hi Peter, We are looking into this issue. Can you please provide your username?'::bytea, 1704067800, 4, 4),
(3, 3, 'tony.stark@stark.example.com', 'dev@gotrs.example.com', 'Request for new feature',
    'We need an AI-powered ticket classification system. Money is no object.'::bytea, 1704070800, 1, 1),
(4, 4, 'bruce.wayne@wayne.example.com', 'billing@gotrs.example.com', 'Billing discrepancy',
    'There seems to be an error in our last invoice. The amount is incorrect.'::bytea, 1704074400, 1, 1),
(5, 5, 'billing@gotrs.example.com', 'bruce.wayne@wayne.example.com', 'RE: Billing discrepancy',
    'Mr. Wayne, We are reviewing your account and will get back to you shortly.'::bytea, 1704075000, 9, 9),
(6, 6, 'john@globex.example.com', 'it@gotrs.example.com', 'System performance issues',
    'The system has been very slow for the past week. All users are affected.'::bytea, 1704078000, 1, 1),
(7, 7, 'jane@initech.example.com', 'sales@gotrs.example.com', 'Product inquiry',
    'We are interested in your enterprise package. Please send more information.'::bytea, 1704081600, 1, 1),
(8, 8, 'norman@oscorp.example.com', 'support@gotrs.example.com', 'Password reset request',
    'I forgot my password and need it reset immediately.'::bytea, 1704085200, 1, 1),
(9, 9, 'support@gotrs.example.com', 'norman@oscorp.example.com', 'RE: Password reset request',
    'Password has been reset. Please check your email for the temporary password.'::bytea, 1704085800, 6, 6),
(10, 10, 'system', 'system', 'Password reset completed',
    'Password was successfully reset for user norman@oscorp.example.com'::bytea, 1704086400, 1, 1)
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- Sample Ticket History
-- ============================================
INSERT INTO ticket_history (
    id, name, history_type_id, ticket_id, article_id,
    type_id, queue_id, owner_id, priority_id, state_id,
    create_by, change_by
) VALUES
(1, 'New ticket created', 1, 1, 1, 2, 7, 4, 3, 1, 1, 1),
(2, 'State changed to open', 2, 1, NULL, 2, 7, 4, 3, 2, 4, 4),
(3, 'New ticket created', 1, 2, 3, 3, 10, 2, 2, 1, 1, 1),
(4, 'New ticket created', 1, 3, 4, 4, 8, 9, 4, 1, 1, 1),
(5, 'Ticket locked', 12, 3, NULL, 4, 8, 9, 4, 1, 3, 3),
(6, 'State changed to open', 2, 3, NULL, 4, 8, 9, 4, 2, 3, 3),
(7, 'New ticket created', 1, 4, 6, 2, 7, 4, 5, 1, 1, 1),
(8, 'Priority changed to very high', 3, 4, NULL, 2, 7, 4, 5, 2, 4, 4),
(9, 'New ticket created', 1, 6, 8, 3, 5, 6, 3, 1, 1, 1),
(10, 'State changed to closed successful', 2, 6, 10, 3, 5, 6, 3, 3, 6, 6)
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- System Configuration Settings
-- ============================================
INSERT INTO system_data (data_key, data_value, create_by, change_by) VALUES
('SystemID', '10', 1, 1),
('FQDN', 'gotrs.example.com', 1, 1),
('Organization', 'GOTRS Test Company', 1, 1),
('AdminEmail', 'admin@gotrs.example.com', 1, 1),
('NotificationSenderName', 'GOTRS System', 1, 1),
('NotificationSenderEmail', 'noreply@gotrs.example.com', 1, 1),
('TicketNumberGenerator', 'DateChecksum', 1, 1),
('TicketNumberFormat', 'Date', 1, 1),
('MinimumPasswordLength', '8', 1, 1),
('DefaultLanguage', 'en', 1, 1),
('DefaultTheme', 'Standard', 1, 1),
('SessionMaxIdleTime', '7200', 1, 1),
('TicketDefaultQueue', '1', 1, 1),
('TicketDefaultPriority', '3', 1, 1),
('TicketDefaultState', '1', 1, 1),
('ArticleDefaultSenderType', '1', 1, 1)
ON CONFLICT (data_key) DO NOTHING;

-- Reset sequences to proper values
SELECT setval('users_id_seq', GREATEST(13, (SELECT MAX(id) FROM users)), true);
SELECT setval('groups_id_seq', GREATEST(7, (SELECT MAX(id) FROM groups)), true);
SELECT setval('queue_id_seq', GREATEST(12, (SELECT MAX(id) FROM queue)), true);
SELECT setval('customer_user_id_seq', GREATEST(11, (SELECT MAX(id) FROM customer_user)), true);
SELECT setval('ticket_id_seq', GREATEST(20, (SELECT MAX(id) FROM ticket)), true);
SELECT setval('article_id_seq', GREATEST(10, (SELECT MAX(id) FROM article)), true);
SELECT setval('ticket_history_id_seq', GREATEST(10, (SELECT MAX(id) FROM ticket_history)), true);