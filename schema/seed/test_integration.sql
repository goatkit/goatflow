-- Additional integration test fixtures for the PostgreSQL test database
-- Applied automatically by the postgres-test container during initialization

BEGIN;

-- Ensure auxiliary test groups exist
INSERT INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES 
(4, 'support', 'Frontline support team', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
(5, 'testgroup', 'Integration test group', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (id) DO NOTHING;

-- Add a Support queue expected by integration tests
INSERT INTO queue (
    id,
    name,
    group_id,
    system_address_id,
    salutation_id,
    signature_id,
    unlock_timeout,
    follow_up_id,
    follow_up_lock,
    comments,
    valid_id,
    create_time,
    create_by,
    change_time,
    change_by
) VALUES
    (5, 'Support', 1, 1, 1, 1, 0, 1, 0, 'Primary support queue for agents', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (6, 'OBC', 1, 1, 1, 1, 0, 1, 0, 'OBC customer queue required by compatibility tests', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (id) DO NOTHING;

-- Seed a deterministic test user referenced by admin integration tests
INSERT INTO users (id, login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
VALUES (15, 'testuser', '8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918', 'Test', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (id) DO NOTHING;

-- Grant baseline group memberships to the seeded user if missing
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
SELECT 15, g.id, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM groups g
WHERE g.name IN ('users', 'admin')
  AND NOT EXISTS (
    SELECT 1 FROM group_user gu
    WHERE gu.user_id = 15 AND gu.group_id = g.id AND gu.permission_key = 'rw'
  );

-- Ensure customer entities referenced by tickets exist
INSERT INTO customer_company (customer_id, name, street, zip, city, country, url, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES
    ('COMP1', 'Test Company Alpha', NULL, NULL, NULL, NULL, NULL, 'Test fixture company', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    ('COMP2', 'Test Company Beta', NULL, NULL, NULL, NULL, NULL, 'Test fixture company', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (customer_id) DO NOTHING;

INSERT INTO customer_user (login, email, customer_id, pw, title, first_name, last_name, phone, fax, mobile, street, zip, city, country, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES
    ('john.customer', 'john.customer@example.test', 'COMP1', NULL, NULL, 'Test', 'Customer Alpha', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'Seeded integration user', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    ('jane.customer', 'jane.customer@example.test', 'COMP2', NULL, NULL, 'Test', 'Customer Beta', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'Seeded integration user', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (login) DO NOTHING;

-- Update Support queue to use the 'support' group (id=4) so customers can have unique preferred queues
UPDATE queue SET group_id = 4 WHERE id = 5 AND name = 'Support';

-- Give customers preferred queues via group_customer (for queue preference tests)
-- COMP1 (john.customer) gets Support queue via support group (id=4) with rw permission
-- COMP2 (jane.customer) gets Misc queue via users group (id=1) with ro permission
INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context, create_time, create_by, change_time, change_by)
VALUES
    ('COMP1', 4, 'rw', 1, '', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    ('COMP2', 1, 'ro', 1, '', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT DO NOTHING;

-- Seed queue statistics: Raw queue (2 tickets) and Junk queue (1 ticket)
-- Also seed ticket IDs 1-2 explicitly for API tests that expect them
INSERT INTO ticket (
    id,
    tn,
    title,
    queue_id,
    ticket_lock_id,
    type_id,
    service_id,
    sla_id,
    user_id,
    responsible_user_id,
    ticket_priority_id,
    ticket_state_id,
    customer_id,
    customer_user_id,
    timeout,
    until_time,
    escalation_time,
    escalation_update_time,
    escalation_response_time,
    escalation_solution_time,
    archive_flag,
    create_time,
    create_by,
    change_time,
    change_by
) VALUES
    (1, 'TEST-0001', 'Test ticket for API tests', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (2, 'TEST-0002', 'Another test ticket', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (id) DO NOTHING;

-- Explicit-ID inserts above do not advance the SERIAL sequence; resync before
-- auto-ID inserts so PG doesn't collide (MySQL bumps AUTO_INCREMENT implicitly).
SELECT setval(pg_get_serial_sequence('ticket', 'id'), GREATEST(1, (SELECT MAX(id) FROM ticket)));

INSERT INTO ticket (
    tn,
    title,
    queue_id,
    ticket_lock_id,
    type_id,
    service_id,
    sla_id,
    user_id,
    responsible_user_id,
    ticket_priority_id,
    ticket_state_id,
    customer_id,
    customer_user_id,
    timeout,
    until_time,
    escalation_time,
    escalation_update_time,
    escalation_response_time,
    escalation_solution_time,
    archive_flag,
    create_time,
    create_by,
    change_time,
    change_by
) VALUES
    ('RAW-0001', 'First Raw queue ticket', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    ('RAW-0002', 'Second Raw queue ticket', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    ('JUNK-0001', 'Junk queue ticket', 3, 1, 1, NULL, NULL, 1, 1, 2, 5, 'COMP2', 'jane.customer', 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (tn) DO NOTHING;

-- Seed dynamic fields for integration tests (various types for Ticket and Article objects)
INSERT INTO dynamic_field (id, internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
VALUES
    (1, 0, 'TestTextField', 'Test Text Field', 1, 'Text', 'Ticket', '---
DefaultValue: 
', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (2, 0, 'TestDropdown', 'Test Dropdown', 2, 'Dropdown', 'Ticket', '---
PossibleValues:
  option1: Option 1
  option2: Option 2
  option3: Option 3
DefaultValue: option1
', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (3, 0, 'TestCheckbox', 'Test Checkbox', 3, 'Checkbox', 'Ticket', '---
DefaultValue: 0
', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (4, 0, 'TestDate', 'Test Date Field', 4, 'Date', 'Ticket', '---
DefaultValue: 
YearsInFuture: 5
YearsInPast: 5
', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (5, 0, 'TestTextArea', 'Test Text Area', 5, 'TextArea', 'Ticket', '---
DefaultValue: 
Rows: 5
Cols: 60
', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (6, 0, 'ArticleNote', 'Article Note Field', 1, 'Text', 'Article', '---
DefaultValue: 
', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),
    (7, 0, 'ArticleCategory', 'Article Category', 2, 'Dropdown', 'Article', '---
PossibleValues:
  internal: Internal
  external: External
  escalation: Escalation
DefaultValue: internal
', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (id) DO NOTHING;

-- Keep sequences aligned with the inserted fixtures
SELECT setval('groups_id_seq', GREATEST((SELECT COALESCE(MAX(id), 1) FROM groups), 4));
SELECT setval('queue_id_seq', GREATEST((SELECT COALESCE(MAX(id), 1) FROM queue), 6));
SELECT setval('users_id_seq', GREATEST((SELECT COALESCE(MAX(id), 1) FROM users), 15));
SELECT setval('dynamic_field_id_seq', GREATEST((SELECT COALESCE(MAX(id), 1) FROM dynamic_field), 7));

COMMIT;
