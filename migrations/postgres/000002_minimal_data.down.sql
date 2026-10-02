-- Remove minimal upstream seed data from PostgreSQL deployments.
-- Mirrors migrations/mysql/000002_minimal_data.down.sql. Every seeded row references
-- users(1) via create_by/change_by while users(1) references valid(2); the final
-- statement deletes users and valid together because non-deferrable FKs are checked
-- at the end of the statement, which breaks the cycle without superuser rights.
BEGIN;

DELETE FROM group_user WHERE user_id = 1 AND group_id IN (1,2,3);
DELETE FROM queue WHERE id IN (1,2,3,4);
DELETE FROM signature WHERE id = 1;
DELETE FROM salutation WHERE id = 1;
DELETE FROM system_address WHERE id IN (1,2,3,4);
DELETE FROM groups WHERE id IN (1,2,3);
DELETE FROM follow_up_possible WHERE id IN (1,2,3);
DELETE FROM communication_channel WHERE id IN (1,2,3,4);
DELETE FROM article_sender_type WHERE id IN (1,2,3);
DELETE FROM ticket_lock_type WHERE id IN (1,2);
DELETE FROM ticket_type WHERE id IN (1,2,3,4,5);
DELETE FROM ticket_priority WHERE id IN (1,2,3,4,5);
DELETE FROM ticket_state WHERE id IN (1,2,3,4,5);
DELETE FROM ticket_state_type WHERE id IN (1,2,3,4,5);
WITH removed_user AS (
    DELETE FROM users WHERE id = 1 RETURNING id
)
DELETE FROM valid WHERE id IN (1,2,3);

COMMIT;
