-- Canned responses (pre-written response templates for agents)
-- Ported from migrations/mysql/000005_canned_response.up.sql

CREATE TABLE IF NOT EXISTS canned_response (
    id SERIAL PRIMARY KEY,
    name varchar(200) NOT NULL,
    category varchar(100) DEFAULT NULL,
    content text NOT NULL,
    content_type varchar(50) NOT NULL DEFAULT 'text',
    tags text DEFAULT NULL,
    scope varchar(20) NOT NULL DEFAULT 'personal',
    owner_id int NOT NULL,
    team_id int DEFAULT NULL,
    placeholders text DEFAULT NULL,
    usage_count int NOT NULL DEFAULT 0,
    last_used timestamptz DEFAULT NULL,
    valid_id smallint NOT NULL DEFAULT 1,
    create_time timestamptz NOT NULL,
    create_by int NOT NULL,
    change_time timestamptz NOT NULL,
    change_by int NOT NULL,
    CONSTRAINT fk_cr_owner_id FOREIGN KEY (owner_id) REFERENCES users(id),
    CONSTRAINT fk_cr_create_by_id FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_cr_change_by_id FOREIGN KEY (change_by) REFERENCES users(id),
    CONSTRAINT fk_cr_valid_id_id FOREIGN KEY (valid_id) REFERENCES valid(id)
);

CREATE INDEX idx_cr_name ON canned_response (name);
CREATE INDEX idx_cr_scope ON canned_response (scope);
CREATE INDEX idx_cr_owner_id ON canned_response (owner_id);
CREATE INDEX idx_cr_team_id ON canned_response (team_id);
CREATE INDEX idx_cr_category ON canned_response (category);

-- Canned response categories
CREATE TABLE IF NOT EXISTS canned_response_category (
    id SERIAL PRIMARY KEY,
    name varchar(100) NOT NULL,
    description varchar(250) DEFAULT NULL,
    parent_id int DEFAULT NULL,
    valid_id smallint NOT NULL DEFAULT 1,
    create_time timestamptz NOT NULL,
    create_by int NOT NULL,
    change_time timestamptz NOT NULL,
    change_by int NOT NULL,
    CONSTRAINT uq_crc_name UNIQUE (name),
    CONSTRAINT fk_crc_parent_id FOREIGN KEY (parent_id) REFERENCES canned_response_category(id),
    CONSTRAINT fk_crc_create_by_id FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_crc_change_by_id FOREIGN KEY (change_by) REFERENCES users(id),
    CONSTRAINT fk_crc_valid_id_id FOREIGN KEY (valid_id) REFERENCES valid(id)
);

-- Insert default categories
INSERT INTO canned_response_category (name, description, valid_id, create_time, create_by, change_time, change_by) VALUES
  ('General', 'General purpose responses', 1, NOW(), 1, NOW(), 1),
  ('Account', 'Account-related responses', 1, NOW(), 1, NOW(), 1),
  ('Technical', 'Technical support responses', 1, NOW(), 1, NOW(), 1),
  ('Billing', 'Billing and payment responses', 1, NOW(), 1, NOW(), 1),
  ('System', 'System notifications and alerts', 1, NOW(), 1, NOW(), 1)
ON CONFLICT (name) DO NOTHING;
