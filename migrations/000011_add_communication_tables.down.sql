-- Rollback: Remove communication and email tables
DROP TABLE IF EXISTS mail_queue CASCADE;
DROP TABLE IF EXISTS communication_channel CASCADE;
DROP TABLE IF EXISTS postmaster_filter CASCADE;
DROP TABLE IF EXISTS mail_account CASCADE;
DROP TABLE IF EXISTS follow_up_possible CASCADE;
DROP TABLE IF EXISTS salutation CASCADE;
DROP TABLE IF EXISTS signature CASCADE;
DROP TABLE IF EXISTS system_address CASCADE;
DROP TABLE IF EXISTS queue_auto_response CASCADE;
DROP TABLE IF EXISTS auto_response CASCADE;
DROP TABLE IF EXISTS auto_response_type CASCADE;