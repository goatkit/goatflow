-- Remove article_type_id column from article table
DROP INDEX IF EXISTS idx_article_type_id;
ALTER TABLE article DROP COLUMN IF EXISTS article_type_id;