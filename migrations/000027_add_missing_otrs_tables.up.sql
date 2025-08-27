-- Add missing OTRS tables that have data in the dump
-- This expands GOTRS schema compatibility with full OTRS installations

-- Core lookup tables
CREATE TABLE IF NOT EXISTS valid (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT valid_pkey PRIMARY KEY (id),
  CONSTRAINT valid_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS salutation (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  text text,
  content_type varchar(250),
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT salutation_pkey PRIMARY KEY (id),
  CONSTRAINT salutation_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS signature (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  text text,
  content_type varchar(250),
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT signature_pkey PRIMARY KEY (id),
  CONSTRAINT signature_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS follow_up_possible (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT follow_up_possible_pkey PRIMARY KEY (id),
  CONSTRAINT follow_up_possible_name_key UNIQUE (name)
);

-- Auto response system
CREATE TABLE IF NOT EXISTS auto_response_type (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT auto_response_type_pkey PRIMARY KEY (id),
  CONSTRAINT auto_response_type_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS auto_response (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  text0 text,
  text1 text,
  text2 text,
  type_id smallint NOT NULL,
  system_address_id smallint NOT NULL,
  charset varchar(80) NOT NULL,
  content_type varchar(250),
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT auto_response_pkey PRIMARY KEY (id),
  CONSTRAINT auto_response_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS system_address (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  value0 varchar(200) NOT NULL,
  value1 varchar(200) NOT NULL,
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT system_address_pkey PRIMARY KEY (id)
);

-- Article and ticket extensions
CREATE TABLE IF NOT EXISTS article_flag (
  article_id bigint NOT NULL,
  article_key varchar(50) NOT NULL,
  article_value varchar(50),
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  CONSTRAINT article_flag_article_id_fkey FOREIGN KEY (article_id) REFERENCES article(id)
);

CREATE TABLE IF NOT EXISTS ticket_flag (
  ticket_id bigint NOT NULL,
  ticket_key varchar(50) NOT NULL,
  ticket_value varchar(50),
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  CONSTRAINT ticket_flag_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES ticket(id)
);

CREATE TABLE IF NOT EXISTS article_data_mime_plain (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  article_id bigint NOT NULL,
  body text,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT article_data_mime_plain_pkey PRIMARY KEY (id),
  CONSTRAINT article_data_mime_plain_article_id_fkey FOREIGN KEY (article_id) REFERENCES article(id)
);

CREATE TABLE IF NOT EXISTS article_data_mime_send_error (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  article_id bigint NOT NULL,
  message_id varchar(3800),
  log_message text,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT article_data_mime_send_error_pkey PRIMARY KEY (id),
  CONSTRAINT article_data_mime_send_error_article_id_fkey FOREIGN KEY (article_id) REFERENCES article(id)
);

-- Ticket counters and search
CREATE TABLE IF NOT EXISTS ticket_number_counter (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  counter bigint NOT NULL,
  content_path text,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT ticket_number_counter_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS article_search_index (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  ticket_id bigint NOT NULL,
  article_id bigint NOT NULL,
  article_key varchar(200) NOT NULL,
  article_value text,
  CONSTRAINT article_search_index_pkey PRIMARY KEY (id),
  CONSTRAINT article_search_index_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES ticket(id),
  CONSTRAINT article_search_index_article_id_fkey FOREIGN KEY (article_id) REFERENCES article(id)
);

-- Customer relationships
CREATE TABLE IF NOT EXISTS customer_preferences (
  user_id varchar(250) NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value text
);

CREATE TABLE IF NOT EXISTS group_customer_user (
  user_id varchar(100) NOT NULL,
  group_id int NOT NULL,
  permission_key varchar(20) NOT NULL,
  permission_value smallint NOT NULL DEFAULT 0,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT group_customer_user_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups(id)
);

CREATE TABLE IF NOT EXISTS group_customer (
  customer_id varchar(150) NOT NULL,
  group_id int NOT NULL,
  permission_key varchar(20) NOT NULL,
  permission_value smallint NOT NULL DEFAULT 0,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT group_customer_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups(id)
);

-- Templates and attachments
CREATE TABLE IF NOT EXISTS standard_attachment (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(250) NOT NULL,
  content_type varchar(250) NOT NULL,
  content bytea NOT NULL,
  filename varchar(250) NOT NULL,
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT standard_attachment_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS standard_template (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  text text,
  content_type varchar(250),
  template_type varchar(100) NOT NULL DEFAULT 'Answer',
  comments varchar(250),
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT standard_template_pkey PRIMARY KEY (id),
  CONSTRAINT standard_template_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS queue_standard_template (
  queue_id int NOT NULL,
  standard_template_id int NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT queue_standard_template_queue_id_fkey FOREIGN KEY (queue_id) REFERENCES queue(id),
  CONSTRAINT queue_standard_template_standard_template_id_fkey FOREIGN KEY (standard_template_id) REFERENCES standard_template(id)
);

-- User preferences
CREATE TABLE IF NOT EXISTS user_preferences (
  user_id int NOT NULL,
  preferences_key varchar(150) NOT NULL,
  preferences_value text,
  CONSTRAINT user_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Time accounting
CREATE TABLE IF NOT EXISTS time_accounting (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  ticket_id bigint NOT NULL,
  article_id bigint,
  time_unit decimal(10,2) NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT time_accounting_pkey PRIMARY KEY (id),
  CONSTRAINT time_accounting_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES ticket(id),
  CONSTRAINT time_accounting_article_id_fkey FOREIGN KEY (article_id) REFERENCES article(id)
);

-- Communication system
CREATE TABLE IF NOT EXISTS communication_channel (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  module varchar(200) NOT NULL,
  package_name varchar(200),
  channel_data text,
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT communication_channel_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS communication_log (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  transport varchar(200) NOT NULL,
  direction varchar(200) NOT NULL,
  status varchar(200) NOT NULL,
  account_type varchar(200),
  account_id varchar(200),
  object_log_type varchar(200),
  object_log_id varchar(200),
  communication_id varchar(32),
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT communication_log_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS communication_log_object (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  communication_id varchar(200) NOT NULL,
  object_type varchar(200) NOT NULL,
  status varchar(200) NOT NULL
);

CREATE TABLE IF NOT EXISTS communication_log_object_entry (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  communication_log_object_id bigint NOT NULL,
  log_key varchar(200) NOT NULL,
  log_value text,
  priority varchar(50) NOT NULL DEFAULT 'info'
);

CREATE TABLE IF NOT EXISTS communication_log_obj_lookup (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  communication_log_object_id bigint NOT NULL,
  object_type varchar(200) NOT NULL,
  object_id varchar(200) NOT NULL
);

-- Notification system
CREATE TABLE IF NOT EXISTS notification_event (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  subject varchar(200),
  text text,
  content_type varchar(250),
  charset varchar(100),
  valid_id smallint NOT NULL DEFAULT 1,
  comments varchar(250),
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT notification_event_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS notification_event_item (
  notification_id int NOT NULL,
  event_key varchar(200) NOT NULL,
  event_value varchar(200) NOT NULL,
  CONSTRAINT notification_event_item_notification_id_fkey FOREIGN KEY (notification_id) REFERENCES notification_event(id)
);

CREATE TABLE IF NOT EXISTS notification_event_message (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  notification_id int NOT NULL,
  subject varchar(200),
  text text,
  content_type varchar(250),
  language varchar(60) NOT NULL,
  CONSTRAINT notification_event_message_pkey PRIMARY KEY (id),
  CONSTRAINT notification_event_message_notification_id_fkey FOREIGN KEY (notification_id) REFERENCES notification_event(id)
);

-- Links system
CREATE TABLE IF NOT EXISTS link_type (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(50) NOT NULL,
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT link_type_pkey PRIMARY KEY (id),
  CONSTRAINT link_type_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS link_state (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(50) NOT NULL,
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT link_state_pkey PRIMARY KEY (id),
  CONSTRAINT link_state_name_key UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS link_object (
  id smallint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(100) NOT NULL,
  CONSTRAINT link_object_pkey PRIMARY KEY (id),
  CONSTRAINT link_object_name_key UNIQUE (name)
);

-- Dynamic fields
CREATE TABLE IF NOT EXISTS dynamic_field (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  internal_field smallint NOT NULL DEFAULT 0,
  name varchar(200) NOT NULL,
  label varchar(200) NOT NULL,
  field_order int NOT NULL,
  field_type varchar(200) NOT NULL,
  object_type varchar(200) NOT NULL,
  config text,
  valid_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT dynamic_field_pkey PRIMARY KEY (id),
  CONSTRAINT dynamic_field_name_key UNIQUE (name)
);

-- Scheduler system
CREATE TABLE IF NOT EXISTS scheduler_recurrent_task (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(150) NOT NULL UNIQUE,
  task_type varchar(150) NOT NULL,
  task_data text,
  attempts int NOT NULL,
  lock_key bigint NOT NULL DEFAULT 0,
  lock_time timestamp,
  lock_update_time timestamp,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT scheduler_recurrent_task_pkey PRIMARY KEY (id)
);

-- System configuration
CREATE TABLE IF NOT EXISTS sysconfig_default (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(250) NOT NULL UNIQUE,
  description text NOT NULL,
  navigation varchar(200) NOT NULL,
  is_invisible smallint NOT NULL DEFAULT 0,
  is_readonly smallint NOT NULL DEFAULT 0,
  is_required smallint NOT NULL DEFAULT 1,
  is_valid smallint NOT NULL DEFAULT 1,
  has_configlevel smallint NOT NULL DEFAULT 0,
  user_modification_possible smallint NOT NULL DEFAULT 0,
  user_modification_active smallint NOT NULL DEFAULT 0,
  user_preferences_group varchar(250),
  xml_content_raw text NOT NULL,
  xml_content_parsed text NOT NULL,
  xml_filename varchar(250) NOT NULL,
  effective_value text NOT NULL,
  is_dirty smallint NOT NULL DEFAULT 1,
  exclusive_lock_guid varchar(32) NOT NULL DEFAULT 0,
  exclusive_lock_user_id int,
  exclusive_lock_expiry_time timestamp,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT sysconfig_default_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sysconfig_default_version (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  sysconfig_default_id int NOT NULL,
  name varchar(250) NOT NULL,
  description text NOT NULL,
  navigation varchar(200) NOT NULL,
  is_invisible smallint NOT NULL DEFAULT 0,
  is_readonly smallint NOT NULL DEFAULT 0,
  is_required smallint NOT NULL DEFAULT 1,
  is_valid smallint NOT NULL DEFAULT 1,
  has_configlevel smallint NOT NULL DEFAULT 0,
  user_modification_possible smallint NOT NULL DEFAULT 0,
  user_modification_active smallint NOT NULL DEFAULT 0,
  user_preferences_group varchar(250),
  xml_content_raw text NOT NULL,
  xml_content_parsed text NOT NULL,
  xml_filename varchar(250) NOT NULL,
  effective_value text NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  CONSTRAINT sysconfig_default_version_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sysconfig_modified (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  sysconfig_default_id int NOT NULL,
  name varchar(250) NOT NULL,
  user_id int,
  is_valid smallint NOT NULL DEFAULT 1,
  user_modification_active smallint NOT NULL DEFAULT 0,
  effective_value text NOT NULL,
  reset_to_default smallint NOT NULL DEFAULT 0,
  is_dirty smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  CONSTRAINT sysconfig_modified_pkey PRIMARY KEY (id),
  CONSTRAINT sysconfig_modified_sysconfig_default_id_fkey FOREIGN KEY (sysconfig_default_id) REFERENCES sysconfig_default(id)
);

CREATE TABLE IF NOT EXISTS sysconfig_modified_version (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  sysconfig_version_id int NOT NULL,
  name varchar(250) NOT NULL,
  user_id int,
  is_valid smallint NOT NULL DEFAULT 1,
  user_modification_active smallint NOT NULL DEFAULT 0,
  effective_value text NOT NULL,
  reset_to_default smallint NOT NULL DEFAULT 0,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  CONSTRAINT sysconfig_modified_version_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sysconfig_deployment (
  id int NOT NULL GENERATED ALWAYS AS IDENTITY,
  comments varchar(250),
  user_id int,
  effective_value text NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  CONSTRAINT sysconfig_deployment_pkey PRIMARY KEY (id)
);

-- XML storage for complex configurations
CREATE TABLE IF NOT EXISTS xml_storage (
  xml_type varchar(200) NOT NULL,
  xml_key varchar(250) NOT NULL,
  xml_content_key varchar(250) NOT NULL,
  xml_content_value text
);

-- Create indexes for performance
CREATE INDEX article_flag_article_id ON article_flag(article_id);
CREATE INDEX ticket_flag_ticket_id ON ticket_flag(ticket_id);
CREATE INDEX article_search_index_ticket_id ON article_search_index(ticket_id);
CREATE INDEX article_search_index_article_id ON article_search_index(article_id);
CREATE INDEX customer_preferences_user_id ON customer_preferences(user_id);
CREATE INDEX group_customer_user_user_id ON group_customer_user(user_id);
CREATE INDEX group_customer_user_group_id ON group_customer_user(group_id);
CREATE INDEX group_customer_customer_id ON group_customer(customer_id);
CREATE INDEX group_customer_group_id ON group_customer(group_id);
CREATE INDEX user_preferences_user_id ON user_preferences(user_id);
CREATE INDEX time_accounting_ticket_id ON time_accounting(ticket_id);
CREATE INDEX communication_log_create_time ON communication_log(create_time);
CREATE INDEX notification_event_item_notification_id ON notification_event_item(notification_id);
CREATE INDEX sysconfig_default_name ON sysconfig_default(name);
CREATE INDEX sysconfig_modified_sysconfig_default_id ON sysconfig_modified(sysconfig_default_id);
CREATE INDEX xml_storage_type_key ON xml_storage(xml_type, xml_key);