DO $$
BEGIN
  IF to_regclass('sysconfig_modified') IS NOT NULL THEN
    DELETE FROM sysconfig_modified WHERE name LIKE 'CustomerPortal::%';
  END IF;

  IF to_regclass('sysconfig_default') IS NOT NULL THEN
    DELETE FROM sysconfig_default WHERE name LIKE 'CustomerPortal::%';
  END IF;
EXCEPTION
  WHEN undefined_table THEN
    NULL;
END;
$$;
