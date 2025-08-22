-- Rollback: Remove notification and event tables
DROP TABLE IF EXISTS communication_log_object_entry CASCADE;
DROP TABLE IF EXISTS communication_log_object CASCADE;
DROP TABLE IF EXISTS communication_log_obj_lookup CASCADE;
DROP TABLE IF EXISTS communication_log CASCADE;
DROP TABLE IF EXISTS notification_event_message CASCADE;
DROP TABLE IF EXISTS notification_event_item CASCADE;
DROP TABLE IF EXISTS notification_event CASCADE;