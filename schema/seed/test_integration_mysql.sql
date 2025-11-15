-- Additional integration test fixtures for the MySQL test database
-- Mirrors schema/seed/test_integration.sql but uses MySQL syntax.

START TRANSACTION;

-- Ensure auxiliary test groups exist
INSERT IGNORE INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES (4, 'testgroup', 'Integration test group', 1, NOW(), 1, NOW(), 1);

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

-- Seed queue statistics: Raw queue (2 tickets) and Junk queue (1 ticket)
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

COMMIT;
