-- Migration: Add missing OTRS v6 standard attachment tables
-- These tables exist in OTRS v6 but were missing from our initial schema migration
-- Per SCHEMA_FREEZE policy: We maintain exact OTRS v6 schema compatibility

-- Create standard_attachment table (exists in OTRS v6)
CREATE TABLE IF NOT EXISTS standard_attachment (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    content_type VARCHAR(250) NOT NULL,
    content BYTEA NOT NULL,  -- PostgreSQL uses BYTEA instead of MySQL LONGBLOB
    filename VARCHAR(250) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- Create indexes matching OTRS
CREATE INDEX idx_standard_attachment_valid_id ON standard_attachment(valid_id);
CREATE INDEX idx_standard_attachment_create_by ON standard_attachment(create_by);
CREATE INDEX idx_standard_attachment_change_by ON standard_attachment(change_by);

-- Create standard_template table (exists in OTRS v6)
CREATE TABLE IF NOT EXISTS standard_template (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    text TEXT,
    content_type VARCHAR(250) DEFAULT 'text/plain; charset=utf-8',
    template_type VARCHAR(100) NOT NULL DEFAULT 'Answer',  -- Answer, Forward, Create
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- Create indexes matching OTRS
CREATE INDEX idx_standard_template_valid_id ON standard_template(valid_id);
CREATE INDEX idx_standard_template_create_by ON standard_template(create_by);
CREATE INDEX idx_standard_template_change_by ON standard_template(change_by);

-- Create standard_template_attachment junction table (exists in OTRS v6)
CREATE TABLE IF NOT EXISTS standard_template_attachment (
    id SERIAL PRIMARY KEY,
    standard_attachment_id INTEGER NOT NULL,
    standard_template_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    
    -- Foreign key constraints matching OTRS
    CONSTRAINT fk_sta_attachment FOREIGN KEY (standard_attachment_id) 
        REFERENCES standard_attachment(id) ON DELETE CASCADE,
    CONSTRAINT fk_sta_template FOREIGN KEY (standard_template_id) 
        REFERENCES standard_template(id) ON DELETE CASCADE,
    
    -- Ensure unique combination
    CONSTRAINT uk_template_attachment UNIQUE(standard_attachment_id, standard_template_id)
);

-- Create indexes matching OTRS
CREATE INDEX idx_sta_attachment_id ON standard_template_attachment(standard_attachment_id);
CREATE INDEX idx_sta_template_id ON standard_template_attachment(standard_template_id);
CREATE INDEX idx_sta_create_by ON standard_template_attachment(create_by);
CREATE INDEX idx_sta_change_by ON standard_template_attachment(change_by);

-- Create queue_standard_template junction table (exists in OTRS v6)
CREATE TABLE IF NOT EXISTS queue_standard_template (
    id SERIAL PRIMARY KEY,
    queue_id INTEGER NOT NULL,
    standard_template_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    
    -- Foreign key constraints matching OTRS
    CONSTRAINT fk_qst_queue FOREIGN KEY (queue_id) 
        REFERENCES queue(id) ON DELETE CASCADE,
    CONSTRAINT fk_qst_template FOREIGN KEY (standard_template_id) 
        REFERENCES standard_template(id) ON DELETE CASCADE,
    
    -- Ensure unique combination
    CONSTRAINT uk_queue_template UNIQUE(queue_id, standard_template_id)
);

-- Create indexes matching OTRS
CREATE INDEX idx_qst_queue_id ON queue_standard_template(queue_id);
CREATE INDEX idx_qst_template_id ON queue_standard_template(standard_template_id);
CREATE INDEX idx_qst_create_by ON queue_standard_template(create_by);
CREATE INDEX idx_qst_change_by ON queue_standard_template(change_by);