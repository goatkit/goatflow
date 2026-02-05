-- API Tokens for programmatic access (Personal Access Tokens)
-- Supports both agent and customer users with scoped permissions

CREATE TYPE user_type_enum AS ENUM ('agent', 'customer');

CREATE TABLE user_api_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    user_type user_type_enum NOT NULL DEFAULT 'agent',
    
    -- Token identification
    name VARCHAR(100) NOT NULL,
    prefix VARCHAR(8) NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    
    -- Permissions (NULL = inherit all user permissions)
    scopes JSONB,
    
    -- Lifecycle
    expires_at TIMESTAMP NULL,
    last_used_at TIMESTAMP NULL,
    last_used_ip VARCHAR(45),
    
    -- Rate limiting
    rate_limit INT NOT NULL DEFAULT 1000 CHECK (rate_limit > 0 AND rate_limit <= 100000),
    
    -- Audit
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INT,
    revoked_at TIMESTAMP NULL,
    revoked_by INT
);

-- Indexes
CREATE INDEX idx_user_api_tokens_prefix ON user_api_tokens(prefix);
CREATE INDEX idx_user_api_tokens_user ON user_api_tokens(user_id, user_type);
CREATE INDEX idx_user_api_tokens_active ON user_api_tokens(revoked_at, expires_at);
