-- Seed global service-worker cache configuration.
SET FOREIGN_KEY_CHECKS = 0;

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
    'ServiceWorker::Enabled',
    'Enable the GoatFlow service worker for offline support and push notifications.',
    'Frontend::PWA',
    0, 0, 0, 1, 0, 1, 1, NULL,
    '{"type":"boolean","default":true}',
    '{"type":"boolean","default":true}',
    'ServiceWorker.xml',
    'true',
    0, '', NULL, NULL, NOW(), 1, NOW(), 1
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = NOW(),
    change_by = 1;

INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'ServiceWorker::DefaultNavigationStrategy',
    'Default cache strategy for navigation requests not matched by a route rule.',
    'Frontend::PWA',
    0, 0, 0, 1, 0, 1, 1, NULL,
    '{"type":"string","default":"network-first"}',
    '{"type":"string","default":"network-first"}',
    'ServiceWorker.xml',
    'network-first',
    0, '', NULL, NULL, NOW(), 1, NOW(), 1
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = NOW(),
    change_by = 1;

INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'ServiceWorker::Routes',
    'JSON array of same-origin service-worker cache route rules with path and strategy.',
    'Frontend::PWA',
    0, 0, 0, 1, 0, 1, 1, NULL,
    '{"type":"string","default":"[]"}',
    '{"type":"string","default":"[]"}',
    'ServiceWorker.xml',
    '[]',
    0, '', NULL, NULL, NOW(), 1, NOW(), 1
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = NOW(),
    change_by = 1;

SET FOREIGN_KEY_CHECKS = 1;
