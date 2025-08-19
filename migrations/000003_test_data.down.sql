-- Remove test data
DELETE FROM ticket_history WHERE ticket_id IN (SELECT id FROM ticket WHERE tn LIKE '202501%');
DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE tn LIKE '202501%'));
DELETE FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE tn LIKE '202501%');
DELETE FROM ticket WHERE tn LIKE '202501%';
DELETE FROM user_groups WHERE user_id IN (SELECT id FROM users WHERE login IN ('agent.smith', 'agent.jones'));
DELETE FROM users WHERE login IN ('agent.smith', 'agent.jones');
DELETE FROM customer_user WHERE login IN ('john.customer', 'jane.customer', 'bob.customer');
DELETE FROM customer_company WHERE customer_id IN ('COMP1', 'COMP2', 'COMP3');