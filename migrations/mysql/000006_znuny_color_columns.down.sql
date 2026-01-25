-- Note: We don't drop the column in down migration as it may have user data
-- and the column IS part of current Znuny schema. This is intentionally a no-op.
-- If you truly need to remove it, run: ALTER TABLE ticket_priority DROP COLUMN color;
SELECT 1;
