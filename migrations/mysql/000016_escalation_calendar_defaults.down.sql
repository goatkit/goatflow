-- Remove the default business-calendar seed row.
DELETE FROM sysconfig_default WHERE name = 'TimeWorkingHours';
