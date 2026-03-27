-- Organisations (GoatKit PaaS Core — Multi-Tenancy)
CREATE TABLE IF NOT EXISTS gk_organisation (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    name        VARCHAR(200) NOT NULL,
    slug        VARCHAR(100) NOT NULL,
    parent_id   BIGINT DEFAULT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    customer_company_id VARCHAR(150) DEFAULT NULL,
    valid_id    SMALLINT NOT NULL DEFAULT 1,
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by   INT NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_slug (slug),
    KEY idx_parent (parent_id),
    KEY idx_status (status),
    KEY idx_customer_company (customer_company_id),
    CONSTRAINT fk_org_parent FOREIGN KEY (parent_id) REFERENCES gk_organisation(id),
    CONSTRAINT fk_org_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_org_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- User <-> Organisation membership (agents and customers)
CREATE TABLE IF NOT EXISTS gk_user_organisation (
    id          BIGINT NOT NULL AUTO_INCREMENT,
    org_id      BIGINT NOT NULL,
    user_id     INT DEFAULT NULL,
    customer_login VARCHAR(200) DEFAULT NULL,
    role        VARCHAR(50) NOT NULL DEFAULT 'member',
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,

    PRIMARY KEY (id),
    KEY idx_org (org_id),
    KEY idx_user (user_id),
    KEY idx_customer (customer_login),
    UNIQUE KEY uk_org_user (org_id, user_id),
    CONSTRAINT fk_uo_org FOREIGN KEY (org_id) REFERENCES gk_organisation(id) ON DELETE CASCADE,
    CONSTRAINT fk_uo_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_uo_create_by FOREIGN KEY (create_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Per-org sysconfig overrides
CREATE TABLE IF NOT EXISTS sysconfig_org (
    id              BIGINT NOT NULL AUTO_INCREMENT,
    org_id          BIGINT NOT NULL,
    name            VARCHAR(250) NOT NULL,
    effective_value LONGBLOB NOT NULL,
    is_valid        SMALLINT NOT NULL DEFAULT 1,
    create_time     DATETIME NOT NULL,
    create_by       INT NOT NULL,
    change_time     DATETIME NOT NULL,
    change_by       INT NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_org_name (org_id, name),
    CONSTRAINT fk_sco_org FOREIGN KEY (org_id) REFERENCES gk_organisation(id) ON DELETE CASCADE,
    CONSTRAINT fk_sco_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_sco_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
