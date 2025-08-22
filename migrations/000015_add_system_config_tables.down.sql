-- Rollback: Remove system configuration tables
DROP TABLE IF EXISTS system_maintenance CASCADE;
DROP TABLE IF EXISTS sysconfig_deployment_lock CASCADE;
DROP TABLE IF EXISTS sysconfig_deployment CASCADE;
DROP TABLE IF EXISTS sysconfig_modified_version CASCADE;
DROP TABLE IF EXISTS sysconfig_modified CASCADE;
DROP TABLE IF EXISTS sysconfig_default_version CASCADE;
DROP TABLE IF EXISTS sysconfig_default CASCADE;