-- Reverse the setup.assistant.completed seed.
DELETE FROM sysconfig_default WHERE name = 'setup.assistant.completed';
