-- Rollback test data migration
-- This removes only test data, preserving system data

-- Remove test attachments
DELETE FROM article_attachments WHERE article_id IN (
    SELECT id FROM articles WHERE ticket_id IN (
        SELECT id FROM tickets WHERE tn LIKE '2025%'
    )
);

-- Remove test articles/messages
DELETE FROM articles WHERE ticket_id IN (
    SELECT id FROM tickets WHERE tn LIKE '2025%'
);

-- Remove test tickets
DELETE FROM tickets WHERE tn LIKE '2025%';

-- Remove test email templates
DELETE FROM email_templates WHERE create_by > 1 OR name IN (
    'Ticket Created', 'Ticket Updated', 'Ticket Resolved', 
    'Escalation Notice', 'Customer Survey'
);

-- Remove KB articles from system_config
DELETE FROM system_config WHERE name LIKE 'KB::Article::%';

-- Remove user group assignments for test agents
DELETE FROM user_groups WHERE user_id IN (
    SELECT id FROM users WHERE login LIKE 'agent.%' 
    OR login LIKE 'supervisor.%' 
    OR login LIKE 'manager.%'
);

-- Remove test users (customers and agents)
DELETE FROM users WHERE id > 2;  -- Keep system and admin users

-- Remove test ticket categories
DELETE FROM ticket_categories WHERE name IN (
    'Incident', 'Service Request', 'Problem', 'Change', 'Knowledge'
);

-- Remove test queues (keep the original 4)
DELETE FROM queues WHERE id > 4;

-- Remove test organizations
DELETE FROM organizations;

-- Reset sequences to reasonable values
ALTER SEQUENCE users_id_seq RESTART WITH 100;
ALTER SEQUENCE tickets_id_seq RESTART WITH 1;
ALTER SEQUENCE articles_id_seq RESTART WITH 1;
ALTER SEQUENCE queues_id_seq RESTART WITH 10;
ALTER SEQUENCE organizations_id_seq RESTART WITH 1;

DO $$
BEGIN
    RAISE NOTICE '========================================';
    RAISE NOTICE 'Test Data Rollback Complete!';
    RAISE NOTICE '========================================';
    RAISE NOTICE 'Removed all test data while preserving:';
    RAISE NOTICE '  - System user';
    RAISE NOTICE '  - Admin user';
    RAISE NOTICE '  - Default groups and roles';
    RAISE NOTICE '  - Initial 4 queues';
    RAISE NOTICE '  - System configuration';
    RAISE NOTICE '========================================';
END $$;