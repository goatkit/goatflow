-- Seed customer portal sysconfig entries (reuses existing sysconfig_* tables; no new schema).
-- Note: golang-migrate manages transactions, so we don't use START TRANSACTION/COMMIT here.

SET FOREIGN_KEY_CHECKS = 0;

SET @cfg_navigation := 'Frontend::Customer::Portal';
SET @cfg_file := 'CustomerPortal.xml';
SET @now := NOW();
SET @has_sysconfig_default := (
    SELECT COUNT(*)
        FROM information_schema.tables
     WHERE table_schema = DATABASE()
         AND table_name = 'sysconfig_default'
);
SET @has_sysconfig_default := IFNULL(@has_sysconfig_default, 0);
SET @author := 1;

-- Customer portal enable/disable
INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'CustomerPortal::Enabled',
    'Allow customers to access the portal and ticket UI.',
    @cfg_navigation,
    0, 0, 0, 1,
    0, 1, 1, NULL,
    '{"type":"boolean","default":true}',
    '{"type":"boolean","default":true}',
    @cfg_file,
    'true',
    0,
    '', NULL, NULL,
    @now, @author, @now, @author
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    effective_value = VALUES(effective_value),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = @now,
    change_by = @author;

-- Customer portal login requirement
INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'CustomerPortal::LoginRequired',
    'Require customer authentication before accessing the portal.',
    @cfg_navigation,
    0, 0, 0, 1,
    0, 1, 1, NULL,
    '{"type":"boolean","default":true}',
    '{"type":"boolean","default":true}',
    @cfg_file,
    'true',
    0,
    '', NULL, NULL,
    @now, @author, @now, @author
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    effective_value = VALUES(effective_value),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = @now,
    change_by = @author;

-- Customer portal title/branding
INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'CustomerPortal::Title',
    'Portal title shown in header and HTML title.',
    @cfg_navigation,
    0, 0, 0, 1,
    0, 1, 1, NULL,
    '{"type":"string","default":"Customer Portal"}',
    '{"type":"string","default":"Customer Portal"}',
    @cfg_file,
    'Customer Portal',
    0,
    '', NULL, NULL,
    @now, @author, @now, @author
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    effective_value = VALUES(effective_value),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = @now,
    change_by = @author;

-- Footer text
INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'CustomerPortal::FooterText',
    'Footer text displayed on customer portal pages.',
    @cfg_navigation,
    0, 0, 0, 1,
    0, 1, 1, NULL,
    '{"type":"string","default":"Powered by GoatFlow"}',
    '{"type":"string","default":"Powered by GoatFlow"}',
    @cfg_file,
    'Powered by GoatFlow',
    0,
    '', NULL, NULL,
    @now, @author, @now, @author
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    effective_value = VALUES(effective_value),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = @now,
    change_by = @author;

-- Landing page
INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'CustomerPortal::LandingPage',
    'Relative path used after login (or on portal entry).',
    @cfg_navigation,
    0, 0, 0, 1,
    0, 1, 1, NULL,
    '{"type":"string","default":"/customer/tickets"}',
    '{"type":"string","default":"/customer/tickets"}',
    @cfg_file,
    '/customer/tickets',
    0,
    '', NULL, NULL,
    @now, @author, @now, @author
  WHERE @has_sysconfig_default = 1
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    navigation = VALUES(navigation),
    xml_content_raw = VALUES(xml_content_raw),
    xml_content_parsed = VALUES(xml_content_parsed),
    xml_filename = VALUES(xml_filename),
    effective_value = VALUES(effective_value),
    is_valid = VALUES(is_valid),
    user_modification_possible = VALUES(user_modification_possible),
    user_modification_active = VALUES(user_modification_active),
    change_time = @now,
    change_by = @author;

SET FOREIGN_KEY_CHECKS = 1;
-- Migration complete (transaction managed by golang-migrate)
