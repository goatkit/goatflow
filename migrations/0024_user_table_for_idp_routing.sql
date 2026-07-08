-- Add user_table column to identity_providers for agent vs customer routing
ALTER TABLE gk_identity_provider ADD COLUMN user_table VARCHAR(20) NOT NULL DEFAULT 'users';
