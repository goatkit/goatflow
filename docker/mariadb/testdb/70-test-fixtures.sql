-- Test fixtures for E2E tests
-- This file is loaded during test database initialization

-- Insert test templates for admin_templates_test.go
INSERT INTO standard_template (name, text, content_type, template_type, comments, valid_id, create_time, create_by, change_time, change_by)
VALUES
    ('Test Answer Template', '<p>Thank you for contacting support. Your issue has been resolved.</p>', 'text/html', 'Answer', 'E2E test fixture', 1, NOW(), 1, NOW(), 1),
    ('Test Note Template', '<p>Internal note: Follow-up required on this ticket.</p>', 'text/html', 'Note', 'E2E test fixture', 1, NOW(), 1, NOW(), 1),
    ('Test Forward Template', '<p>Forwarding this ticket to the appropriate department.</p>', 'text/html', 'Forward', 'E2E test fixture', 1, NOW(), 1, NOW(), 1),
    ('Test Create Template', '<p>New ticket created from template.</p>', 'text/html', 'Create', 'E2E test fixture', 1, NOW(), 1, NOW(), 1),
    ('Plain Text Template', 'This is a plain text response without HTML formatting.', 'text/plain', 'Answer', 'E2E test fixture - plain text', 1, NOW(), 1, NOW(), 1)
ON DUPLICATE KEY UPDATE change_time = NOW();

-- Insert a test ticket for note_alignment_test.go if not exists
-- (The test creates its own ticket via API, but having a base ticket helps)
INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time, escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time, create_time, create_by, change_time, change_by)
SELECT '20251230000001', 'E2E Test Ticket', 1, 1, 1, 1, 1, 3, 1, '', '', 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM ticket WHERE tn = '20251230000001');
