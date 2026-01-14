-- Additional integration test fixtures for the MySQL test database
-- Mirrors schema/seed/test_integration.sql but uses MySQL syntax.

START TRANSACTION;

-- Ensure auxiliary test groups exist
INSERT IGNORE INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES
(4, 'support', 'Frontline support team', 1, NOW(), 1, NOW(), 1),
(5, 'testgroup', 'Integration test group', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO queue (
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
) VALUES (
    5,
    'Support',
    1,
    1,
    1,
    1,
    0,
    1,
    0,
    'Primary support queue for agents',
    1,
    NOW(),
    1,
    NOW(),
    1
);

-- Ensure essential queues exist for validation tests
INSERT IGNORE INTO queue (
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
    (
        1,
        'Postmaster',
        1,
        1,
        1,
        1,
        0,
        1,
        0,
        'Inbound email queue',
        1,
        NOW(),
        1,
        NOW(),
        1
    ),
    (
        2,
        'Raw',
        1,
        2,
        1,
        1,
        0,
        1,
        0,
        'Unprocessed tickets',
        1,
        NOW(),
        1,
        NOW(),
        1
    ),
    (
        3,
        'Junk',
        1,
        3,
        1,
        1,
        0,
        1,
        0,
        'Spam quarantine queue',
        1,
        NOW(),
        1,
        NOW(),
        1
    ),
    (
        4,
        'Misc',
        1,
        4,
        1,
        1,
        0,
        1,
        0,
        'Miscellaneous work queue',
        1,
        NOW(),
        1,
        NOW(),
        1
    ),
    (
        6,
        'OBC',
        1,
        1,
        1,
        1,
        0,
        1,
        0,
        'Outbound communication queue',
        1,
        NOW(),
        1,
        NOW(),
        1
    );

-- Seed test roles for admin role permissions tests
INSERT IGNORE INTO roles (id, name, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES
    (1, 'Admin', 'System administrators with full access', 1, NOW(), 1, NOW(), 1),
    (2, 'Agent', 'Standard support agents', 1, NOW(), 1, NOW(), 1),
    (3, 'Supervisor', 'Team supervisors with elevated permissions', 1, NOW(), 1, NOW(), 1);

-- Seed a deterministic test user referenced by admin integration tests
INSERT IGNORE INTO users (id, login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
VALUES (15, 'testuser', '8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918', 'Test', 'Agent', 1, NOW(), 1, NOW(), 1);

-- Grant baseline group memberships to the seeded user if missing
INSERT IGNORE INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
VALUES
    (15, 1, 'rw', NOW(), 1, NOW(), 1),
    (15, 2, 'rw', NOW(), 1, NOW(), 1);

-- Ensure customer entities referenced by tickets exist
INSERT IGNORE INTO customer_company (customer_id, name, street, zip, city, country, url, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES
    ('COMP1', 'Test Company Alpha', NULL, NULL, NULL, NULL, NULL, 'Test fixture company', 1, NOW(), 1, NOW(), 1),
    ('COMP2', 'Test Company Beta', NULL, NULL, NULL, NULL, NULL, 'Test fixture company', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO customer_user (login, email, customer_id, pw, title, first_name, last_name, phone, fax, mobile, street, zip, city, country, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES
    ('john.customer', 'john.customer@example.test', 'COMP1', NULL, NULL, 'Test', 'Customer Alpha', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'Seeded integration user', 1, NOW(), 1, NOW(), 1),
    ('jane.customer', 'jane.customer@example.test', 'COMP2', NULL, NULL, 'Test', 'Customer Beta', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'Seeded integration user', 1, NOW(), 1, NOW(), 1);

-- Update Support queue to use the 'support' group (id=4) so customers can have unique preferred queues
UPDATE queue SET group_id = 4 WHERE id = 5 AND name = 'Support';

-- Give customers preferred queues via group_customer (for queue preference tests)
-- COMP1 (john.customer) gets Support queue via support group (id=4) with rw permission
-- COMP2 (jane.customer) gets Misc queue via users group (id=1) with ro permission
INSERT IGNORE INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context, create_time, create_by, change_time, change_by)
VALUES
    ('COMP1', 4, 'rw', 1, '', NOW(), 1, NOW(), 1),
    ('COMP2', 1, 'ro', 1, '', NOW(), 1, NOW(), 1);

-- Seed queue statistics: Raw queue (2 tickets) and Junk queue (1 ticket)
-- Also seed ticket ID 1 explicitly for API tests that expect it
INSERT IGNORE INTO ticket (
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
    (1, 'TEST-0001', 'Test ticket for API tests', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1),
    (2, 'TEST-0002', 'Another test ticket', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO ticket (
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
    ('RAW-0001', 'First Raw queue ticket', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1),
    ('RAW-0002', 'Second Raw queue ticket', 2, 1, 1, NULL, NULL, 1, 1, 3, 2, 'COMP1', 'john.customer', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1),
    ('JUNK-0001', 'Junk queue ticket', 3, 1, 1, NULL, NULL, 1, 1, 2, 5, 'COMP2', 'jane.customer', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1);

-- Seed dynamic fields for integration tests (various types for Ticket and Article objects)
INSERT IGNORE INTO dynamic_field (id, internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
VALUES
    (1, 0, 'TestTextField', 'Test Text Field', 1, 'Text', 'Ticket', '---\nDefaultValue: \n', 1, NOW(), 1, NOW(), 1),
    (2, 0, 'TestDropdown', 'Test Dropdown', 2, 'Dropdown', 'Ticket', '---\nPossibleValues:\n  option1: Option 1\n  option2: Option 2\n  option3: Option 3\nDefaultValue: option1\n', 1, NOW(), 1, NOW(), 1),
    (3, 0, 'TestCheckbox', 'Test Checkbox', 3, 'Checkbox', 'Ticket', '---\nDefaultValue: 0\n', 1, NOW(), 1, NOW(), 1),
    (4, 0, 'TestDate', 'Test Date Field', 4, 'Date', 'Ticket', '---\nDefaultValue: \nYearsInFuture: 5\nYearsInPast: 5\n', 1, NOW(), 1, NOW(), 1),
    (5, 0, 'TestTextArea', 'Test Text Area', 5, 'TextArea', 'Ticket', '---\nDefaultValue: \nRows: 5\nCols: 60\n', 1, NOW(), 1, NOW(), 1),
    (6, 0, 'ArticleNote', 'Article Note Field', 1, 'Text', 'Article', '---\nDefaultValue: \n', 1, NOW(), 1, NOW(), 1),
    (7, 0, 'ArticleCategory', 'Article Category', 2, 'Dropdown', 'Article', '---\nPossibleValues:\n  internal: Internal\n  external: External\n  escalation: Escalation\nDefaultValue: internal\n', 1, NOW(), 1, NOW(), 1);

COMMIT;
