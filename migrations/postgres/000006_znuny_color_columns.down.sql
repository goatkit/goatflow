-- Note: We don't drop the column in down migration as it may have user data
-- and the column IS part of current Znuny schema. This is intentionally a no-op.
SELECT 1;
