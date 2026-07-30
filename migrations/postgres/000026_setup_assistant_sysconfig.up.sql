-- Seed the setup.assistant.completed sysconfig flag used by the Setup Assistant
-- to gate the first-run wizard redirect. Reuses the existing sysconfig_default
-- table; no schema change. The upsert is a deliberate no-op so re-applying this
-- migration never clobbers an admin's completion state.
DO $$
DECLARE
    def_exists boolean := to_regclass('sysconfig_default') IS NOT NULL;
    now_ts timestamptz := NOW();
    author int := 1;
BEGIN
    IF NOT def_exists THEN
        RAISE NOTICE 'sysconfig_default missing; skipping SetupAssistant seed';
        RETURN;
    END IF;

    INSERT INTO sysconfig_default (
            name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
            has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
            xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
            exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
            create_time, create_by, change_time, change_by
    ) VALUES (
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
            now_ts, author, now_ts, author
    )
    ON CONFLICT (name) DO NOTHING;
END;
$$;
