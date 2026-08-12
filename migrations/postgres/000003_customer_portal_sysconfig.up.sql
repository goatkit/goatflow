-- Seed customer portal sysconfig entries (no schema change).
-- NOTE: use DO-block variables, not a WITH CTE — a CTE is scoped to the single
-- statement it is attached to, so later INSERTs referencing it fail with
-- 'relation "cfg" does not exist'.
DO $$
DECLARE
    def_exists boolean := to_regclass('sysconfig_default') IS NOT NULL;
    v_nav text := 'Frontend::Customer::Portal';
    v_xml_file text := 'CustomerPortal.xml';
    v_now timestamp := NOW();
    v_author int := 1;
BEGIN
    IF NOT def_exists THEN
        RAISE NOTICE 'sysconfig_default missing; skipping CustomerPortal seed';
        RETURN;
    END IF;

    INSERT INTO sysconfig_default (
            name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
            has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
            xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
            exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
            create_time, create_by, change_time, change_by
    ) VALUES (
            'CustomerPortal::Enabled',
            'Allow customers to access the portal and ticket UI.',
            v_nav,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"boolean","default":true}',
            '{"type":"boolean","default":true}',
            v_xml_file,
            'true',
            0,
            '', NULL, NULL,
            v_now, v_author, v_now, v_author)
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
    ) VALUES (
            'CustomerPortal::LoginRequired',
            'Require customer authentication before accessing the portal.',
            v_nav,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"boolean","default":true}',
            '{"type":"boolean","default":true}',
            v_xml_file,
            'true',
            0,
            '', NULL, NULL,
            v_now, v_author, v_now, v_author)
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
    ) VALUES (
            'CustomerPortal::Title',
            'Portal title shown in header and HTML title.',
            v_nav,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"string","default":"Customer Portal"}',
            '{"type":"string","default":"Customer Portal"}',
            v_xml_file,
            'Customer Portal',
            0,
            '', NULL, NULL,
            v_now, v_author, v_now, v_author)
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
    ) VALUES (
            'CustomerPortal::FooterText',
            'Footer text displayed on customer portal pages.',
            v_nav,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"string","default":"Powered by GoatFlow"}',
            '{"type":"string","default":"Powered by GoatFlow"}',
            v_xml_file,
            'Powered by GoatFlow',
            0,
            '', NULL, NULL,
            v_now, v_author, v_now, v_author)
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
    ) VALUES (
            'CustomerPortal::LandingPage',
            'Relative path used after login (or on portal entry).',
            v_nav,
            0, 0, 0, 1,
            0, 1, 1, NULL,
            '{"type":"string","default":"/customer/tickets"}',
            '{"type":"string","default":"/customer/tickets"}',
            v_xml_file,
            '/customer/tickets',
            0,
            '', NULL, NULL,
            v_now, v_author, v_now, v_author)
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
