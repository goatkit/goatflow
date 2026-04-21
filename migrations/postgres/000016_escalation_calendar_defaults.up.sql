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

DO $$
DECLARE
    def_exists boolean := to_regclass('sysconfig_default') IS NOT NULL;
BEGIN
    IF NOT def_exists THEN
        RAISE NOTICE 'sysconfig_default missing; skipping TimeWorkingHours seed';
        RETURN;
    END IF;

    WITH cfg AS (
        SELECT 'Core::Time'::text AS navigation,
               'Framework.xml'::text AS xml_file,
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
        'TimeWorkingHours',
        'Defines the default business hours used by the SLA escalation service. Each day lists the clock hours during which work happens; an empty list means the day is non-working.',
        navigation,
        0, 0, 1, 1,
        0, 1, 1, NULL,
        '{"type":"hash"}',
        '{"type":"hash"}',
        xml_file,
        '{Mon: [8,9,10,11,12,13,14,15,16,17], Tue: [8,9,10,11,12,13,14,15,16,17], Wed: [8,9,10,11,12,13,14,15,16,17], Thu: [8,9,10,11,12,13,14,15,16,17], Fri: [8,9,10,11,12,13,14,15,16,17], Sat: [], Sun: []}',
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
        is_valid = EXCLUDED.is_valid,
        -- Do NOT overwrite effective_value on re-run — preserves any admin
        -- edits applied via the sysconfig UI. Fresh installs still pick up
        -- the default because the row doesn't exist yet.
        change_time = EXCLUDED.change_time,
        change_by = EXCLUDED.change_by;
END $$;
