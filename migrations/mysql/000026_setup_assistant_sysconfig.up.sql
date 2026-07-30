-- Seed the setup.assistant.completed sysconfig flag used by the Setup Assistant
-- to gate the first-run wizard redirect. Reuses the existing sysconfig_default
-- table; no schema change. The upsert is a deliberate no-op so re-applying this
-- migration never clobbers an admin's completion state.
SET FOREIGN_KEY_CHECKS = 0;
SET @now := NOW();
SET @author := 1;
SET @has_sysconfig_default := (
    SELECT COUNT(*)
        FROM information_schema.tables
     WHERE table_schema = DATABASE()
         AND table_name = 'sysconfig_default'
);
SET @has_sysconfig_default := IFNULL(@has_sysconfig_default, 0);

INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'setup.assistant.completed',
    'Whether the first-run setup wizard has been completed.',
    'Core::SetupAssistant',
    0, 0, 0, 1,
    0, 0, 0, NULL,
    '{"type":"boolean","default":false}',
    '{"type":"boolean","default":false}',
    'SetupAssistant.xml',
    'false',
    0,
    '', NULL, NULL,
    @now, @author, @now, @author
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE name = name;

SET FOREIGN_KEY_CHECKS = 1;
