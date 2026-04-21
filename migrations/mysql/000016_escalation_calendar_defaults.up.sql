-- Seed the default business calendar for the escalation service.
--
-- Without a TimeWorkingHours row in sysconfig_default, the scheduler's
-- escalation service logs every minute:
--   scheduler: failed to initialize escalation service: failed to load default calendar:
--   failed to get TimeWorkingHours: sql: no rows in result set
-- and SLA escalation calculations don't run at all.
--
-- Default: 08:00–18:00 Monday–Friday, weekends off.
--
-- Format expected by internal/services/escalation/calendar.go::applyWorkingHours
-- is OTRS-style YAML: { Mon: [hours...], ... } where each hour number means
-- "this clock hour is a working hour". The calendar service derives
-- start/end times as [min(hours), max(hours)+1), so [8..17] → 08:00–18:00.
--
-- Admins can override per-install by editing the sysconfig UI (lands in
-- sysconfig_modified) or by changing the effective_value on this row.

SET FOREIGN_KEY_CHECKS = 0;

SET @cfg_navigation := 'Core::Time';
SET @cfg_file := 'Framework.xml';
SET @now := NOW();
SET @has_sysconfig_default := (
    SELECT COUNT(*)
        FROM information_schema.tables
     WHERE table_schema = DATABASE()
         AND table_name = 'sysconfig_default'
);
SET @has_sysconfig_default := IFNULL(@has_sysconfig_default, 0);
SET @author := 1;

INSERT INTO sysconfig_default (
    name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
    has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
    xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
    exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
    create_time, create_by, change_time, change_by
) SELECT
    'TimeWorkingHours',
    'Defines the default business hours used by the SLA escalation service. Each day lists the clock hours during which work happens; an empty list means the day is non-working.',
    @cfg_navigation,
    0, 0, 1, 1,
    0, 1, 1, NULL,
    '{"type":"hash"}',
    '{"type":"hash"}',
    @cfg_file,
    '{Mon: [8,9,10,11,12,13,14,15,16,17], Tue: [8,9,10,11,12,13,14,15,16,17], Wed: [8,9,10,11,12,13,14,15,16,17], Thu: [8,9,10,11,12,13,14,15,16,17], Fri: [8,9,10,11,12,13,14,15,16,17], Sat: [], Sun: []}',
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
    is_valid = VALUES(is_valid),
    -- Do NOT overwrite effective_value on re-run — preserves any admin edits
    -- that were applied via the sysconfig UI. Fresh installs still pick up
    -- the default because the row doesn't exist yet.
    change_time = @now,
    change_by = @author;

SET FOREIGN_KEY_CHECKS = 1;
