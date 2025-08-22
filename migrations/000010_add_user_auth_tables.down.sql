-- Rollback: Remove user and authentication tables
DROP TABLE IF EXISTS group_user CASCADE;
DROP TABLE IF EXISTS customer_preferences CASCADE;
DROP TABLE IF EXISTS personal_services CASCADE;
DROP TABLE IF EXISTS personal_queues CASCADE;
DROP TABLE IF EXISTS user_preferences CASCADE;
DROP TABLE IF EXISTS group_role CASCADE;
DROP TABLE IF EXISTS role_user CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS valid CASCADE;