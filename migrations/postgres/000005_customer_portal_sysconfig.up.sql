-- Seed customer portal sysconfig entries (no schema change).
DO $$
DECLARE
    def_exists boolean := to_regclass('sysconfig_default') IS NOT NULL;
BEGIN
    IF NOT def_exists THEN
        RAISE NOTICE 'sysconfig_default missing; skipping CustomerPortal seed';
        RETURN;
    END IF;

    WITH cfg AS (
        SELECT 'Frontend::Customer::Portal'::text AS navigation,
                     'CustomerPortal.xml'::text AS xml_file,
                     NOW() AS now_ts,
                     1 AS author
    )
    INSERT INTO sysconfig_default (
            name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
            has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
            xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
            exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
            create_time, create_by, change_time, change_by
    ) SELECT
            'CustomerPortal::Enabled',
            'Allow customers to access the portal and ticket UI.',
            navigation,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"boolean","default":true}',
            '{"type":"boolean","default":true}',
            xml_file,
            'true',
            0,
            '', NULL, NULL,
            now_ts, author, now_ts, author
        FROM cfg
    ON CONFLICT (name) DO UPDATE SET
            description = EXCLUDED.description,
            navigation = EXCLUDED.navigation,
            xml_content_raw = EXCLUDED.xml_content_raw,
            xml_content_parsed = EXCLUDED.xml_content_parsed,
            xml_filename = EXCLUDED.xml_filename,
            effective_value = EXCLUDED.effective_value,
            is_valid = EXCLUDED.is_valid,
            user_modification_possible = EXCLUDED.user_modification_possible,
            user_modification_active = EXCLUDED.user_modification_active,
            change_time = EXCLUDED.change_time,
            change_by = EXCLUDED.change_by;

    INSERT INTO sysconfig_default (
            name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
            has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
            xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
            exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
            create_time, create_by, change_time, change_by
    ) SELECT
            'CustomerPortal::LoginRequired',
            'Require customer authentication before accessing the portal.',
            navigation,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"boolean","default":true}',
            '{"type":"boolean","default":true}',
            xml_file,
            'true',
            0,
            '', NULL, NULL,
            now_ts, author, now_ts, author
        FROM cfg
    ON CONFLICT (name) DO UPDATE SET
            description = EXCLUDED.description,
            navigation = EXCLUDED.navigation,
            xml_content_raw = EXCLUDED.xml_content_raw,
            xml_content_parsed = EXCLUDED.xml_content_parsed,
            xml_filename = EXCLUDED.xml_filename,
            effective_value = EXCLUDED.effective_value,
            is_valid = EXCLUDED.is_valid,
            user_modification_possible = EXCLUDED.user_modification_possible,
            user_modification_active = EXCLUDED.user_modification_active,
            change_time = EXCLUDED.change_time,
            change_by = EXCLUDED.change_by;

    INSERT INTO sysconfig_default (
            name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
            has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
            xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
            exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
            create_time, create_by, change_time, change_by
    ) SELECT
            'CustomerPortal::Title',
            'Portal title shown in header and HTML title.',
            navigation,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"string","default":"Customer Portal"}',
            '{"type":"string","default":"Customer Portal"}',
            xml_file,
            'Customer Portal',
            0,
            '', NULL, NULL,
            now_ts, author, now_ts, author
        FROM cfg
    ON CONFLICT (name) DO UPDATE SET
            description = EXCLUDED.description,
            navigation = EXCLUDED.navigation,
            xml_content_raw = EXCLUDED.xml_content_raw,
            xml_content_parsed = EXCLUDED.xml_content_parsed,
            xml_filename = EXCLUDED.xml_filename,
            effective_value = EXCLUDED.effective_value,
            is_valid = EXCLUDED.is_valid,
            user_modification_possible = EXCLUDED.user_modification_possible,
            user_modification_active = EXCLUDED.user_modification_active,
            change_time = EXCLUDED.change_time,
            change_by = EXCLUDED.change_by;

    INSERT INTO sysconfig_default (
            name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
            has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
            xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
            exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
            create_time, create_by, change_time, change_by
    ) SELECT
            'CustomerPortal::FooterText',
            'Footer text displayed on customer portal pages.',
            navigation,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"string","default":"Powered by GoatFlow"}',
            '{"type":"string","default":"Powered by GoatFlow"}',
            xml_file,
            'Powered by GoatFlow',
            0,
            '', NULL, NULL,
            now_ts, author, now_ts, author
        FROM cfg
    ON CONFLICT (name) DO UPDATE SET
            description = EXCLUDED.description,
            navigation = EXCLUDED.navigation,
            xml_content_raw = EXCLUDED.xml_content_raw,
            xml_content_parsed = EXCLUDED.xml_content_parsed,
            xml_filename = EXCLUDED.xml_filename,
            effective_value = EXCLUDED.effective_value,
            is_valid = EXCLUDED.is_valid,
            user_modification_possible = EXCLUDED.user_modification_possible,
            user_modification_active = EXCLUDED.user_modification_active,
            change_time = EXCLUDED.change_time,
            change_by = EXCLUDED.change_by;

    INSERT INTO sysconfig_default (
            name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
            has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
            xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
            exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
            create_time, create_by, change_time, change_by
    ) SELECT
            'CustomerPortal::LandingPage',
            'Relative path used after login (or on portal entry).',
            navigation,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"string","default":"/customer/tickets"}',
            '{"type":"string","default":"/customer/tickets"}',
            xml_file,
            '/customer/tickets',
            0,
            '', NULL, NULL,
            now_ts, author, now_ts, author
        FROM cfg
    ON CONFLICT (name) DO UPDATE SET
            description = EXCLUDED.description,
            navigation = EXCLUDED.navigation,
            xml_content_raw = EXCLUDED.xml_content_raw,
            xml_content_parsed = EXCLUDED.xml_content_parsed,
            xml_filename = EXCLUDED.xml_filename,
            effective_value = EXCLUDED.effective_value,
            is_valid = EXCLUDED.is_valid,
            user_modification_possible = EXCLUDED.user_modification_possible,
            user_modification_active = EXCLUDED.user_modification_active,
            change_time = EXCLUDED.change_time,
            change_by = EXCLUDED.change_by;
END;
$$;
