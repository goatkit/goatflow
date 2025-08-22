-- Rollback: Remove dynamic field tables
DROP TABLE IF EXISTS dynamic_field_obj_id_name CASCADE;
DROP TABLE IF EXISTS dynamic_field_value CASCADE;
DROP TABLE IF EXISTS dynamic_field CASCADE;