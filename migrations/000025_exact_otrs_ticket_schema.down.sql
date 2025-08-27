-- Revert to original GOTRS ticket schema
DROP TABLE IF EXISTS ticket CASCADE;
DROP TABLE IF EXISTS article CASCADE; 
DROP TABLE IF EXISTS ticket_history CASCADE;

-- This down migration would need to restore the original schema
-- For now, we'll leave it as a drop since we're doing a clean slate approach