-- Rollback enhanced schema changes

-- Remove indexes
DROP INDEX IF EXISTS idx_email_accounts_queue;
DROP INDEX IF EXISTS idx_tickets_category;
DROP INDEX IF EXISTS idx_customer_accounts_org;
DROP INDEX IF EXISTS idx_customer_accounts_email;
DROP INDEX IF EXISTS idx_organizations_name;

-- Remove new tables
DROP TABLE IF EXISTS email_templates CASCADE;
DROP TABLE IF EXISTS email_accounts CASCADE;
DROP TABLE IF EXISTS customer_accounts CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
DROP TABLE IF EXISTS ticket_categories CASCADE;

-- Remove added columns from tickets
ALTER TABLE tickets
DROP COLUMN IF EXISTS category_id,
DROP COLUMN IF EXISTS response_deadline,
DROP COLUMN IF EXISTS resolution_deadline,
DROP COLUMN IF EXISTS last_customer_contact,
DROP COLUMN IF EXISTS last_agent_contact;

-- Remove color column from priorities
ALTER TABLE ticket_priorities 
DROP COLUMN IF EXISTS display_color;

-- Note: We cannot safely revert customer_id back to INTEGER if there's data
-- This would need manual intervention based on the data state

-- Restore original constraint
ALTER TABLE ticket_states 
DROP CONSTRAINT IF EXISTS ticket_states_type_id_check;

ALTER TABLE ticket_states 
ADD CONSTRAINT ticket_states_type_id_check 
CHECK (type_id IN (1, 2, 3, 4, 5, 6));