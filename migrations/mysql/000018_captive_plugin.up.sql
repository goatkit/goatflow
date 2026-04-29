-- Per-org "captive plugin" setting. When set, customers of this org are
-- redirected to the named plugin on login and can't navigate away to
-- the standard customer portal pages. Used when a plugin fully replaces
-- the customer UX for a given tenant (e.g. an org whose only product is
-- goatfictus would set captive_plugin='goatfictus'). NULL = normal
-- portal behaviour. Enforced at the login-redirect and customer-portal
-- middleware layers; an admin-side check keeps you from picking a
-- plugin that isn't enabled for the org via gk_org_plugin_access.
ALTER TABLE gk_organisation
    ADD COLUMN captive_plugin VARCHAR(100) DEFAULT NULL;
