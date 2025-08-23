-- Add missing article_type_id column to article table
-- This column is part of standard OTRS schema but was missing from initial migration

ALTER TABLE article 
ADD COLUMN IF NOT EXISTS article_type_id SMALLINT NOT NULL DEFAULT 1;

-- Add index for better query performance
CREATE INDEX IF NOT EXISTS idx_article_type_id ON article(article_type_id);

-- Add comment explaining the column
COMMENT ON COLUMN article.article_type_id IS 'Type of article (1=email-external, 2=email-internal, 3=email-notification, 4=phone, 5=fax, 6=sms, 7=webrequest, 8=note-internal, 9=note-external, 10=note-report)';