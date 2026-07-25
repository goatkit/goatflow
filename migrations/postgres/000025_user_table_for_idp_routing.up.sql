-- Add user_table column to identity_providers for agent vs customer routing
-- Idempotent: uses IF NOT EXISTS to prevent duplicate column errors  
ALTER TABLE gk_identity_provider ADD COLUMN IF NOT EXISTS user_table VARCHAR(20) NOT NULL DEFAULT 'users';
