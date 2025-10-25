CREATE TABLE acl (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  description varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  stop_after_match SMALLINT DEFAULT NULL,
  config_match BYTEA,
  config_change BYTEA,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE acl_sync (
  id BIGSERIAL PRIMARY KEY,
  acl_id varchar(200) NOT NULL,
  sync_state varchar(30) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  change_time TIMESTAMP NOT NULL
);

CREATE TABLE acl_ticket_attribute_relations (
  id BIGSERIAL PRIMARY KEY,
  filename varchar(255) NOT NULL,
  attribute_1 varchar(200) NOT NULL,
  attribute_2 varchar(200) NOT NULL,
  acl_data varchar NOT NULL,
  priority BIGINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (filename)
);

CREATE TABLE activity (
  id SERIAL PRIMARY KEY,
  dbcrud_uuid varchar(36) DEFAULT NULL,
  user_id INTEGER NOT NULL,
  activity_type varchar(200) NOT NULL,
  activity_title varchar(255) NOT NULL,
  activity_text TEXT,
  activity_state varchar(255) DEFAULT NULL,
  activity_link varchar(255) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  UNIQUE (dbcrud_uuid)
);

CREATE TABLE translation (
  id SERIAL PRIMARY KEY,
  dbcrud_uuid varchar(36) DEFAULT NULL,
  language_id varchar(5) NOT NULL,
  source_string varchar(1000) NOT NULL,
  destination_string varchar(1000) NOT NULL,
  valid_id SMALLINT NOT NULL DEFAULT 1,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  deployment_state SMALLINT NOT NULL DEFAULT 0,
  UNIQUE (dbcrud_uuid)
);

CREATE TABLE article_color (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  color varchar(10) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE article (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL,
  article_sender_type_id SMALLINT NOT NULL,
  communication_channel_id BIGINT NOT NULL,
  is_visible_for_customer SMALLINT NOT NULL,
  search_index_needs_rebuild SMALLINT NOT NULL DEFAULT '1',
  insert_fingerprint varchar(64) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX article_article_sender_type_id ON article (article_sender_type_id);
CREATE INDEX article_communication_channel_id ON article (communication_channel_id);
CREATE INDEX article_search_index_needs_rebuild ON article (search_index_needs_rebuild);
CREATE INDEX article_ticket_id ON article (ticket_id);
CREATE TABLE article_data_mime (
  id BIGSERIAL PRIMARY KEY,
  article_id BIGINT NOT NULL,
  a_from TEXT,
  a_reply_to TEXT,
  a_to TEXT,
  a_cc TEXT,
  a_bcc TEXT,
  a_subject TEXT,
  a_message_id TEXT,
  a_message_id_md5 varchar(32) DEFAULT NULL,
  a_in_reply_to TEXT,
  a_references TEXT,
  a_content_type varchar(250) DEFAULT NULL,
  a_body TEXT,
  incoming_time INTEGER NOT NULL,
  content_path varchar(250) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX article_data_mime_article_id ON article_data_mime (article_id);
CREATE INDEX article_data_mime_message_id_md5 ON article_data_mime (a_message_id_md5);
CREATE TABLE article_data_mime_attachment (
  id BIGSERIAL PRIMARY KEY,
  article_id BIGINT NOT NULL,
  filename varchar(250) DEFAULT NULL,
  content_size varchar(30) DEFAULT NULL,
  content_type TEXT,
  content_id varchar(250) DEFAULT NULL,
  content_alternative varchar(50) DEFAULT NULL,
  disposition varchar(15) DEFAULT NULL,
  content BYTEA,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX article_data_mime_attachment_article_id ON article_data_mime_attachment (article_id);
CREATE TABLE article_data_mime_plain (
  id BIGSERIAL PRIMARY KEY,
  article_id BIGINT NOT NULL,
  body BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX article_data_mime_plain_article_id ON article_data_mime_plain (article_id);
CREATE TABLE article_data_mime_send_error (
  id BIGSERIAL PRIMARY KEY,
  article_id BIGINT NOT NULL,
  message_id varchar(200) DEFAULT NULL,
  log_message TEXT,
  create_time TIMESTAMP NOT NULL
);

CREATE INDEX article_data_mime_transmission_article_id ON article_data_mime_send_error (article_id);
CREATE INDEX article_data_mime_transmission_message_id ON article_data_mime_send_error (message_id);
CREATE TABLE article_data_otrs_chat (
  id BIGSERIAL PRIMARY KEY,
  article_id BIGINT NOT NULL,
  chat_participant_id varchar(255) NOT NULL,
  chat_participant_name varchar(255) NOT NULL,
  chat_participant_type varchar(255) NOT NULL,
  message_text TEXT,
  system_generated SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL
);

CREATE INDEX article_data_otrs_chat_article_id ON article_data_otrs_chat (article_id);
CREATE TABLE article_flag (
  article_id BIGINT NOT NULL,
  article_key varchar(50) NOT NULL,
  article_value varchar(50) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL
);

CREATE INDEX article_flag_article_id ON article_flag (article_id);
CREATE INDEX article_flag_article_id_create_by ON article_flag (article_id,create_by);
CREATE TABLE article_search_index (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL,
  article_id BIGINT NOT NULL,
  article_key varchar(200) NOT NULL,
  article_value TEXT
);

CREATE INDEX article_search_index_article_id ON article_search_index (article_id,article_key);
CREATE INDEX article_search_index_ticket_id ON article_search_index (ticket_id,article_key);
CREATE TABLE article_sender_type (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE auto_response (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  text0 TEXT,
  text1 TEXT,
  type_id SMALLINT NOT NULL,
  system_address_id SMALLINT NOT NULL,
  content_type varchar(250) DEFAULT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE auto_response_type (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE calendar (
  id BIGSERIAL PRIMARY KEY,
  group_id INTEGER NOT NULL,
  name varchar(200) NOT NULL,
  salt_string varchar(64) NOT NULL,
  color varchar(7) NOT NULL,
  ticket_appointments BYTEA,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE calendar_appointment (
  id BIGSERIAL PRIMARY KEY,
  parent_id BIGINT DEFAULT NULL,
  calendar_id BIGINT NOT NULL,
  unique_id varchar(255) NOT NULL,
  title varchar(255) NOT NULL,
  description TEXT,
  location varchar(255) DEFAULT NULL,
  start_time TIMESTAMP NOT NULL,
  end_time TIMESTAMP NOT NULL,
  all_day SMALLINT DEFAULT NULL,
  notify_time TIMESTAMP DEFAULT NULL,
  notify_template varchar(255) DEFAULT NULL,
  notify_custom varchar(255) DEFAULT NULL,
  notify_custom_unit_count BIGINT DEFAULT NULL,
  notify_custom_unit varchar(255) DEFAULT NULL,
  notify_custom_unit_point varchar(255) DEFAULT NULL,
  notify_custom_date TIMESTAMP DEFAULT NULL,
  team_id TEXT,
  resource_id TEXT,
  recurring SMALLINT DEFAULT NULL,
  recur_type varchar(20) DEFAULT NULL,
  recur_freq varchar(255) DEFAULT NULL,
  recur_count INTEGER DEFAULT NULL,
  recur_interval INTEGER DEFAULT NULL,
  recur_until TIMESTAMP DEFAULT NULL,
  recur_id TIMESTAMP DEFAULT NULL,
  recur_exclude TEXT,
  ticket_appointment_rule_id varchar(32) DEFAULT NULL,
  create_time TIMESTAMP DEFAULT NULL,
  create_by INTEGER DEFAULT NULL,
  change_time TIMESTAMP DEFAULT NULL,
  change_by INTEGER DEFAULT NULL
);

CREATE TABLE calendar_appointment_plugin (
  id SERIAL PRIMARY KEY,
  dbcrud_uuid varchar(36) DEFAULT NULL,
  appointment_id BIGINT NOT NULL,
  plugin_key varchar(1000) NOT NULL,
  config varchar DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (dbcrud_uuid)
);

CREATE TABLE calendar_appointment_ticket (
  calendar_id BIGINT NOT NULL,
  ticket_id BIGINT NOT NULL,
  rule_id varchar(32) NOT NULL,
  appointment_id BIGINT NOT NULL,
  UNIQUE (calendar_id,ticket_id,rule_id)
);

CREATE INDEX calendar_appointment_ticket_appointment_id ON calendar_appointment_ticket (appointment_id);
CREATE INDEX calendar_appointment_ticket_calendar_id ON calendar_appointment_ticket (calendar_id);
CREATE INDEX calendar_appointment_ticket_rule_id ON calendar_appointment_ticket (rule_id);
CREATE INDEX calendar_appointment_ticket_ticket_id ON calendar_appointment_ticket (ticket_id);
CREATE TABLE cloud_service_config (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  config BYTEA NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE communication_channel (
  id BIGSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  module varchar(200) NOT NULL,
  package_name varchar(200) NOT NULL,
  channel_data BYTEA NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE communication_log (
  id BIGSERIAL PRIMARY KEY,
  insert_fingerprint varchar(64) DEFAULT NULL,
  transport varchar(200) NOT NULL,
  direction varchar(200) NOT NULL,
  status varchar(200) NOT NULL,
  account_type varchar(200) DEFAULT NULL,
  account_id varchar(200) DEFAULT NULL,
  start_time TIMESTAMP NOT NULL,
  end_time TIMESTAMP DEFAULT NULL
);

CREATE INDEX communication_direction ON communication_log (direction);
CREATE INDEX communication_start_time ON communication_log (start_time);
CREATE INDEX communication_status ON communication_log (status);
CREATE INDEX communication_transport ON communication_log (transport);
CREATE TABLE communication_log_obj_lookup (
  id BIGSERIAL PRIMARY KEY,
  communication_log_object_id BIGINT NOT NULL,
  object_type varchar(200) NOT NULL,
  object_id BIGINT NOT NULL
);

CREATE INDEX communication_log_obj_lookup_target ON communication_log_obj_lookup (object_type,object_id);
CREATE TABLE communication_log_object (
  id BIGSERIAL PRIMARY KEY,
  insert_fingerprint varchar(64) DEFAULT NULL,
  communication_id BIGINT NOT NULL,
  object_type varchar(50) NOT NULL,
  status varchar(200) NOT NULL,
  start_time TIMESTAMP NOT NULL,
  end_time TIMESTAMP DEFAULT NULL
);

CREATE INDEX communication_log_object_object_type ON communication_log_object (object_type);
CREATE INDEX communication_log_object_status ON communication_log_object (status);
CREATE TABLE communication_log_object_entry (
  id BIGSERIAL PRIMARY KEY,
  communication_log_object_id BIGINT NOT NULL,
  log_key varchar(200) NOT NULL,
  log_value TEXT NOT NULL,
  priority varchar(50) NOT NULL,
  create_time TIMESTAMP NOT NULL
);

CREATE INDEX communication_log_object_entry_key ON communication_log_object_entry (log_key);
CREATE TABLE customer_company (
  customer_id varchar(150) NOT NULL,
  name varchar(200) NOT NULL,
  street varchar(200) DEFAULT NULL,
  zip varchar(200) DEFAULT NULL,
  city varchar(200) DEFAULT NULL,
  country varchar(200) DEFAULT NULL,
  url varchar(200) DEFAULT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  PRIMARY KEY (customer_id),
  UNIQUE (name)
);

CREATE TABLE customer_preferences (
  user_id varchar(250) NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value varchar(250) DEFAULT NULL
);

CREATE INDEX customer_preferences_user_id ON customer_preferences (user_id);
CREATE TABLE customer_user (
  id SERIAL PRIMARY KEY,
  login varchar(200) NOT NULL,
  email varchar(150) NOT NULL,
  customer_id varchar(150) NOT NULL,
  pw varchar(128) DEFAULT NULL,
  title varchar(50) DEFAULT NULL,
  first_name varchar(100) NOT NULL,
  last_name varchar(100) NOT NULL,
  phone varchar(150) DEFAULT NULL,
  fax varchar(150) DEFAULT NULL,
  mobile varchar(150) DEFAULT NULL,
  street varchar(150) DEFAULT NULL,
  zip varchar(200) DEFAULT NULL,
  city varchar(200) DEFAULT NULL,
  country varchar(200) DEFAULT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (login)
);

CREATE TABLE customer_user_customer (
  user_id varchar(100) NOT NULL,
  customer_id varchar(150) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX customer_user_customer_customer_id ON customer_user_customer (customer_id);
CREATE INDEX customer_user_customer_user_id ON customer_user_customer (user_id);
CREATE TABLE dynamic_field (
  id SERIAL PRIMARY KEY,
  internal_field SMALLINT NOT NULL DEFAULT '0',
  name varchar(200) NOT NULL,
  label varchar(200) NOT NULL,
  field_order INTEGER NOT NULL,
  field_type varchar(200) NOT NULL,
  object_type varchar(100) NOT NULL,
  config BYTEA,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE dynamic_field_obj_id_name (
  object_id SERIAL PRIMARY KEY,
  object_name varchar(200) NOT NULL,
  object_type varchar(100) NOT NULL,
  UNIQUE (object_name,object_type)
);

CREATE TABLE dynamic_field_value (
  id SERIAL PRIMARY KEY,
  field_id INTEGER NOT NULL,
  object_id BIGINT NOT NULL,
  value_text TEXT,
  value_date TIMESTAMP DEFAULT NULL,
  value_int BIGINT DEFAULT NULL
);

CREATE INDEX dynamic_field_value_field_values ON dynamic_field_value (object_id,field_id);
CREATE INDEX dynamic_field_value_search_date ON dynamic_field_value (field_id,value_date);
CREATE INDEX dynamic_field_value_search_int ON dynamic_field_value (field_id,value_int);
CREATE INDEX dynamic_field_value_search_text ON dynamic_field_value (field_id,value_text);
CREATE TABLE follow_up_possible (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE form_draft (
  id SERIAL PRIMARY KEY,
  object_type varchar(100) NOT NULL,
  object_id INTEGER NOT NULL,
  action varchar(200) NOT NULL,
  title varchar(255) DEFAULT NULL,
  content BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX form_draft_object_type_object_id_action ON form_draft (object_type,object_id,action);

CREATE TABLE smime_keys (
  id SERIAL PRIMARY KEY,
  key_hash varchar(8) NOT NULL,
  key_type varchar(255) NOT NULL,
  file_name varchar(255) NOT NULL,
  email_address varchar DEFAULT NULL,
  expiration_date TIMESTAMP DEFAULT NULL,
  fingerprint varchar(59) DEFAULT NULL,
  subject varchar DEFAULT NULL,
  create_time TIMESTAMP DEFAULT NULL,
  change_time TIMESTAMP DEFAULT NULL,
  create_by INTEGER DEFAULT NULL,
  change_by INTEGER DEFAULT NULL
);

CREATE INDEX smime_keys_file_name ON smime_keys (file_name);
CREATE INDEX smime_keys_key_hash ON smime_keys (key_hash);
CREATE INDEX smime_keys_key_type ON smime_keys (key_type);

CREATE TABLE oauth2_token_config (
  id SERIAL PRIMARY KEY,
  dbcrud_uuid varchar(36) DEFAULT NULL,
  name varchar(250) NOT NULL,
  config varchar NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name),
  UNIQUE (dbcrud_uuid)
);

CREATE TABLE oauth2_token (
  id SERIAL PRIMARY KEY,
  dbcrud_uuid varchar(36) DEFAULT NULL,
  token_config_id INTEGER NOT NULL,
  authorization_code varchar DEFAULT NULL,
  token varchar DEFAULT NULL,
  token_expiration_date TIMESTAMP DEFAULT NULL,
  refresh_token varchar DEFAULT NULL,
  refresh_token_expiration_date TIMESTAMP DEFAULT NULL,
  error_message varchar DEFAULT NULL,
  error_description varchar DEFAULT NULL,
  error_code varchar(1000) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (token_config_id),
  UNIQUE (dbcrud_uuid)
);

CREATE TABLE mention (
  id SERIAL PRIMARY KEY,
  user_id INTEGER DEFAULT NULL,
  ticket_id INTEGER DEFAULT NULL,
  article_id INTEGER DEFAULT NULL,
  create_time TIMESTAMP DEFAULT NULL
);

CREATE INDEX mention_article_id ON mention (article_id);
CREATE INDEX mention_ticket_id ON mention (ticket_id);
CREATE INDEX mention_user_id ON mention (user_id);

CREATE TABLE generic_agent_jobs (
  job_name varchar(200) NOT NULL,
  job_key varchar(200) NOT NULL,
  job_value varchar(200) DEFAULT NULL
);

CREATE INDEX generic_agent_jobs_job_name ON generic_agent_jobs (job_name);
CREATE TABLE gi_debugger_entry (
  id BIGSERIAL PRIMARY KEY,
  communication_id varchar(32) NOT NULL,
  communication_type varchar(50) NOT NULL,
  remote_ip varchar(50) DEFAULT NULL,
  webservice_id INTEGER NOT NULL,
  create_time TIMESTAMP NOT NULL,
  UNIQUE (communication_id)
);

CREATE INDEX gi_debugger_entry_create_time ON gi_debugger_entry (create_time);
CREATE TABLE gi_debugger_entry_content (
  id BIGSERIAL PRIMARY KEY,
  gi_debugger_entry_id BIGINT NOT NULL,
  debug_level varchar(50) NOT NULL,
  subject varchar(255) NOT NULL,
  content BYTEA,
  create_time TIMESTAMP NOT NULL
);

CREATE INDEX gi_debugger_entry_content_create_time ON gi_debugger_entry_content (create_time);
CREATE INDEX gi_debugger_entry_content_debug_level ON gi_debugger_entry_content (debug_level);
CREATE TABLE gi_webservice_config (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  config BYTEA NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE gi_webservice_config_history (
  id BIGSERIAL PRIMARY KEY,
  config_id INTEGER NOT NULL,
  config BYTEA NOT NULL,
  config_md5 varchar(32) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (config_md5)
);

CREATE TABLE group_customer (
  customer_id varchar(150) NOT NULL,
  group_id INTEGER NOT NULL,
  permission_key varchar(20) NOT NULL,
  permission_value SMALLINT NOT NULL,
  permission_context varchar(100) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX group_customer_customer_id ON group_customer (customer_id);
CREATE INDEX group_customer_group_id ON group_customer (group_id);
CREATE TABLE group_customer_user (
  user_id varchar(100) NOT NULL,
  group_id INTEGER NOT NULL,
  permission_key varchar(20) NOT NULL,
  permission_value SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX group_customer_user_group_id ON group_customer_user (group_id);
CREATE INDEX group_customer_user_user_id ON group_customer_user (user_id);
CREATE TABLE group_role (
  role_id INTEGER NOT NULL,
  group_id INTEGER NOT NULL,
  permission_key varchar(20) NOT NULL,
  permission_value SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX group_role_group_id ON group_role (group_id);
CREATE INDEX group_role_role_id ON group_role (role_id);
CREATE TABLE permission_groups (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE group_user (
  user_id INTEGER NOT NULL,
  group_id INTEGER NOT NULL,
  permission_key varchar(20) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX group_user_group_id ON group_user (group_id);
CREATE INDEX group_user_user_id ON group_user (user_id);
CREATE TABLE groups (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE link_object (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(100) NOT NULL,
  UNIQUE (name)
);

CREATE TABLE link_relation (
  source_object_id SMALLINT NOT NULL,
  source_key varchar(50) NOT NULL,
  target_object_id SMALLINT NOT NULL,
  target_key varchar(50) NOT NULL,
  type_id SMALLINT NOT NULL,
  state_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  UNIQUE (source_object_id,source_key,target_object_id,target_key,type_id)
);

CREATE INDEX link_relation_list_source ON link_relation (source_object_id,source_key,state_id);
CREATE INDEX link_relation_list_target ON link_relation (target_object_id,target_key,state_id);
CREATE TABLE link_state (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(50) NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE link_type (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(50) NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE mail_account (
  id SERIAL PRIMARY KEY,
  login varchar(200) NOT NULL,
  pw varchar(200) NOT NULL,
  host varchar(200) NOT NULL,
  account_type varchar(20) NOT NULL,
  queue_id INTEGER NOT NULL,
  trusted SMALLINT NOT NULL,
  imap_folder varchar(250) DEFAULT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE mail_queue (
  id BIGSERIAL PRIMARY KEY,
  insert_fingerprint varchar(64) DEFAULT NULL,
  article_id BIGINT DEFAULT NULL,
  attempts INTEGER NOT NULL,
  sender varchar(200) DEFAULT NULL,
  recipient TEXT NOT NULL,
  raw_message BYTEA NOT NULL,
  due_time TIMESTAMP DEFAULT NULL,
  last_smtp_code INTEGER DEFAULT NULL,
  last_smtp_message TEXT,
  create_time TIMESTAMP NOT NULL,
  UNIQUE (article_id),
  UNIQUE (insert_fingerprint)
);

CREATE INDEX mail_queue_attempts ON mail_queue (attempts);
CREATE TABLE notification_event (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  valid_id SMALLINT NOT NULL,
  comments varchar(250) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE notification_event_item (
  notification_id INTEGER NOT NULL,
  event_key varchar(200) NOT NULL,
  event_value varchar(200) NOT NULL
);

CREATE INDEX notification_event_item_event_key ON notification_event_item (event_key);
CREATE INDEX notification_event_item_event_value ON notification_event_item (event_value);
CREATE INDEX notification_event_item_notification_id ON notification_event_item (notification_id);
CREATE TABLE notification_event_message (
  id SERIAL PRIMARY KEY,
  notification_id INTEGER NOT NULL,
  subject varchar(200) NOT NULL,
  TEXT TEXT NOT NULL,
  content_type varchar(250) NOT NULL,
  language varchar(60) NOT NULL,
  UNIQUE (notification_id,language)
);

CREATE INDEX notification_event_message_language ON notification_event_message (language);
CREATE INDEX notification_event_message_notification_id ON notification_event_message (notification_id);
CREATE TABLE package_repository (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  version varchar(250) NOT NULL,
  vendor varchar(250) NOT NULL,
  install_status varchar(250) NOT NULL,
  filename varchar(250) DEFAULT NULL,
  content_type varchar(250) DEFAULT NULL,
  content BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE personal_queues (
  user_id INTEGER NOT NULL,
  queue_id INTEGER NOT NULL
);

CREATE INDEX personal_queues_queue_id ON personal_queues (queue_id);
CREATE INDEX personal_queues_user_id ON personal_queues (user_id);
CREATE TABLE personal_services (
  user_id INTEGER NOT NULL,
  service_id INTEGER NOT NULL
);

CREATE INDEX personal_services_service_id ON personal_services (service_id);
CREATE INDEX personal_services_user_id ON personal_services (user_id);
CREATE TABLE pm_activity (
  id SERIAL PRIMARY KEY,
  entity_id varchar(50) NOT NULL,
  name varchar(200) NOT NULL,
  config BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (entity_id)
);

CREATE TABLE pm_activity_dialog (
  id SERIAL PRIMARY KEY,
  entity_id varchar(50) NOT NULL,
  name varchar(200) NOT NULL,
  config BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (entity_id)
);

CREATE TABLE pm_entity_sync (
  entity_type varchar(30) NOT NULL,
  entity_id varchar(50) NOT NULL,
  sync_state varchar(30) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  change_time TIMESTAMP NOT NULL,
  UNIQUE (entity_type,entity_id)
);

CREATE TABLE pm_process (
  id SERIAL PRIMARY KEY,
  entity_id varchar(50) NOT NULL,
  name varchar(200) NOT NULL,
  state_entity_id varchar(50) NOT NULL,
  layout BYTEA NOT NULL,
  config BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (entity_id)
);

CREATE TABLE pm_process_preferences (
  process_entity_id varchar(50) NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value varchar(3000) DEFAULT NULL
);

CREATE INDEX pm_process_preferences_process_entity_id ON pm_process_preferences (process_entity_id);

CREATE TABLE pm_transition (
  id SERIAL PRIMARY KEY,
  entity_id varchar(50) NOT NULL,
  name varchar(200) NOT NULL,
  config BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (entity_id)
);

CREATE TABLE pm_transition_action (
  id SERIAL PRIMARY KEY,
  entity_id varchar(50) NOT NULL,
  name varchar(200) NOT NULL,
  config BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (entity_id)
);

CREATE TABLE postmaster_filter (
  f_name varchar(200) NOT NULL,
  f_stop SMALLINT DEFAULT NULL,
  f_type varchar(20) NOT NULL,
  f_key varchar(200) NOT NULL,
  f_value varchar(200) NOT NULL,
  f_not SMALLINT DEFAULT NULL
);

CREATE INDEX postmaster_filter_f_name ON postmaster_filter (f_name);
CREATE TABLE process_id (
  process_name varchar(200) NOT NULL,
  process_id varchar(200) NOT NULL,
  process_host varchar(200) NOT NULL,
  process_create INTEGER NOT NULL,
  process_change INTEGER NOT NULL
);

CREATE TABLE queue (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  group_id INTEGER NOT NULL,
  unlock_timeout INTEGER DEFAULT NULL,
  first_response_time INTEGER DEFAULT NULL,
  first_response_notify SMALLINT DEFAULT NULL,
  update_time INTEGER DEFAULT NULL,
  update_notify SMALLINT DEFAULT NULL,
  solution_time INTEGER DEFAULT NULL,
  solution_notify SMALLINT DEFAULT NULL,
  system_address_id SMALLINT NOT NULL,
  calendar_name varchar(100) DEFAULT NULL,
  default_sign_key varchar(100) DEFAULT NULL,
  salutation_id SMALLINT NOT NULL,
  signature_id SMALLINT NOT NULL,
  follow_up_id SMALLINT NOT NULL,
  follow_up_lock SMALLINT NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE INDEX queue_group_id ON queue (group_id);
CREATE TABLE queue_auto_response (
  id SERIAL PRIMARY KEY,
  queue_id INTEGER NOT NULL,
  auto_response_id INTEGER NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE queue_preferences (
  queue_id INTEGER NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value varchar(250) DEFAULT NULL
);

CREATE INDEX queue_preferences_queue_id ON queue_preferences (queue_id);
CREATE TABLE queue_standard_template (
  queue_id INTEGER NOT NULL,
  standard_template_id INTEGER NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE role_user (
  user_id INTEGER NOT NULL,
  role_id INTEGER NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX role_user_role_id ON role_user (role_id);
CREATE INDEX role_user_user_id ON role_user (user_id);
CREATE TABLE roles (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE salutation (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  TEXT TEXT NOT NULL,
  content_type varchar(250) DEFAULT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE scheduler_future_task (
  id BIGSERIAL PRIMARY KEY,
  ident BIGINT NOT NULL,
  execution_time TIMESTAMP NOT NULL,
  name varchar(150) DEFAULT NULL,
  task_type varchar(150) NOT NULL,
  task_data BYTEA NOT NULL,
  attempts SMALLINT NOT NULL,
  lock_key BIGINT NOT NULL,
  lock_time TIMESTAMP DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  UNIQUE (ident)
);

CREATE INDEX scheduler_future_task_ident_id ON scheduler_future_task (ident,id);
CREATE INDEX scheduler_future_task_lock_key_id ON scheduler_future_task (lock_key,id);
CREATE TABLE scheduler_recurrent_task (
  id BIGSERIAL PRIMARY KEY,
  name varchar(150) NOT NULL,
  task_type varchar(150) NOT NULL,
  last_execution_time TIMESTAMP NOT NULL,
  last_worker_task_id BIGINT DEFAULT NULL,
  last_worker_status SMALLINT DEFAULT NULL,
  last_worker_running_time INTEGER DEFAULT NULL,
  lock_key BIGINT NOT NULL,
  lock_time TIMESTAMP DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  change_time TIMESTAMP NOT NULL,
  UNIQUE (name,task_type)
);

CREATE INDEX scheduler_recurrent_task_lock_key_id ON scheduler_recurrent_task (lock_key,id);
CREATE INDEX scheduler_recurrent_task_task_type_name ON scheduler_recurrent_task (task_type,name);
CREATE TABLE scheduler_task (
  id BIGSERIAL PRIMARY KEY,
  ident BIGINT NOT NULL,
  name varchar(150) DEFAULT NULL,
  task_type varchar(150) NOT NULL,
  task_data BYTEA NOT NULL,
  attempts SMALLINT NOT NULL,
  lock_key BIGINT NOT NULL,
  lock_time TIMESTAMP DEFAULT NULL,
  lock_update_time TIMESTAMP DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  UNIQUE (ident)
);

CREATE INDEX scheduler_task_ident_id ON scheduler_task (ident,id);
CREATE INDEX scheduler_task_lock_key_id ON scheduler_task (lock_key,id);
CREATE TABLE search_profile (
  login varchar(200) NOT NULL,
  profile_name varchar(200) NOT NULL,
  profile_type varchar(30) NOT NULL,
  profile_key varchar(200) NOT NULL,
  profile_value varchar(200) DEFAULT NULL
);

CREATE INDEX search_profile_login ON search_profile (login);
CREATE INDEX search_profile_profile_name ON search_profile (profile_name);
CREATE TABLE service (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  valid_id SMALLINT NOT NULL,
  comments varchar(250) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE service_customer_user (
  customer_user_login varchar(200) NOT NULL,
  service_id INTEGER NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL
);

CREATE INDEX service_customer_user_customer_user_login ON service_customer_user (customer_user_login);
CREATE INDEX service_customer_user_service_id ON service_customer_user (service_id);
CREATE TABLE service_preferences (
  service_id INTEGER NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value varchar(250) DEFAULT NULL
);

CREATE INDEX service_preferences_service_id ON service_preferences (service_id);
CREATE TABLE service_sla (
  service_id INTEGER NOT NULL,
  sla_id INTEGER NOT NULL,
  UNIQUE (service_id,sla_id)
);

CREATE TABLE sessions (
  id BIGSERIAL PRIMARY KEY,
  session_id varchar(100) NOT NULL,
  data_key varchar(100) NOT NULL,
  data_value TEXT,
  serialized SMALLINT NOT NULL
);

CREATE INDEX sessions_data_key ON sessions (data_key);
CREATE INDEX sessions_session_id_data_key ON sessions (session_id,data_key);
CREATE TABLE signature (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  TEXT TEXT NOT NULL,
  content_type varchar(250) DEFAULT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE sla (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  calendar_name varchar(100) DEFAULT NULL,
  first_response_time INTEGER NOT NULL,
  first_response_notify SMALLINT DEFAULT NULL,
  update_time INTEGER NOT NULL,
  update_notify SMALLINT DEFAULT NULL,
  solution_time INTEGER NOT NULL,
  solution_notify SMALLINT DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  comments varchar(250) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE sla_preferences (
  sla_id INTEGER NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value varchar(250) DEFAULT NULL
);

CREATE INDEX sla_preferences_sla_id ON sla_preferences (sla_id);
CREATE TABLE smime_signer_cert_relations (
  id SERIAL PRIMARY KEY,
  cert_hash varchar(8) NOT NULL,
  cert_fingerprint varchar(59) NOT NULL,
  ca_hash varchar(8) NOT NULL,
  ca_fingerprint varchar(59) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE standard_attachment (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  content_type varchar(250) NOT NULL,
  content BYTEA NOT NULL,
  filename varchar(250) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE standard_template (
  id SERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  TEXT TEXT,
  content_type varchar(250) DEFAULT NULL,
  template_type varchar(100) NOT NULL DEFAULT 'Answer',
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE standard_template_attachment (
  id SERIAL PRIMARY KEY,
  standard_attachment_id INTEGER NOT NULL,
  standard_template_id INTEGER NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE sysconfig_default (
  id SERIAL PRIMARY KEY,
  name varchar(250) NOT NULL,
  description BYTEA NOT NULL,
  navigation varchar(200) NOT NULL,
  is_invisible SMALLINT NOT NULL,
  is_readonly SMALLINT NOT NULL,
  is_required SMALLINT NOT NULL,
  is_valid SMALLINT NOT NULL,
  has_configlevel SMALLINT NOT NULL,
  user_modification_possible SMALLINT NOT NULL,
  user_modification_active SMALLINT NOT NULL,
  user_preferences_group varchar(250) DEFAULT NULL,
  xml_content_raw BYTEA NOT NULL,
  xml_content_parsed BYTEA NOT NULL,
  xml_filename varchar(250) NOT NULL,
  effective_value BYTEA NOT NULL,
  is_dirty SMALLINT NOT NULL,
  exclusive_lock_guid varchar(32) NOT NULL,
  exclusive_lock_user_id INTEGER DEFAULT NULL,
  exclusive_lock_expiry_time TIMESTAMP DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE sysconfig_default_version (
  id SERIAL PRIMARY KEY,
  sysconfig_default_id INTEGER DEFAULT NULL,
  name varchar(250) NOT NULL,
  description BYTEA NOT NULL,
  navigation varchar(200) NOT NULL,
  is_invisible SMALLINT NOT NULL,
  is_readonly SMALLINT NOT NULL,
  is_required SMALLINT NOT NULL,
  is_valid SMALLINT NOT NULL,
  has_configlevel SMALLINT NOT NULL,
  user_modification_possible SMALLINT NOT NULL,
  user_modification_active SMALLINT NOT NULL,
  user_preferences_group varchar(250) DEFAULT NULL,
  xml_content_raw BYTEA NOT NULL,
  xml_content_parsed BYTEA NOT NULL,
  xml_filename varchar(250) NOT NULL,
  effective_value BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX scfv_sysconfig_default_id_name ON sysconfig_default_version (sysconfig_default_id,name);
CREATE TABLE sysconfig_deployment (
  id SERIAL PRIMARY KEY,
  comments varchar(250) DEFAULT NULL,
  user_id INTEGER DEFAULT NULL,
  effective_value BYTEA NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL
);

CREATE TABLE sysconfig_deployment_lock (
  id SERIAL PRIMARY KEY,
  exclusive_lock_guid varchar(32) DEFAULT NULL,
  exclusive_lock_user_id INTEGER DEFAULT NULL,
  exclusive_lock_expiry_time TIMESTAMP DEFAULT NULL
);

CREATE TABLE sysconfig_modified (
  id SERIAL PRIMARY KEY,
  sysconfig_default_id INTEGER NOT NULL,
  name varchar(250) NOT NULL,
  user_id INTEGER DEFAULT NULL,
  is_valid SMALLINT NOT NULL,
  user_modification_active SMALLINT NOT NULL,
  effective_value BYTEA NOT NULL,
  is_dirty SMALLINT NOT NULL,
  reset_to_default SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (sysconfig_default_id,user_id)
);

CREATE TABLE sysconfig_modified_version (
  id SERIAL PRIMARY KEY,
  sysconfig_default_version_id INTEGER NOT NULL,
  name varchar(250) NOT NULL,
  user_id INTEGER DEFAULT NULL,
  is_valid SMALLINT NOT NULL,
  user_modification_active SMALLINT NOT NULL,
  effective_value BYTEA NOT NULL,
  reset_to_default SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE system_address (
  id SMALLSERIAL PRIMARY KEY,
  value0 varchar(200) NOT NULL,
  value1 varchar(200) NOT NULL,
  value2 varchar(200) DEFAULT NULL,
  value3 varchar(200) DEFAULT NULL,
  queue_id INTEGER NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE system_data (
  data_key varchar(160) NOT NULL,
  data_value BYTEA,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  PRIMARY KEY (data_key)
);

CREATE TABLE system_maintenance (
  id SERIAL PRIMARY KEY,
  start_date INTEGER NOT NULL,
  stop_date INTEGER NOT NULL,
  comments varchar(250) NOT NULL,
  login_message varchar(250) DEFAULT NULL,
  show_login_message SMALLINT DEFAULT NULL,
  notify_message varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE TABLE ticket (
  id BIGSERIAL PRIMARY KEY,
  tn varchar(50) NOT NULL,
  title varchar(255) DEFAULT NULL,
  queue_id INTEGER NOT NULL,
  ticket_lock_id SMALLINT NOT NULL,
  type_id SMALLINT DEFAULT NULL,
  service_id INTEGER DEFAULT NULL,
  sla_id INTEGER DEFAULT NULL,
  user_id INTEGER NOT NULL,
  responsible_user_id INTEGER NOT NULL,
  ticket_priority_id SMALLINT NOT NULL,
  ticket_state_id SMALLINT NOT NULL,
  customer_id varchar(150) DEFAULT NULL,
  customer_user_id varchar(250) DEFAULT NULL,
  timeout INTEGER NOT NULL,
  until_time INTEGER NOT NULL,
  escalation_time INTEGER NOT NULL,
  escalation_update_time INTEGER NOT NULL,
  escalation_response_time INTEGER NOT NULL,
  escalation_solution_time INTEGER NOT NULL,
  archive_flag SMALLINT NOT NULL DEFAULT '0',
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (tn)
);

CREATE INDEX ticket_archive_flag ON ticket (archive_flag);
CREATE INDEX ticket_create_time ON ticket (create_time);
CREATE INDEX ticket_customer_id ON ticket (customer_id);
CREATE INDEX ticket_customer_user_id ON ticket (customer_user_id);
CREATE INDEX ticket_escalation_response_time ON ticket (escalation_response_time);
CREATE INDEX ticket_escalation_solution_time ON ticket (escalation_solution_time);
CREATE INDEX ticket_escalation_time ON ticket (escalation_time);
CREATE INDEX ticket_escalation_update_time ON ticket (escalation_update_time);
CREATE INDEX ticket_queue_id ON ticket (queue_id);
CREATE INDEX ticket_queue_view ON ticket (ticket_state_id,ticket_lock_id);
CREATE INDEX ticket_responsible_user_id ON ticket (responsible_user_id);
CREATE INDEX ticket_ticket_lock_id ON ticket (ticket_lock_id);
CREATE INDEX ticket_ticket_priority_id ON ticket (ticket_priority_id);
CREATE INDEX ticket_ticket_state_id ON ticket (ticket_state_id);
CREATE INDEX ticket_timeout ON ticket (timeout);
CREATE INDEX ticket_title ON ticket (title);
CREATE INDEX ticket_type_id ON ticket (type_id);
CREATE INDEX ticket_until_time ON ticket (until_time);
CREATE INDEX ticket_user_id ON ticket (user_id);
CREATE TABLE ticket_flag (
  ticket_id BIGINT NOT NULL,
  ticket_key varchar(50) NOT NULL,
  ticket_value varchar(50) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  UNIQUE (ticket_id,ticket_key,create_by)
);

CREATE INDEX ticket_flag_ticket_id ON ticket_flag (ticket_id);
CREATE INDEX ticket_flag_ticket_id_create_by ON ticket_flag (ticket_id,create_by);
CREATE INDEX ticket_flag_ticket_id_ticket_key ON ticket_flag (ticket_id,ticket_key);
CREATE TABLE ticket_history (
  id BIGSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  history_type_id SMALLINT NOT NULL,
  ticket_id BIGINT NOT NULL,
  article_id BIGINT DEFAULT NULL,
  type_id SMALLINT NOT NULL,
  queue_id INTEGER NOT NULL,
  owner_id INTEGER NOT NULL,
  priority_id SMALLINT NOT NULL,
  state_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX ticket_history_article_id ON ticket_history (article_id);
CREATE INDEX ticket_history_create_time ON ticket_history (create_time);
CREATE INDEX ticket_history_history_type_id ON ticket_history (history_type_id);
CREATE INDEX ticket_history_owner_id ON ticket_history (owner_id);
CREATE INDEX ticket_history_priority_id ON ticket_history (priority_id);
CREATE INDEX ticket_history_queue_id ON ticket_history (queue_id);
CREATE INDEX ticket_history_state_id ON ticket_history (state_id);
CREATE INDEX ticket_history_ticket_id ON ticket_history (ticket_id);
CREATE INDEX ticket_history_type_id ON ticket_history (type_id);
CREATE TABLE ticket_history_type (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE ticket_index (
  ticket_id BIGINT NOT NULL,
  queue_id INTEGER NOT NULL,
  queue varchar(200) NOT NULL,
  group_id INTEGER NOT NULL,
  s_lock varchar(200) NOT NULL,
  s_state varchar(200) NOT NULL,
  create_time TIMESTAMP NOT NULL
);

CREATE INDEX ticket_index_group_id ON ticket_index (group_id);
CREATE INDEX ticket_index_queue_id ON ticket_index (queue_id);
CREATE INDEX ticket_index_ticket_id ON ticket_index (ticket_id);
CREATE TABLE ticket_lock_index (
  ticket_id BIGINT NOT NULL
);

CREATE INDEX ticket_lock_index_ticket_id ON ticket_lock_index (ticket_id);
CREATE TABLE ticket_lock_type (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE ticket_loop_protection (
  sent_to varchar(250) NOT NULL,
  sent_date varchar(150) NOT NULL
);

CREATE INDEX ticket_loop_protection_sent_date ON ticket_loop_protection (sent_date);
CREATE INDEX ticket_loop_protection_sent_to ON ticket_loop_protection (sent_to);
CREATE TABLE ticket_number_counter (
  id BIGSERIAL PRIMARY KEY,
  counter BIGINT NOT NULL,
  counter_uid varchar(32) NOT NULL,
  create_time TIMESTAMP DEFAULT NULL,
  UNIQUE (counter_uid)
);

CREATE INDEX ticket_number_counter_create_time ON ticket_number_counter (create_time);
CREATE TABLE ticket_priority (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  valid_id SMALLINT NOT NULL,
  color varchar(25) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE ticket_state (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  type_id SMALLINT NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE ticket_state_type (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  comments varchar(250) DEFAULT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE ticket_type (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE ticket_watcher (
  ticket_id BIGINT NOT NULL,
  user_id INTEGER NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX ticket_watcher_ticket_id ON ticket_watcher (ticket_id);
CREATE INDEX ticket_watcher_user_id ON ticket_watcher (user_id);
CREATE TABLE time_accounting (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL,
  article_id BIGINT DEFAULT NULL,
  time_unit decimal(10,2) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL
);

CREATE INDEX time_accounting_ticket_id ON time_accounting (ticket_id);
CREATE TABLE user_preferences (
  user_id INTEGER NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value BYTEA
);

CREATE INDEX user_preferences_user_id ON user_preferences (user_id);
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  login varchar(200) NOT NULL,
  pw varchar(128) NOT NULL,
  title varchar(50) DEFAULT NULL,
  first_name varchar(100) NOT NULL,
  last_name varchar(100) NOT NULL,
  valid_id SMALLINT NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (login)
);

CREATE TABLE valid (
  id SMALLSERIAL PRIMARY KEY,
  name varchar(200) NOT NULL,
  create_time TIMESTAMP NOT NULL,
  create_by INTEGER NOT NULL,
  change_time TIMESTAMP NOT NULL,
  change_by INTEGER NOT NULL,
  UNIQUE (name)
);

CREATE TABLE virtual_fs (
  id BIGSERIAL PRIMARY KEY,
  filename TEXT NOT NULL,
  backend varchar(60) NOT NULL,
  backend_key varchar(160) NOT NULL,
  create_time TIMESTAMP NOT NULL
);

CREATE INDEX virtual_fs_backend ON virtual_fs (backend);
CREATE INDEX virtual_fs_filename ON virtual_fs (filename);
CREATE TABLE virtual_fs_db (
  id BIGSERIAL PRIMARY KEY,
  filename TEXT NOT NULL,
  content BYTEA,
  create_time TIMESTAMP NOT NULL
);

CREATE INDEX virtual_fs_db_filename ON virtual_fs_db (filename);
CREATE TABLE virtual_fs_preferences (
  virtual_fs_id BIGINT NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value TEXT
);

CREATE INDEX virtual_fs_preferences_key_value ON virtual_fs_preferences (preferences_key,preferences_value);
CREATE INDEX virtual_fs_preferences_virtual_fs_id ON virtual_fs_preferences (virtual_fs_id);
CREATE TABLE web_upload_cache (
  form_id varchar(250) DEFAULT NULL,
  filename varchar(250) DEFAULT NULL,
  content_id varchar(250) DEFAULT NULL,
  content_size varchar(30) DEFAULT NULL,
  content_type varchar(250) DEFAULT NULL,
  disposition varchar(15) DEFAULT NULL,
  content BYTEA NOT NULL,
  create_time_unix BIGINT NOT NULL
);

CREATE TABLE xml_storage (
  xml_type varchar(200) NOT NULL,
  xml_key varchar(250) NOT NULL,
  xml_content_key varchar(250) NOT NULL,
  xml_content_value TEXT
);

CREATE INDEX xml_storage_key_type ON xml_storage (xml_key);
CREATE INDEX xml_storage_xml_content_key ON xml_storage (xml_content_key);