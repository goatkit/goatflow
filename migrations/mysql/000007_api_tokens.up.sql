-- API Tokens for programmatic access (Personal Access Tokens)
-- Supports both agent and customer users with scoped permissions

CREATE TABLE user_api_tokens (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,
    user_type ENUM('agent', 'customer') NOT NULL DEFAULT 'agent',
    
    -- Token identification
    name VARCHAR(100) NOT NULL,
    prefix VARCHAR(8) NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    
    -- Permissions (NULL = inherit all user permissions)
    scopes JSON,
    
    -- Lifecycle
    expires_at DATETIME NULL,
    last_used_at DATETIME NULL,
    last_used_ip VARCHAR(45),
    
    -- Rate limiting
    rate_limit INT NOT NULL DEFAULT 1000,
    
    -- Audit
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INT,
    revoked_at DATETIME NULL,
    revoked_by INT,
    
    -- Indexes
    INDEX idx_user_api_tokens_prefix (prefix),
    INDEX idx_user_api_tokens_user (user_id, user_type),
    INDEX idx_user_api_tokens_active (revoked_at, expires_at),
    
    -- Note: No FK to users table as it could be agent OR customer_user
    -- Application layer handles validation
    CONSTRAINT chk_user_api_tokens_rate_limit CHECK (rate_limit > 0 AND rate_limit <= 100000)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
