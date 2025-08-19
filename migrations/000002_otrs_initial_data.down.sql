-- Remove initial data
DELETE FROM queue;
DELETE FROM ticket_history_type;
DELETE FROM article_sender_type;
DELETE FROM ticket_lock_type;
DELETE FROM ticket_type;
DELETE FROM ticket_priority;
DELETE FROM ticket_state;
DELETE FROM ticket_state_type;
DELETE FROM user_groups WHERE user_id = 1;
DELETE FROM groups WHERE id IN (1, 2);
DELETE FROM users WHERE id = 1;