-- Minimal seed data for development environment
-- This creates just enough data to have a working system

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Admin user placeholder (disabled until password reset). Generate a random hash to avoid static secrets in source.
INSERT INTO users (id, login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'root@localhost', encode(digest(gen_random_bytes(32), 'sha256'), 'hex'), 'System', 'Administrator', 2, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
ON CONFLICT (id) DO NOTHING;

-- Grant admin permissions (group_user has no unique key, so guard explicitly;
-- migration 000002 already grants these on a migrated database)
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
SELECT 1, g.id, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM groups g
WHERE g.id IN (1, 2, 3)
  AND NOT EXISTS (
    SELECT 1 FROM group_user gu
    WHERE gu.user_id = 1 AND gu.group_id = g.id AND gu.permission_key = 'rw'
  );

-- Set sequence starting points
SELECT setval('users_id_seq', COALESCE((SELECT MAX(id) FROM users), 1));
SELECT setval('groups_id_seq', COALESCE((SELECT MAX(id) FROM groups), 3));
SELECT setval('queue_id_seq', COALESCE((SELECT MAX(id) FROM queue), 4));
SELECT setval('ticket_priority_id_seq', COALESCE((SELECT MAX(id) FROM ticket_priority), 5));
SELECT setval('ticket_state_id_seq', COALESCE((SELECT MAX(id) FROM ticket_state), 5));
SELECT setval('ticket_type_id_seq', COALESCE((SELECT MAX(id) FROM ticket_type), 5));

COMMIT;