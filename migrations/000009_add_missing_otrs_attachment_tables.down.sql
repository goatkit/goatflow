-- Rollback: Remove OTRS attachment tables
DROP TABLE IF EXISTS queue_standard_template CASCADE;
DROP TABLE IF EXISTS standard_template_attachment CASCADE;
DROP TABLE IF EXISTS standard_template CASCADE;
DROP TABLE IF EXISTS standard_attachment CASCADE;