-- Mirror of the MySQL migration. See the MySQL copy for the design
-- note.
ALTER TABLE gk_organisation
    ADD COLUMN IF NOT EXISTS captive_plugin VARCHAR(100) DEFAULT NULL;
