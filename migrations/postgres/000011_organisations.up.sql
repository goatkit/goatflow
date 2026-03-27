-- Organisations (GoatKit PaaS Core — Multi-Tenancy)
CREATE TABLE IF NOT EXISTS gk_organisation (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    slug        VARCHAR(100) NOT NULL UNIQUE,
    parent_id   BIGINT DEFAULT NULL REFERENCES gk_organisation(id),
    status      VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','suspended','archived')),
    customer_company_id VARCHAR(150) DEFAULT NULL,
    valid_id    SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP NOT NULL,
    create_by   INT NOT NULL REFERENCES users(id),
    change_time TIMESTAMP NOT NULL,
    change_by   INT NOT NULL REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_org_parent ON gk_organisation(parent_id);
CREATE INDEX IF NOT EXISTS idx_org_status ON gk_organisation(status);
CREATE INDEX IF NOT EXISTS idx_org_customer_company ON gk_organisation(customer_company_id);

-- User <-> Organisation membership (agents and customers)
CREATE TABLE IF NOT EXISTS gk_user_organisation (
    id          BIGSERIAL PRIMARY KEY,
    org_id      BIGINT NOT NULL REFERENCES gk_organisation(id) ON DELETE CASCADE,
    user_id     INT DEFAULT NULL REFERENCES users(id) ON DELETE CASCADE,
    customer_login VARCHAR(200) DEFAULT NULL,
    role        VARCHAR(50) NOT NULL DEFAULT 'member'
        CHECK (role IN ('member','admin','owner')),
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    create_time TIMESTAMP NOT NULL,
    create_by   INT NOT NULL REFERENCES users(id),

    UNIQUE (org_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_uo_org ON gk_user_organisation(org_id);
CREATE INDEX IF NOT EXISTS idx_uo_user ON gk_user_organisation(user_id);
CREATE INDEX IF NOT EXISTS idx_uo_customer ON gk_user_organisation(customer_login);

-- Per-org sysconfig overrides
CREATE TABLE IF NOT EXISTS sysconfig_org (
    id              BIGSERIAL PRIMARY KEY,
    org_id          BIGINT NOT NULL REFERENCES gk_organisation(id) ON DELETE CASCADE,
    name            VARCHAR(250) NOT NULL,
    effective_value BYTEA NOT NULL,
    is_valid        SMALLINT NOT NULL DEFAULT 1,
    create_time     TIMESTAMP NOT NULL,
    create_by       INT NOT NULL REFERENCES users(id),
    change_time     TIMESTAMP NOT NULL,
    change_by       INT NOT NULL REFERENCES users(id),

    UNIQUE (org_id, name)
);
