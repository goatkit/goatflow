-- Quick reset of test database to canonical state
-- Faster than full migration down/up since it just cleans test data
-- Canonical queues from migrations: Postmaster(1), Raw(2), Junk(3), Misc(4)

SET FOREIGN_KEY_CHECKS = 0;

-- Clean ALL tickets first (we'll recreate canonical test data)
DELETE FROM ticket_history;
DELETE FROM article_data_mime;
DELETE FROM article_data_mime_attachment;
DELETE FROM article;
DELETE FROM ticket;

-- Clean test states (preserve IDs 1-5 which are canonical OTRS states)
DELETE FROM ticket_state WHERE id > 5;

-- Clean test types (preserve IDs 1-5 which are canonical types)
DELETE FROM ticket_type WHERE id > 5;

-- Clean test queues - we will recreate them
DELETE FROM queue WHERE id > 4;

-- Clean test groups (preserve IDs 1-4, id=4 is Support for tests)
DELETE FROM group_user WHERE group_id > 4;
DELETE FROM groups WHERE id > 4;

-- Add Support group if not exists (id=4)
INSERT INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by)
SELECT 4, 'Support', 'Support group for automated tests', 1, NOW(), 1, NOW(), 1
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE id = 4);

-- Clean test users (preserve ID 1 root, ID 2 admin, ID 15 testuser)
DELETE FROM group_user WHERE user_id > 15;
DELETE FROM users WHERE id > 15 OR (id > 2 AND id < 15);

-- Ensure admin user (id=2) exists for tests
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
SELECT 2, 'admin@example.com', '', '', 'Test', 'Admin', 1, NOW(), 1, NOW(), 1
WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = 2);

-- Ensure testuser (id=15) exists for tests
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
SELECT 15, 'testuser', '', '', 'Test', 'Agent', 1, NOW(), 1, NOW(), 1
WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = 15);

-- Ensure root user (id=1) is in admin group
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
SELECT 1, 2, 'rw', NOW(), 1, NOW(), 1
WHERE NOT EXISTS (SELECT 1 FROM group_user WHERE user_id = 1 AND group_id = 2);

-- Ensure testuser (id=15) is in users group (id=1)
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
SELECT 15, 1, 'rw', NOW(), 1, NOW(), 1
WHERE NOT EXISTS (SELECT 1 FROM group_user WHERE user_id = 15 AND group_id = 1);

-- Clean test dynamic fields (ignore if table doesn't exist)
DELETE FROM dynamic_field_value WHERE id > 0;
DELETE FROM dynamic_field WHERE id > 10;

-- Clean test customer users  
DELETE FROM customer_user WHERE login NOT IN ('test@example.com', 'customer@goatflow.local');

-- Restore canonical state names (OTRS standard)
UPDATE ticket_state SET name = 'new' WHERE id = 1;
UPDATE ticket_state SET name = 'open' WHERE id = 2;
UPDATE ticket_state SET name = 'pending reminder' WHERE id = 3;
UPDATE ticket_state SET name = 'closed successful' WHERE id = 4;
UPDATE ticket_state SET name = 'closed unsuccessful' WHERE id = 5;

-- Restore canonical type names
UPDATE ticket_type SET name = 'Unclassified' WHERE id = 1;
UPDATE ticket_type SET name = 'Incident' WHERE id = 2;
UPDATE ticket_type SET name = 'Service Request' WHERE id = 3;
UPDATE ticket_type SET name = 'Problem' WHERE id = 4;
UPDATE ticket_type SET name = 'Change Request' WHERE id = 5;

-- Restore canonical priority names
UPDATE ticket_priority SET name = '1 very low' WHERE id = 1;
UPDATE ticket_priority SET name = '2 low' WHERE id = 2;
UPDATE ticket_priority SET name = '3 normal' WHERE id = 3;
UPDATE ticket_priority SET name = '4 high' WHERE id = 4;
UPDATE ticket_priority SET name = '5 very high' WHERE id = 5;

-- Restore canonical queue names (from migrations: Postmaster, Raw, Junk, Misc)
UPDATE queue SET name = 'Postmaster' WHERE id = 1;
UPDATE queue SET name = 'Raw' WHERE id = 2;
UPDATE queue SET name = 'Junk' WHERE id = 3;
UPDATE queue SET name = 'Misc' WHERE id = 4;

-- Seed canonical test tickets
-- Raw queue (id=2) should have exactly 2 tickets for tests
INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
    responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
    timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
    escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
VALUES 
    (1, 'RAW-0001', 'First Raw queue ticket', 2, 1, 1, 1, 1, 3, 2, 'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1),
    (2, 'RAW-0002', 'Second Raw queue ticket', 2, 1, 1, 1, 1, 3, 2, 'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1);

-- Junk queue (id=3) should have exactly 1 ticket for tests
INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
    responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
    timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
    escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
VALUES 
    (3, 'JUNK-0001', 'Junk queue ticket', 3, 1, 1, 1, 1, 3, 2, 'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1);

-- Create test ticket 123 for attachment tests
INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
    responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
    timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
    escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
VALUES (123, 'TEST-0123', 'Test Ticket for Attachments', 1, 1, 1, 1, 1, 3, 2,
    'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1);

SET FOREIGN_KEY_CHECKS = 1;
