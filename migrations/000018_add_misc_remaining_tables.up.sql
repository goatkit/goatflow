-- Batch 9: Miscellaneous remaining tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Customer to Customer Company mapping
CREATE TABLE IF NOT EXISTS customer_user_customer (
    user_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(150) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (user_id, customer_id)
);

CREATE INDEX idx_customer_user_customer_customer_id ON customer_user_customer(customer_id);
CREATE INDEX idx_customer_user_customer_user_id ON customer_user_customer(user_id);

-- Group to Customer mapping
CREATE TABLE IF NOT EXISTS group_customer (
    group_id INTEGER NOT NULL,
    customer_id VARCHAR(150) NOT NULL,
    permission_key VARCHAR(20) NOT NULL,
    permission_value SMALLINT NOT NULL,
    permission_context VARCHAR(100) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    PRIMARY KEY (group_id, customer_id, permission_context, permission_key),
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

CREATE INDEX idx_group_customer_customer_id ON group_customer(customer_id);

-- Service to Customer User mapping
CREATE TABLE IF NOT EXISTS service_customer_user (
    customer_user_login VARCHAR(200) NOT NULL,
    service_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    PRIMARY KEY (customer_user_login, service_id),
    FOREIGN KEY (service_id) REFERENCES service(id) ON DELETE CASCADE
);

CREATE INDEX idx_service_customer_user_customer_user_login ON service_customer_user(customer_user_login);
CREATE INDEX idx_service_customer_user_service_id ON service_customer_user(service_id);

-- Service to SLA mapping
CREATE TABLE IF NOT EXISTS service_sla (
    service_id INTEGER NOT NULL,
    sla_id INTEGER NOT NULL,
    PRIMARY KEY (service_id, sla_id),
    FOREIGN KEY (service_id) REFERENCES service(id) ON DELETE CASCADE,
    FOREIGN KEY (sla_id) REFERENCES sla(id) ON DELETE CASCADE
);

CREATE INDEX idx_service_sla_service_id ON service_sla(service_id);
CREATE INDEX idx_service_sla_sla_id ON service_sla(sla_id);

-- Service preferences
CREATE TABLE IF NOT EXISTS service_preferences (
    service_id INTEGER NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value VARCHAR(250),
    PRIMARY KEY (service_id, preferences_key),
    FOREIGN KEY (service_id) REFERENCES service(id) ON DELETE CASCADE
);

CREATE INDEX idx_service_preferences_service_id ON service_preferences(service_id);

-- SLA preferences
CREATE TABLE IF NOT EXISTS sla_preferences (
    sla_id INTEGER NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value VARCHAR(250),
    PRIMARY KEY (sla_id, preferences_key),
    FOREIGN KEY (sla_id) REFERENCES sla(id) ON DELETE CASCADE
);

CREATE INDEX idx_sla_preferences_sla_id ON sla_preferences(sla_id);

-- Queue preferences
CREATE TABLE IF NOT EXISTS queue_preferences (
    queue_id INTEGER NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value VARCHAR(250),
    PRIMARY KEY (queue_id, preferences_key),
    FOREIGN KEY (queue_id) REFERENCES queue(id) ON DELETE CASCADE
);

CREATE INDEX idx_queue_preferences_queue_id ON queue_preferences(queue_id);

-- Search profiles
CREATE TABLE IF NOT EXISTS search_profile (
    login VARCHAR(200) NOT NULL,
    profile_name VARCHAR(200) NOT NULL,
    profile_type VARCHAR(30) NOT NULL,
    profile_key VARCHAR(200) NOT NULL,
    profile_value VARCHAR(200),
    PRIMARY KEY (login, profile_name, profile_type, profile_key)
);

CREATE INDEX idx_search_profile_login ON search_profile(login);
CREATE INDEX idx_search_profile_profile_name ON search_profile(profile_name);

-- Form drafts
CREATE TABLE IF NOT EXISTS form_draft (
    id SERIAL PRIMARY KEY,
    object_type VARCHAR(100) NOT NULL,
    object_id INTEGER NOT NULL DEFAULT 0,
    action VARCHAR(200) NOT NULL,
    title VARCHAR(255),
    content TEXT NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_form_draft_object_type_action ON form_draft(object_type, action);
CREATE INDEX idx_form_draft_create_by ON form_draft(create_by);

-- Virtual filesystem
CREATE TABLE IF NOT EXISTS virtual_fs (
    id BIGSERIAL PRIMARY KEY,
    filename VARCHAR(350) NOT NULL,
    backend VARCHAR(60) NOT NULL,
    backend_key VARCHAR(160) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_virtual_fs_filename ON virtual_fs(filename);
CREATE INDEX idx_virtual_fs_backend ON virtual_fs(backend);
CREATE INDEX idx_virtual_fs_backend_key ON virtual_fs(backend_key);

-- Virtual filesystem database storage
CREATE TABLE IF NOT EXISTS virtual_fs_db (
    id BIGSERIAL PRIMARY KEY,
    filename VARCHAR(350) NOT NULL,
    content BYTEA NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_virtual_fs_db_filename ON virtual_fs_db(filename);

-- Virtual filesystem preferences
CREATE TABLE IF NOT EXISTS virtual_fs_preferences (
    virtual_fs_id BIGINT NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value VARCHAR(350),
    PRIMARY KEY (virtual_fs_id, preferences_key),
    FOREIGN KEY (virtual_fs_id) REFERENCES virtual_fs(id) ON DELETE CASCADE
);

CREATE INDEX idx_virtual_fs_preferences_virtual_fs_id ON virtual_fs_preferences(virtual_fs_id);

-- Web upload cache
CREATE TABLE IF NOT EXISTS web_upload_cache (
    form_id VARCHAR(128) NOT NULL,
    field_name VARCHAR(250) NOT NULL,
    content_id VARCHAR(250) NOT NULL,
    filename VARCHAR(250) NOT NULL,
    content_size VARCHAR(30) NOT NULL,
    content_type VARCHAR(250) NOT NULL,
    disposition VARCHAR(15) NOT NULL,
    content BYTEA NOT NULL,
    create_time_unix BIGINT NOT NULL
);

CREATE INDEX idx_web_upload_cache_form_field ON web_upload_cache(form_id, field_name);
CREATE INDEX idx_web_upload_cache_form_id ON web_upload_cache(form_id);

-- XML storage
CREATE TABLE IF NOT EXISTS xml_storage (
    xml_type VARCHAR(200) NOT NULL,
    xml_key VARCHAR(250) NOT NULL,
    xml_content_key VARCHAR(250) NOT NULL,
    xml_content_value TEXT,
    PRIMARY KEY (xml_type, xml_key, xml_content_key)
);

CREATE INDEX idx_xml_storage_xml_type ON xml_storage(xml_type);
CREATE INDEX idx_xml_storage_xml_key ON xml_storage(xml_key);

-- Package repository
CREATE TABLE IF NOT EXISTS package_repository (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    version VARCHAR(250) NOT NULL,
    vendor VARCHAR(250) NOT NULL,
    install_status VARCHAR(250) NOT NULL,
    filename VARCHAR(250),
    content_type VARCHAR(250),
    content BYTEA NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

-- Cloud service configuration
CREATE TABLE IF NOT EXISTS cloud_service_config (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    config TEXT,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_cloud_service_config_valid_id ON cloud_service_config(valid_id);

-- GI (Generic Interface) Webservice configuration
CREATE TABLE IF NOT EXISTS gi_webservice_config (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    config TEXT NOT NULL,
    config_md5 VARCHAR(32) NOT NULL,
    valid_id SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_gi_webservice_config_valid_id ON gi_webservice_config(valid_id);
CREATE INDEX idx_gi_webservice_config_config_md5 ON gi_webservice_config(config_md5);

-- GI Webservice configuration history
CREATE TABLE IF NOT EXISTS gi_webservice_config_history (
    id BIGSERIAL PRIMARY KEY,
    config_id INTEGER NOT NULL,
    config TEXT NOT NULL,
    config_md5 VARCHAR(32) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL,
    FOREIGN KEY (config_id) REFERENCES gi_webservice_config(id) ON DELETE CASCADE
);

CREATE INDEX idx_gi_webservice_config_history_config_id ON gi_webservice_config_history(config_id);

-- GI Debugger entry
CREATE TABLE IF NOT EXISTS gi_debugger_entry (
    id BIGSERIAL PRIMARY KEY,
    communication_id VARCHAR(32) NOT NULL,
    communication_type VARCHAR(50) NOT NULL,
    remote_ip VARCHAR(50),
    webservice_id INTEGER NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (webservice_id) REFERENCES gi_webservice_config(id) ON DELETE CASCADE
);

CREATE INDEX idx_gi_debugger_entry_communication_id ON gi_debugger_entry(communication_id);
CREATE INDEX idx_gi_debugger_entry_webservice_id ON gi_debugger_entry(webservice_id);
CREATE INDEX idx_gi_debugger_entry_create_time ON gi_debugger_entry(create_time);

-- GI Debugger entry content
CREATE TABLE IF NOT EXISTS gi_debugger_entry_content (
    id BIGSERIAL PRIMARY KEY,
    gi_debugger_entry_id BIGINT NOT NULL,
    debug_level VARCHAR(50) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    content TEXT,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (gi_debugger_entry_id) REFERENCES gi_debugger_entry(id) ON DELETE CASCADE
);

CREATE INDEX idx_gi_debugger_entry_content_entry_id ON gi_debugger_entry_content(gi_debugger_entry_id);
CREATE INDEX idx_gi_debugger_entry_content_create_time ON gi_debugger_entry_content(create_time);
CREATE INDEX idx_gi_debugger_entry_content_debug_level ON gi_debugger_entry_content(debug_level);

-- SMIME signer certificate relations
CREATE TABLE IF NOT EXISTS smime_signer_cert_relations (
    id SERIAL PRIMARY KEY,
    cert_hash VARCHAR(8) NOT NULL,
    cert_fingerprint VARCHAR(59) NOT NULL,
    ca_hash VARCHAR(8) NOT NULL,
    ca_fingerprint VARCHAR(59) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_smime_signer_cert_relations_cert ON smime_signer_cert_relations(cert_hash, cert_fingerprint);
CREATE INDEX idx_smime_signer_cert_relations_ca ON smime_signer_cert_relations(ca_hash, ca_fingerprint);