-- Rollback: Remove workflow and process management tables
DROP TABLE IF EXISTS scheduler_recurrent_task CASCADE;
DROP TABLE IF EXISTS scheduler_future_task CASCADE;
DROP TABLE IF EXISTS scheduler_task CASCADE;
DROP TABLE IF EXISTS process_id CASCADE;
DROP TABLE IF EXISTS pm_entity_sync CASCADE;
DROP TABLE IF EXISTS pm_transition_action CASCADE;
DROP TABLE IF EXISTS pm_transition CASCADE;
DROP TABLE IF EXISTS pm_activity_dialog CASCADE;
DROP TABLE IF EXISTS pm_activity CASCADE;
DROP TABLE IF EXISTS pm_process CASCADE;
DROP TABLE IF EXISTS generic_agent_jobs CASCADE;