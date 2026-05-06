-- Seed global service-worker cache configuration.
DO $$
DECLARE
    def_exists boolean := to_regclass('sysconfig_default') IS NOT NULL;
BEGIN
    IF NOT def_exists THEN
        RAISE NOTICE 'sysconfig_default missing; skipping ServiceWorker seed';
        RETURN;
    END IF;

    INSERT INTO sysconfig_default (
        name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
        has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
        xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
        exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
        create_time, create_by, change_time, change_by
    ) VALUES
    (
        'ServiceWorker::Enabled',
        'Enable the GoatFlow service worker for offline support and push notifications.',
        'Frontend::PWA',
        0, 0, 0, 1, 0, 1, 1, NULL,
        '{"type":"boolean","default":true}',
        '{"type":"boolean","default":true}',
        'ServiceWorker.xml',
        'true',
        0, '', NULL, NULL, NOW(), 1, NOW(), 1
    ),
    (
        'ServiceWorker::DefaultNavigationStrategy',
        'Default cache strategy for navigation requests not matched by a route rule.',
        'Frontend::PWA',
        0, 0, 0, 1, 0, 1, 1, NULL,
        '{"type":"string","default":"network-first"}',
        '{"type":"string","default":"network-first"}',
        'ServiceWorker.xml',
        'network-first',
        0, '', NULL, NULL, NOW(), 1, NOW(), 1
    ),
    (
        'ServiceWorker::Routes',
        'JSON array of same-origin service-worker cache route rules with path and strategy.',
        'Frontend::PWA',
        0, 0, 0, 1, 0, 1, 1, NULL,
        '{"type":"string","default":"[]"}',
        '{"type":"string","default":"[]"}',
        'ServiceWorker.xml',
        '[]',
        0, '', NULL, NULL, NOW(), 1, NOW(), 1
    )
    ON CONFLICT (name) DO UPDATE SET
        description = EXCLUDED.description,
        navigation = EXCLUDED.navigation,
        xml_content_raw = EXCLUDED.xml_content_raw,
        xml_content_parsed = EXCLUDED.xml_content_parsed,
        xml_filename = EXCLUDED.xml_filename,
        is_valid = EXCLUDED.is_valid,
        user_modification_possible = EXCLUDED.user_modification_possible,
        user_modification_active = EXCLUDED.user_modification_active,
        change_time = EXCLUDED.change_time,
        change_by = EXCLUDED.change_by;
END $$;
