-- Indexes for the Zinc/Elasticsearch search sync (internal/platform/search):
-- the runner reads ticket, article and customer_user rows by change_time
-- every 30 seconds to find what to reindex.
CREATE INDEX IF NOT EXISTS ticket_change_time ON ticket (change_time);
CREATE INDEX IF NOT EXISTS article_change_time ON article (change_time);
CREATE INDEX IF NOT EXISTS customer_user_change_time ON customer_user (change_time);
