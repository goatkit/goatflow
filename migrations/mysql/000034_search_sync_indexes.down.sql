-- Reverses 000034_search_sync_indexes.up.sql.
DROP INDEX IF EXISTS ticket_change_time ON ticket;
DROP INDEX IF EXISTS article_change_time ON article;
DROP INDEX IF EXISTS customer_user_change_time ON customer_user;
