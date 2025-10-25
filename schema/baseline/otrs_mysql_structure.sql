CREATE TABLE `acl` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `description` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `stop_after_match` smallint(6) DEFAULT NULL,
  `config_match` longblob,
  `config_change` longblob,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `acl_name` (`name`),
  KEY `FK_acl_create_by_id` (`create_by`),
  KEY `FK_acl_change_by_id` (`change_by`),
  KEY `FK_acl_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_acl_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_acl_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_acl_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `acl_sync` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `acl_id` varchar(200) NOT NULL,
  `sync_state` varchar(30) NOT NULL,
  `create_time` datetime NOT NULL,
  `change_time` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `acl_ticket_attribute_relations` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `filename` varchar(255) NOT NULL,
  `attribute_1` varchar(200) NOT NULL,
  `attribute_2` varchar(200) NOT NULL,
  `acl_data` mediumtext NOT NULL,
  `priority` bigint(20) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `acl_tar_filename` (`filename`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `activity` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `dbcrud_uuid` varchar(36) DEFAULT NULL,
  `user_id` int(11) NOT NULL,
  `activity_type` varchar(200) NOT NULL,
  `activity_title` varchar(255) NOT NULL,
  `activity_text` longblob,
  `activity_state` varchar(255) DEFAULT NULL,
  `activity_link` varchar(255) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `activity_uuid` (`dbcrud_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `translation` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `dbcrud_uuid` varchar(36) DEFAULT NULL,
  `language_id` varchar(5) NOT NULL,
  `source_string` text NOT NULL,
  `destination_string` text NOT NULL,
  `valid_id` smallint(6) NOT NULL DEFAULT 1,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  `deployment_state` smallint(6) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `translation_uuid` (`dbcrud_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `article_color` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `color` varchar(10) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `article_color_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `article` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `ticket_id` bigint(20) NOT NULL,
  `article_sender_type_id` smallint(6) NOT NULL,
  `communication_channel_id` bigint(20) NOT NULL,
  `is_visible_for_customer` smallint(6) NOT NULL,
  `search_index_needs_rebuild` smallint(6) NOT NULL DEFAULT '1',
  `insert_fingerprint` varchar(64) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `article_article_sender_type_id` (`article_sender_type_id`),
  KEY `article_communication_channel_id` (`communication_channel_id`),
  KEY `article_search_index_needs_rebuild` (`search_index_needs_rebuild`),
  KEY `article_ticket_id` (`ticket_id`),
  KEY `FK_article_create_by_id` (`create_by`),
  KEY `FK_article_change_by_id` (`change_by`),
  CONSTRAINT `FK_article_article_sender_type_id_id` FOREIGN KEY (`article_sender_type_id`) REFERENCES `article_sender_type` (`id`),
  CONSTRAINT `FK_article_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_article_communication_channel_id_id` FOREIGN KEY (`communication_channel_id`) REFERENCES `communication_channel` (`id`),
  CONSTRAINT `FK_article_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_article_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=12 DEFAULT CHARSET=utf8;

CREATE TABLE `article_data_mime` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `article_id` bigint(20) NOT NULL,
  `a_from` mediumtext,
  `a_reply_to` mediumtext,
  `a_to` mediumtext,
  `a_cc` mediumtext,
  `a_bcc` mediumtext,
  `a_subject` text,
  `a_message_id` text,
  `a_message_id_md5` varchar(32) DEFAULT NULL,
  `a_in_reply_to` mediumtext,
  `a_references` mediumtext,
  `a_content_type` varchar(250) DEFAULT NULL,
  `a_body` mediumtext,
  `incoming_time` int(11) NOT NULL,
  `content_path` varchar(250) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `article_data_mime_article_id` (`article_id`),
  KEY `article_data_mime_message_id_md5` (`a_message_id_md5`),
  KEY `FK_article_data_mime_create_by_id` (`create_by`),
  KEY `FK_article_data_mime_change_by_id` (`change_by`),
  CONSTRAINT `FK_article_data_mime_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`),
  CONSTRAINT `FK_article_data_mime_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_article_data_mime_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=12 DEFAULT CHARSET=utf8;

CREATE TABLE `article_data_mime_attachment` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `article_id` bigint(20) NOT NULL,
  `filename` varchar(250) DEFAULT NULL,
  `content_size` varchar(30) DEFAULT NULL,
  `content_type` text,
  `content_id` varchar(250) DEFAULT NULL,
  `content_alternative` varchar(50) DEFAULT NULL,
  `disposition` varchar(15) DEFAULT NULL,
  `content` longblob,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `article_data_mime_attachment_article_id` (`article_id`),
  KEY `FK_article_data_mime_attachment_create_by_id` (`create_by`),
  KEY `FK_article_data_mime_attachment_change_by_id` (`change_by`),
  CONSTRAINT `FK_article_data_mime_attachment_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`),
  CONSTRAINT `FK_article_data_mime_attachment_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_article_data_mime_attachment_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=12 DEFAULT CHARSET=utf8;

CREATE TABLE `article_data_mime_plain` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `article_id` bigint(20) NOT NULL,
  `body` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `article_data_mime_plain_article_id` (`article_id`),
  KEY `FK_article_data_mime_plain_create_by_id` (`create_by`),
  KEY `FK_article_data_mime_plain_change_by_id` (`change_by`),
  CONSTRAINT `FK_article_data_mime_plain_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`),
  CONSTRAINT `FK_article_data_mime_plain_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_article_data_mime_plain_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8;

CREATE TABLE `article_data_mime_send_error` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `article_id` bigint(20) NOT NULL,
  `message_id` varchar(200) DEFAULT NULL,
  `log_message` mediumtext,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `article_data_mime_transmission_article_id` (`article_id`),
  KEY `article_data_mime_transmission_message_id` (`message_id`),
  CONSTRAINT `FK_article_data_mime_send_error_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=7 DEFAULT CHARSET=utf8;

CREATE TABLE `article_data_otrs_chat` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `article_id` bigint(20) NOT NULL,
  `chat_participant_id` varchar(255) NOT NULL,
  `chat_participant_name` varchar(255) NOT NULL,
  `chat_participant_type` varchar(255) NOT NULL,
  `message_text` text,
  `system_generated` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `article_data_otrs_chat_article_id` (`article_id`),
  CONSTRAINT `FK_article_data_otrs_chat_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `article_flag` (
  `article_id` bigint(20) NOT NULL,
  `article_key` varchar(50) NOT NULL,
  `article_value` varchar(50) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  KEY `article_flag_article_id` (`article_id`),
  KEY `article_flag_article_id_create_by` (`article_id`,`create_by`),
  KEY `FK_article_flag_create_by_id` (`create_by`),
  CONSTRAINT `FK_article_flag_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`),
  CONSTRAINT `FK_article_flag_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `article_search_index` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `ticket_id` bigint(20) NOT NULL,
  `article_id` bigint(20) NOT NULL,
  `article_key` varchar(200) NOT NULL,
  `article_value` mediumtext,
  PRIMARY KEY (`id`),
  KEY `article_search_index_article_id` (`article_id`,`article_key`),
  KEY `article_search_index_ticket_id` (`ticket_id`,`article_key`),
  CONSTRAINT `FK_article_search_index_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`),
  CONSTRAINT `FK_article_search_index_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=83 DEFAULT CHARSET=utf8;

CREATE TABLE `article_sender_type` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `article_sender_type_name` (`name`),
  KEY `FK_article_sender_type_create_by_id` (`create_by`),
  KEY `FK_article_sender_type_change_by_id` (`change_by`),
  KEY `FK_article_sender_type_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_article_sender_type_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_article_sender_type_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_article_sender_type_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8;

CREATE TABLE `auto_response` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `text0` text,
  `text1` text,
  `type_id` smallint(6) NOT NULL,
  `system_address_id` smallint(6) NOT NULL,
  `content_type` varchar(250) DEFAULT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `auto_response_name` (`name`),
  KEY `FK_auto_response_type_id_id` (`type_id`),
  KEY `FK_auto_response_system_address_id_id` (`system_address_id`),
  KEY `FK_auto_response_create_by_id` (`create_by`),
  KEY `FK_auto_response_change_by_id` (`change_by`),
  KEY `FK_auto_response_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_auto_response_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_auto_response_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_auto_response_system_address_id_id` FOREIGN KEY (`system_address_id`) REFERENCES `system_address` (`id`),
  CONSTRAINT `FK_auto_response_type_id_id` FOREIGN KEY (`type_id`) REFERENCES `auto_response_type` (`id`),
  CONSTRAINT `FK_auto_response_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8;

CREATE TABLE `auto_response_type` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `auto_response_type_name` (`name`),
  KEY `FK_auto_response_type_create_by_id` (`create_by`),
  KEY `FK_auto_response_type_change_by_id` (`change_by`),
  KEY `FK_auto_response_type_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_auto_response_type_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_auto_response_type_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_auto_response_type_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8;

CREATE TABLE `calendar` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `group_id` int(11) NOT NULL,
  `name` varchar(200) NOT NULL,
  `salt_string` varchar(64) NOT NULL,
  `color` varchar(7) NOT NULL,
  `ticket_appointments` longblob,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `calendar_name` (`name`),
  KEY `FK_calendar_group_id_id` (`group_id`),
  KEY `FK_calendar_create_by_id` (`create_by`),
  KEY `FK_calendar_change_by_id` (`change_by`),
  KEY `FK_calendar_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_calendar_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_calendar_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_calendar_group_id_id` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`),
  CONSTRAINT `FK_calendar_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `calendar_appointment` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `parent_id` bigint(20) DEFAULT NULL,
  `calendar_id` bigint(20) NOT NULL,
  `unique_id` varchar(255) NOT NULL,
  `title` varchar(255) NOT NULL,
  `description` text,
  `location` varchar(255) DEFAULT NULL,
  `start_time` datetime NOT NULL,
  `end_time` datetime NOT NULL,
  `all_day` smallint(6) DEFAULT NULL,
  `notify_time` datetime DEFAULT NULL,
  `notify_template` varchar(255) DEFAULT NULL,
  `notify_custom` varchar(255) DEFAULT NULL,
  `notify_custom_unit_count` bigint(20) DEFAULT NULL,
  `notify_custom_unit` varchar(255) DEFAULT NULL,
  `notify_custom_unit_point` varchar(255) DEFAULT NULL,
  `notify_custom_date` datetime DEFAULT NULL,
  `team_id` text,
  `resource_id` text,
  `recurring` smallint(6) DEFAULT NULL,
  `recur_type` varchar(20) DEFAULT NULL,
  `recur_freq` varchar(255) DEFAULT NULL,
  `recur_count` int(11) DEFAULT NULL,
  `recur_interval` int(11) DEFAULT NULL,
  `recur_until` datetime DEFAULT NULL,
  `recur_id` datetime DEFAULT NULL,
  `recur_exclude` text,
  `ticket_appointment_rule_id` varchar(32) DEFAULT NULL,
  `create_time` datetime DEFAULT NULL,
  `create_by` int(11) DEFAULT NULL,
  `change_time` datetime DEFAULT NULL,
  `change_by` int(11) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_calendar_appointment_calendar_id_id` (`calendar_id`),
  KEY `FK_calendar_appointment_parent_id_id` (`parent_id`),
  KEY `FK_calendar_appointment_create_by_id` (`create_by`),
  KEY `FK_calendar_appointment_change_by_id` (`change_by`),
  CONSTRAINT `FK_calendar_appointment_calendar_id_id` FOREIGN KEY (`calendar_id`) REFERENCES `calendar` (`id`),
  CONSTRAINT `FK_calendar_appointment_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_calendar_appointment_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_calendar_appointment_parent_id_id` FOREIGN KEY (`parent_id`) REFERENCES `calendar_appointment` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `calendar_appointment_plugin` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `dbcrud_uuid` varchar(36) DEFAULT NULL,
  `appointment_id` bigint(20) NOT NULL,
  `plugin_key` text NOT NULL,
  `config` mediumtext,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `calendar_appointment_plugin_uuid` (`dbcrud_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `calendar_appointment_ticket` (
  `calendar_id` bigint(20) NOT NULL,
  `ticket_id` bigint(20) NOT NULL,
  `rule_id` varchar(32) NOT NULL,
  `appointment_id` bigint(20) NOT NULL,
  UNIQUE KEY `calendar_appointment_ticket_calendar_id_ticket_id_rule_id` (`calendar_id`,`ticket_id`,`rule_id`),
  KEY `calendar_appointment_ticket_appointment_id` (`appointment_id`),
  KEY `calendar_appointment_ticket_calendar_id` (`calendar_id`),
  KEY `calendar_appointment_ticket_rule_id` (`rule_id`),
  KEY `calendar_appointment_ticket_ticket_id` (`ticket_id`),
  CONSTRAINT `FK_calendar_appointment_ticket_appointment_id_id` FOREIGN KEY (`appointment_id`) REFERENCES `calendar_appointment` (`id`),
  CONSTRAINT `FK_calendar_appointment_ticket_calendar_id_id` FOREIGN KEY (`calendar_id`) REFERENCES `calendar` (`id`),
  CONSTRAINT `FK_calendar_appointment_ticket_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `cloud_service_config` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `config` longblob NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cloud_service_config_name` (`name`),
  KEY `FK_cloud_service_config_create_by_id` (`create_by`),
  KEY `FK_cloud_service_config_change_by_id` (`change_by`),
  KEY `FK_cloud_service_config_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_cloud_service_config_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_cloud_service_config_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_cloud_service_config_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `communication_channel` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `module` varchar(200) NOT NULL,
  `package_name` varchar(200) NOT NULL,
  `channel_data` longblob NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `communication_channel_name` (`name`),
  KEY `FK_communication_channel_create_by_id` (`create_by`),
  KEY `FK_communication_channel_change_by_id` (`change_by`),
  KEY `FK_communication_channel_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_communication_channel_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_communication_channel_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_communication_channel_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8;

CREATE TABLE `communication_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `insert_fingerprint` varchar(64) DEFAULT NULL,
  `transport` varchar(200) NOT NULL,
  `direction` varchar(200) NOT NULL,
  `status` varchar(200) NOT NULL,
  `account_type` varchar(200) DEFAULT NULL,
  `account_id` varchar(200) DEFAULT NULL,
  `start_time` datetime NOT NULL,
  `end_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `communication_direction` (`direction`),
  KEY `communication_start_time` (`start_time`),
  KEY `communication_status` (`status`),
  KEY `communication_transport` (`transport`)
) ENGINE=InnoDB AUTO_INCREMENT=32 DEFAULT CHARSET=utf8;

CREATE TABLE `communication_log_obj_lookup` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `communication_log_object_id` bigint(20) NOT NULL,
  `object_type` varchar(200) NOT NULL,
  `object_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `communication_log_obj_lookup_target` (`object_type`,`object_id`),
  KEY `FK_communication_log_obj_lookup_communication_log_object_i0f` (`communication_log_object_id`),
  CONSTRAINT `FK_communication_log_obj_lookup_communication_log_object_i0f` FOREIGN KEY (`communication_log_object_id`) REFERENCES `communication_log_object` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8;

CREATE TABLE `communication_log_object` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `insert_fingerprint` varchar(64) DEFAULT NULL,
  `communication_id` bigint(20) NOT NULL,
  `object_type` varchar(50) NOT NULL,
  `status` varchar(200) NOT NULL,
  `start_time` datetime NOT NULL,
  `end_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `communication_log_object_object_type` (`object_type`),
  KEY `communication_log_object_status` (`status`),
  KEY `FK_communication_log_object_communication_id_id` (`communication_id`),
  CONSTRAINT `FK_communication_log_object_communication_id_id` FOREIGN KEY (`communication_id`) REFERENCES `communication_log` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=47 DEFAULT CHARSET=utf8;

CREATE TABLE `communication_log_object_entry` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `communication_log_object_id` bigint(20) NOT NULL,
  `log_key` varchar(200) NOT NULL,
  `log_value` mediumtext NOT NULL,
  `priority` varchar(50) NOT NULL,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `communication_log_object_entry_key` (`log_key`),
  KEY `FK_communication_log_object_entry_communication_log_objectaa` (`communication_log_object_id`),
  CONSTRAINT `FK_communication_log_object_entry_communication_log_objectaa` FOREIGN KEY (`communication_log_object_id`) REFERENCES `communication_log_object` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=427 DEFAULT CHARSET=utf8;

CREATE TABLE `customer_company` (
  `customer_id` varchar(150) NOT NULL,
  `name` varchar(200) NOT NULL,
  `street` varchar(200) DEFAULT NULL,
  `zip` varchar(200) DEFAULT NULL,
  `city` varchar(200) DEFAULT NULL,
  `country` varchar(200) DEFAULT NULL,
  `url` varchar(200) DEFAULT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`customer_id`),
  UNIQUE KEY `customer_company_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `customer_preferences` (
  `user_id` varchar(250) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` varchar(250) DEFAULT NULL,
  KEY `customer_preferences_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `customer_user` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `login` varchar(200) NOT NULL,
  `email` varchar(150) NOT NULL,
  `customer_id` varchar(150) NOT NULL,
  `pw` varchar(128) DEFAULT NULL,
  `title` varchar(50) DEFAULT NULL,
  `first_name` varchar(100) NOT NULL,
  `last_name` varchar(100) NOT NULL,
  `phone` varchar(150) DEFAULT NULL,
  `fax` varchar(150) DEFAULT NULL,
  `mobile` varchar(150) DEFAULT NULL,
  `street` varchar(150) DEFAULT NULL,
  `zip` varchar(200) DEFAULT NULL,
  `city` varchar(200) DEFAULT NULL,
  `country` varchar(200) DEFAULT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `customer_user_login` (`login`),
  KEY `FK_customer_user_create_by_id` (`create_by`),
  KEY `FK_customer_user_change_by_id` (`change_by`),
  KEY `FK_customer_user_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_customer_user_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_customer_user_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_customer_user_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `customer_user_customer` (
  `user_id` varchar(100) NOT NULL,
  `customer_id` varchar(150) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `customer_user_customer_customer_id` (`customer_id`),
  KEY `customer_user_customer_user_id` (`user_id`),
  KEY `FK_customer_user_customer_create_by_id` (`create_by`),
  KEY `FK_customer_user_customer_change_by_id` (`change_by`),
  CONSTRAINT `FK_customer_user_customer_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_customer_user_customer_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `dynamic_field` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `internal_field` smallint(6) NOT NULL DEFAULT '0',
  `name` varchar(200) NOT NULL,
  `label` varchar(200) NOT NULL,
  `field_order` int(11) NOT NULL,
  `field_type` varchar(200) NOT NULL,
  `object_type` varchar(100) NOT NULL,
  `config` longblob,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `dynamic_field_name` (`name`),
  KEY `FK_dynamic_field_create_by_id` (`create_by`),
  KEY `FK_dynamic_field_change_by_id` (`change_by`),
  KEY `FK_dynamic_field_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_dynamic_field_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_dynamic_field_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_dynamic_field_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `dynamic_field_obj_id_name` (
  `object_id` int(11) NOT NULL AUTO_INCREMENT,
  `object_name` varchar(200) NOT NULL,
  `object_type` varchar(100) NOT NULL,
  PRIMARY KEY (`object_id`),
  UNIQUE KEY `dynamic_field_object_name` (`object_name`,`object_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `dynamic_field_value` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `field_id` int(11) NOT NULL,
  `object_id` bigint(20) NOT NULL,
  `value_text` text,
  `value_date` datetime DEFAULT NULL,
  `value_int` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `dynamic_field_value_field_values` (`object_id`,`field_id`),
  KEY `dynamic_field_value_search_date` (`field_id`,`value_date`),
  KEY `dynamic_field_value_search_int` (`field_id`,`value_int`),
  KEY `dynamic_field_value_search_text` (`field_id`,`value_text`(150)),
  CONSTRAINT `FK_dynamic_field_value_field_id_id` FOREIGN KEY (`field_id`) REFERENCES `dynamic_field` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `follow_up_possible` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `follow_up_possible_name` (`name`),
  KEY `FK_follow_up_possible_create_by_id` (`create_by`),
  KEY `FK_follow_up_possible_change_by_id` (`change_by`),
  KEY `FK_follow_up_possible_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_follow_up_possible_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_follow_up_possible_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_follow_up_possible_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8;

CREATE TABLE `form_draft` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `object_type` varchar(100) NOT NULL,
  `object_id` int(11) NOT NULL,
  `action` varchar(200) NOT NULL,
  `title` varchar(255) DEFAULT NULL,
  `content` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `form_draft_object_type_object_id_action` (`object_type`,`object_id`,`action`),
  KEY `FK_form_draft_create_by_id` (`create_by`),
  KEY `FK_form_draft_change_by_id` (`change_by`),
  CONSTRAINT `FK_form_draft_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_form_draft_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `smime_keys` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `key_hash` varchar(8) NOT NULL,
  `key_type` varchar(255) NOT NULL,
  `file_name` varchar(255) NOT NULL,
  `email_address` mediumtext,
  `expiration_date` datetime DEFAULT NULL,
  `fingerprint` varchar(59) DEFAULT NULL,
  `subject` mediumtext,
  `create_time` datetime DEFAULT NULL,
  `change_time` datetime DEFAULT NULL,
  `create_by` int(11) DEFAULT NULL,
  `change_by` int(11) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `smime_keys_file_name` (`file_name`),
  KEY `smime_keys_key_hash` (`key_hash`),
  KEY `smime_keys_key_type` (`key_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `oauth2_token_config` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `dbcrud_uuid` varchar(36) DEFAULT NULL,
  `name` varchar(250) NOT NULL,
  `config` mediumtext NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `oauth2_token_config_name` (`name`),
  UNIQUE KEY `oauth2_token_config_uuid` (`dbcrud_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `oauth2_token` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `dbcrud_uuid` varchar(36) DEFAULT NULL,
  `token_config_id` int(11) NOT NULL,
  `authorization_code` mediumtext,
  `token` mediumtext,
  `token_expiration_date` datetime DEFAULT NULL,
  `refresh_token` mediumtext,
  `refresh_token_expiration_date` datetime DEFAULT NULL,
  `error_message` mediumtext,
  `error_description` mediumtext,
  `error_code` varchar(1000) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `oauth2_token_config_id` (`token_config_id`),
  UNIQUE KEY `oauth2_token_uuid` (`dbcrud_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `mention` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `user_id` int(11) DEFAULT NULL,
  `ticket_id` int(11) DEFAULT NULL,
  `article_id` int(11) DEFAULT NULL,
  `create_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `mention_article_id` (`article_id`),
  KEY `mention_ticket_id` (`ticket_id`),
  KEY `mention_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `generic_agent_jobs` (
  `job_name` varchar(200) NOT NULL,
  `job_key` varchar(200) NOT NULL,
  `job_value` varchar(200) DEFAULT NULL,
  KEY `generic_agent_jobs_job_name` (`job_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `gi_debugger_entry` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `communication_id` varchar(32) NOT NULL,
  `communication_type` varchar(50) NOT NULL,
  `remote_ip` varchar(50) DEFAULT NULL,
  `webservice_id` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `gi_debugger_entry_communication_id` (`communication_id`),
  KEY `gi_debugger_entry_create_time` (`create_time`),
  KEY `FK_gi_debugger_entry_webservice_id_id` (`webservice_id`),
  CONSTRAINT `FK_gi_debugger_entry_webservice_id_id` FOREIGN KEY (`webservice_id`) REFERENCES `gi_webservice_config` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `gi_debugger_entry_content` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `gi_debugger_entry_id` bigint(20) NOT NULL,
  `debug_level` varchar(50) NOT NULL,
  `subject` varchar(255) NOT NULL,
  `content` longblob,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `gi_debugger_entry_content_create_time` (`create_time`),
  KEY `gi_debugger_entry_content_debug_level` (`debug_level`),
  KEY `FK_gi_debugger_entry_content_gi_debugger_entry_id_id` (`gi_debugger_entry_id`),
  CONSTRAINT `FK_gi_debugger_entry_content_gi_debugger_entry_id_id` FOREIGN KEY (`gi_debugger_entry_id`) REFERENCES `gi_debugger_entry` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `gi_webservice_config` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `config` longblob NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `gi_webservice_config_name` (`name`),
  KEY `FK_gi_webservice_config_create_by_id` (`create_by`),
  KEY `FK_gi_webservice_config_change_by_id` (`change_by`),
  KEY `FK_gi_webservice_config_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_gi_webservice_config_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_gi_webservice_config_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_gi_webservice_config_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `gi_webservice_config_history` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `config_id` int(11) NOT NULL,
  `config` longblob NOT NULL,
  `config_md5` varchar(32) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `gi_webservice_config_history_config_md5` (`config_md5`),
  KEY `FK_gi_webservice_config_history_config_id_id` (`config_id`),
  KEY `FK_gi_webservice_config_history_create_by_id` (`create_by`),
  KEY `FK_gi_webservice_config_history_change_by_id` (`change_by`),
  CONSTRAINT `FK_gi_webservice_config_history_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_gi_webservice_config_history_config_id_id` FOREIGN KEY (`config_id`) REFERENCES `gi_webservice_config` (`id`),
  CONSTRAINT `FK_gi_webservice_config_history_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `group_customer` (
  `customer_id` varchar(150) NOT NULL,
  `group_id` int(11) NOT NULL,
  `permission_key` varchar(20) NOT NULL,
  `permission_value` smallint(6) NOT NULL,
  `permission_context` varchar(100) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `group_customer_customer_id` (`customer_id`),
  KEY `group_customer_group_id` (`group_id`),
  KEY `FK_group_customer_create_by_id` (`create_by`),
  KEY `FK_group_customer_change_by_id` (`change_by`),
  CONSTRAINT `FK_group_customer_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_customer_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_customer_group_id_id` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `group_customer_user` (
  `user_id` varchar(100) NOT NULL,
  `group_id` int(11) NOT NULL,
  `permission_key` varchar(20) NOT NULL,
  `permission_value` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `group_customer_user_group_id` (`group_id`),
  KEY `group_customer_user_user_id` (`user_id`),
  KEY `FK_group_customer_user_create_by_id` (`create_by`),
  KEY `FK_group_customer_user_change_by_id` (`change_by`),
  CONSTRAINT `FK_group_customer_user_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_customer_user_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_customer_user_group_id_id` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `group_role` (
  `role_id` int(11) NOT NULL,
  `group_id` int(11) NOT NULL,
  `permission_key` varchar(20) NOT NULL,
  `permission_value` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `group_role_group_id` (`group_id`),
  KEY `group_role_role_id` (`role_id`),
  KEY `FK_group_role_create_by_id` (`create_by`),
  KEY `FK_group_role_change_by_id` (`change_by`),
  CONSTRAINT `FK_group_role_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_role_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_role_group_id_id` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`),
  CONSTRAINT `FK_group_role_role_id_id` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `group_user` (
  `user_id` int(11) NOT NULL,
  `group_id` int(11) NOT NULL,
  `permission_key` varchar(20) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `group_user_group_id` (`group_id`),
  KEY `group_user_user_id` (`user_id`),
  KEY `FK_group_user_create_by_id` (`create_by`),
  KEY `FK_group_user_change_by_id` (`change_by`),
  CONSTRAINT `FK_group_user_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_user_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_group_user_group_id_id` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`),
  CONSTRAINT `FK_group_user_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `groups` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `groups_name` (`name`),
  KEY `FK_groups_create_by_id` (`create_by`),
  KEY `FK_groups_change_by_id` (`change_by`),
  KEY `FK_groups_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_groups_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_groups_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_groups_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8;

CREATE TABLE `permission_groups` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `permission_groups_name` (`name`),
  KEY `FK_permission_groups_create_by_id` (`create_by`),
  KEY `FK_permission_groups_change_by_id` (`change_by`),
  KEY `FK_permission_groups_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_permission_groups_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_permission_groups_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_permission_groups_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `link_object` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `link_object_name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8;

CREATE TABLE `link_relation` (
  `source_object_id` smallint(6) NOT NULL,
  `source_key` varchar(50) NOT NULL,
  `target_object_id` smallint(6) NOT NULL,
  `target_key` varchar(50) NOT NULL,
  `type_id` smallint(6) NOT NULL,
  `state_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  UNIQUE KEY `link_relation_view` (`source_object_id`,`source_key`,`target_object_id`,`target_key`,`type_id`),
  KEY `link_relation_list_source` (`source_object_id`,`source_key`,`state_id`),
  KEY `link_relation_list_target` (`target_object_id`,`target_key`,`state_id`),
  KEY `FK_link_relation_state_id_id` (`state_id`),
  KEY `FK_link_relation_type_id_id` (`type_id`),
  KEY `FK_link_relation_create_by_id` (`create_by`),
  CONSTRAINT `FK_link_relation_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_link_relation_source_object_id_id` FOREIGN KEY (`source_object_id`) REFERENCES `link_object` (`id`),
  CONSTRAINT `FK_link_relation_state_id_id` FOREIGN KEY (`state_id`) REFERENCES `link_state` (`id`),
  CONSTRAINT `FK_link_relation_target_object_id_id` FOREIGN KEY (`target_object_id`) REFERENCES `link_object` (`id`),
  CONSTRAINT `FK_link_relation_type_id_id` FOREIGN KEY (`type_id`) REFERENCES `link_type` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `link_state` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `link_state_name` (`name`),
  KEY `FK_link_state_create_by_id` (`create_by`),
  KEY `FK_link_state_change_by_id` (`change_by`),
  KEY `FK_link_state_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_link_state_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_link_state_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_link_state_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `link_type` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `link_type_name` (`name`),
  KEY `FK_link_type_create_by_id` (`create_by`),
  KEY `FK_link_type_change_by_id` (`change_by`),
  KEY `FK_link_type_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_link_type_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_link_type_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_link_type_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `mail_account` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `login` varchar(200) NOT NULL,
  `pw` varchar(200) NOT NULL,
  `host` varchar(200) NOT NULL,
  `account_type` varchar(20) NOT NULL,
  `queue_id` int(11) NOT NULL,
  `trusted` smallint(6) NOT NULL,
  `imap_folder` varchar(250) DEFAULT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_mail_account_create_by_id` (`create_by`),
  KEY `FK_mail_account_change_by_id` (`change_by`),
  KEY `FK_mail_account_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_mail_account_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_mail_account_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_mail_account_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `mail_queue` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `insert_fingerprint` varchar(64) DEFAULT NULL,
  `article_id` bigint(20) DEFAULT NULL,
  `attempts` int(11) NOT NULL,
  `sender` varchar(200) DEFAULT NULL,
  `recipient` mediumtext NOT NULL,
  `raw_message` longblob NOT NULL,
  `due_time` datetime DEFAULT NULL,
  `last_smtp_code` int(11) DEFAULT NULL,
  `last_smtp_message` mediumtext,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `mail_queue_article_id` (`article_id`),
  UNIQUE KEY `mail_queue_insert_fingerprint` (`insert_fingerprint`),
  KEY `mail_queue_attempts` (`attempts`),
  CONSTRAINT `FK_mail_queue_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `notification_event` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `notification_event_name` (`name`),
  KEY `FK_notification_event_create_by_id` (`create_by`),
  KEY `FK_notification_event_change_by_id` (`change_by`),
  KEY `FK_notification_event_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_notification_event_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_notification_event_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_notification_event_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8;

CREATE TABLE `notification_event_item` (
  `notification_id` int(11) NOT NULL,
  `event_key` varchar(200) NOT NULL,
  `event_value` varchar(200) NOT NULL,
  KEY `notification_event_item_event_key` (`event_key`),
  KEY `notification_event_item_event_value` (`event_value`),
  KEY `notification_event_item_notification_id` (`notification_id`),
  CONSTRAINT `FK_notification_event_item_notification_id_id` FOREIGN KEY (`notification_id`) REFERENCES `notification_event` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `notification_event_message` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `notification_id` int(11) NOT NULL,
  `subject` varchar(200) NOT NULL,
  `text` text NOT NULL,
  `content_type` varchar(250) NOT NULL,
  `language` varchar(60) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `notification_event_message_notification_id_language` (`notification_id`,`language`),
  KEY `notification_event_message_language` (`language`),
  KEY `notification_event_message_notification_id` (`notification_id`),
  CONSTRAINT `FK_notification_event_message_notification_id_id` FOREIGN KEY (`notification_id`) REFERENCES `notification_event` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=112 DEFAULT CHARSET=utf8;

CREATE TABLE `package_repository` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `version` varchar(250) NOT NULL,
  `vendor` varchar(250) NOT NULL,
  `install_status` varchar(250) NOT NULL,
  `filename` varchar(250) DEFAULT NULL,
  `content_type` varchar(250) DEFAULT NULL,
  `content` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_package_repository_create_by_id` (`create_by`),
  KEY `FK_package_repository_change_by_id` (`change_by`),
  CONSTRAINT `FK_package_repository_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_package_repository_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `personal_queues` (
  `user_id` int(11) NOT NULL,
  `queue_id` int(11) NOT NULL,
  KEY `personal_queues_queue_id` (`queue_id`),
  KEY `personal_queues_user_id` (`user_id`),
  CONSTRAINT `FK_personal_queues_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`),
  CONSTRAINT `FK_personal_queues_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `personal_services` (
  `user_id` int(11) NOT NULL,
  `service_id` int(11) NOT NULL,
  KEY `personal_services_service_id` (`service_id`),
  KEY `personal_services_user_id` (`user_id`),
  CONSTRAINT `FK_personal_services_service_id_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`),
  CONSTRAINT `FK_personal_services_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `pm_activity` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `entity_id` varchar(50) NOT NULL,
  `name` varchar(200) NOT NULL,
  `config` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `pm_activity_entity_id` (`entity_id`),
  KEY `FK_pm_activity_create_by_id` (`create_by`),
  KEY `FK_pm_activity_change_by_id` (`change_by`),
  CONSTRAINT `FK_pm_activity_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_pm_activity_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `pm_activity_dialog` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `entity_id` varchar(50) NOT NULL,
  `name` varchar(200) NOT NULL,
  `config` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `pm_activity_dialog_entity_id` (`entity_id`),
  KEY `FK_pm_activity_dialog_create_by_id` (`create_by`),
  KEY `FK_pm_activity_dialog_change_by_id` (`change_by`),
  CONSTRAINT `FK_pm_activity_dialog_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_pm_activity_dialog_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `pm_entity_sync` (
  `entity_type` varchar(30) NOT NULL,
  `entity_id` varchar(50) NOT NULL,
  `sync_state` varchar(30) NOT NULL,
  `create_time` datetime NOT NULL,
  `change_time` datetime NOT NULL,
  UNIQUE KEY `pm_entity_sync_list` (`entity_type`,`entity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `pm_process` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `entity_id` varchar(50) NOT NULL,
  `name` varchar(200) NOT NULL,
  `state_entity_id` varchar(50) NOT NULL,
  `layout` longblob NOT NULL,
  `config` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `pm_process_entity_id` (`entity_id`),
  KEY `FK_pm_process_create_by_id` (`create_by`),
  KEY `FK_pm_process_change_by_id` (`change_by`),
  CONSTRAINT `FK_pm_process_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_pm_process_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `pm_process_preferences` (
  `process_entity_id` varchar(50) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` varchar(3000) DEFAULT NULL,
  KEY `pm_process_preferences_process_entity_id` (`process_entity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `pm_transition` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `entity_id` varchar(50) NOT NULL,
  `name` varchar(200) NOT NULL,
  `config` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `pm_transition_entity_id` (`entity_id`),
  KEY `FK_pm_transition_create_by_id` (`create_by`),
  KEY `FK_pm_transition_change_by_id` (`change_by`),
  CONSTRAINT `FK_pm_transition_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_pm_transition_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `pm_transition_action` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `entity_id` varchar(50) NOT NULL,
  `name` varchar(200) NOT NULL,
  `config` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `pm_transition_action_entity_id` (`entity_id`),
  KEY `FK_pm_transition_action_create_by_id` (`create_by`),
  KEY `FK_pm_transition_action_change_by_id` (`change_by`),
  CONSTRAINT `FK_pm_transition_action_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_pm_transition_action_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `postmaster_filter` (
  `f_name` varchar(200) NOT NULL,
  `f_stop` smallint(6) DEFAULT NULL,
  `f_type` varchar(20) NOT NULL,
  `f_key` varchar(200) NOT NULL,
  `f_value` varchar(200) NOT NULL,
  `f_not` smallint(6) DEFAULT NULL,
  KEY `postmaster_filter_f_name` (`f_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `process_id` (
  `process_name` varchar(200) NOT NULL,
  `process_id` varchar(200) NOT NULL,
  `process_host` varchar(200) NOT NULL,
  `process_create` int(11) NOT NULL,
  `process_change` int(11) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `queue` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `group_id` int(11) NOT NULL,
  `unlock_timeout` int(11) DEFAULT NULL,
  `first_response_time` int(11) DEFAULT NULL,
  `first_response_notify` smallint(6) DEFAULT NULL,
  `update_time` int(11) DEFAULT NULL,
  `update_notify` smallint(6) DEFAULT NULL,
  `solution_time` int(11) DEFAULT NULL,
  `solution_notify` smallint(6) DEFAULT NULL,
  `system_address_id` smallint(6) NOT NULL,
  `calendar_name` varchar(100) DEFAULT NULL,
  `default_sign_key` varchar(100) DEFAULT NULL,
  `salutation_id` smallint(6) NOT NULL,
  `signature_id` smallint(6) NOT NULL,
  `follow_up_id` smallint(6) NOT NULL,
  `follow_up_lock` smallint(6) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `queue_name` (`name`),
  KEY `queue_group_id` (`group_id`),
  KEY `FK_queue_follow_up_id_id` (`follow_up_id`),
  KEY `FK_queue_salutation_id_id` (`salutation_id`),
  KEY `FK_queue_signature_id_id` (`signature_id`),
  KEY `FK_queue_system_address_id_id` (`system_address_id`),
  KEY `FK_queue_create_by_id` (`create_by`),
  KEY `FK_queue_change_by_id` (`change_by`),
  KEY `FK_queue_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_queue_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_queue_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_queue_follow_up_id_id` FOREIGN KEY (`follow_up_id`) REFERENCES `follow_up_possible` (`id`),
  CONSTRAINT `FK_queue_group_id_id` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`),
  CONSTRAINT `FK_queue_salutation_id_id` FOREIGN KEY (`salutation_id`) REFERENCES `salutation` (`id`),
  CONSTRAINT `FK_queue_signature_id_id` FOREIGN KEY (`signature_id`) REFERENCES `signature` (`id`),
  CONSTRAINT `FK_queue_system_address_id_id` FOREIGN KEY (`system_address_id`) REFERENCES `system_address` (`id`),
  CONSTRAINT `FK_queue_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8;

CREATE TABLE `queue_auto_response` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `queue_id` int(11) NOT NULL,
  `auto_response_id` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_queue_auto_response_auto_response_id_id` (`auto_response_id`),
  KEY `FK_queue_auto_response_queue_id_id` (`queue_id`),
  KEY `FK_queue_auto_response_create_by_id` (`create_by`),
  KEY `FK_queue_auto_response_change_by_id` (`change_by`),
  CONSTRAINT `FK_queue_auto_response_auto_response_id_id` FOREIGN KEY (`auto_response_id`) REFERENCES `auto_response` (`id`),
  CONSTRAINT `FK_queue_auto_response_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_queue_auto_response_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_queue_auto_response_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `queue_preferences` (
  `queue_id` int(11) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` varchar(250) DEFAULT NULL,
  KEY `queue_preferences_queue_id` (`queue_id`),
  CONSTRAINT `FK_queue_preferences_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `queue_standard_template` (
  `queue_id` int(11) NOT NULL,
  `standard_template_id` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `FK_queue_standard_template_queue_id_id` (`queue_id`),
  KEY `FK_queue_standard_template_standard_template_id_id` (`standard_template_id`),
  KEY `FK_queue_standard_template_create_by_id` (`create_by`),
  KEY `FK_queue_standard_template_change_by_id` (`change_by`),
  CONSTRAINT `FK_queue_standard_template_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_queue_standard_template_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_queue_standard_template_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`),
  CONSTRAINT `FK_queue_standard_template_standard_template_id_id` FOREIGN KEY (`standard_template_id`) REFERENCES `standard_template` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `role_user` (
  `user_id` int(11) NOT NULL,
  `role_id` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `role_user_role_id` (`role_id`),
  KEY `role_user_user_id` (`user_id`),
  KEY `FK_role_user_create_by_id` (`create_by`),
  KEY `FK_role_user_change_by_id` (`change_by`),
  CONSTRAINT `FK_role_user_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_role_user_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_role_user_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `roles` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `roles_name` (`name`),
  KEY `FK_roles_create_by_id` (`create_by`),
  KEY `FK_roles_change_by_id` (`change_by`),
  KEY `FK_roles_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_roles_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_roles_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_roles_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `salutation` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `text` text NOT NULL,
  `content_type` varchar(250) DEFAULT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `salutation_name` (`name`),
  KEY `FK_salutation_create_by_id` (`create_by`),
  KEY `FK_salutation_change_by_id` (`change_by`),
  KEY `FK_salutation_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_salutation_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_salutation_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_salutation_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8;

CREATE TABLE `scheduler_future_task` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `ident` bigint(20) NOT NULL,
  `execution_time` datetime NOT NULL,
  `name` varchar(150) DEFAULT NULL,
  `task_type` varchar(150) NOT NULL,
  `task_data` longblob NOT NULL,
  `attempts` smallint(6) NOT NULL,
  `lock_key` bigint(20) NOT NULL,
  `lock_time` datetime DEFAULT NULL,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `scheduler_future_task_ident` (`ident`),
  KEY `scheduler_future_task_ident_id` (`ident`,`id`),
  KEY `scheduler_future_task_lock_key_id` (`lock_key`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `scheduler_recurrent_task` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(150) NOT NULL,
  `task_type` varchar(150) NOT NULL,
  `last_execution_time` datetime NOT NULL,
  `last_worker_task_id` bigint(20) DEFAULT NULL,
  `last_worker_status` smallint(6) DEFAULT NULL,
  `last_worker_running_time` int(11) DEFAULT NULL,
  `lock_key` bigint(20) NOT NULL,
  `lock_time` datetime DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `change_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `scheduler_recurrent_task_name_task_type` (`name`,`task_type`),
  KEY `scheduler_recurrent_task_lock_key_id` (`lock_key`,`id`),
  KEY `scheduler_recurrent_task_task_type_name` (`task_type`,`name`)
) ENGINE=InnoDB AUTO_INCREMENT=21 DEFAULT CHARSET=utf8;

CREATE TABLE `scheduler_task` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `ident` bigint(20) NOT NULL,
  `name` varchar(150) DEFAULT NULL,
  `task_type` varchar(150) NOT NULL,
  `task_data` longblob NOT NULL,
  `attempts` smallint(6) NOT NULL,
  `lock_key` bigint(20) NOT NULL,
  `lock_time` datetime DEFAULT NULL,
  `lock_update_time` datetime DEFAULT NULL,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `scheduler_task_ident` (`ident`),
  KEY `scheduler_task_ident_id` (`ident`,`id`),
  KEY `scheduler_task_lock_key_id` (`lock_key`,`id`)
) ENGINE=InnoDB AUTO_INCREMENT=12 DEFAULT CHARSET=utf8;

CREATE TABLE `search_profile` (
  `login` varchar(200) NOT NULL,
  `profile_name` varchar(200) NOT NULL,
  `profile_type` varchar(30) NOT NULL,
  `profile_key` varchar(200) NOT NULL,
  `profile_value` varchar(200) DEFAULT NULL,
  KEY `search_profile_login` (`login`),
  KEY `search_profile_profile_name` (`profile_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `service` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `service_name` (`name`),
  KEY `FK_service_create_by_id` (`create_by`),
  KEY `FK_service_change_by_id` (`change_by`),
  CONSTRAINT `FK_service_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_service_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `service_customer_user` (
  `customer_user_login` varchar(200) NOT NULL,
  `service_id` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  KEY `service_customer_user_customer_user_login` (`customer_user_login`(10)),
  KEY `service_customer_user_service_id` (`service_id`),
  KEY `FK_service_customer_user_create_by_id` (`create_by`),
  CONSTRAINT `FK_service_customer_user_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_service_customer_user_service_id_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `service_preferences` (
  `service_id` int(11) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` varchar(250) DEFAULT NULL,
  KEY `service_preferences_service_id` (`service_id`),
  CONSTRAINT `FK_service_preferences_service_id_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `service_sla` (
  `service_id` int(11) NOT NULL,
  `sla_id` int(11) NOT NULL,
  UNIQUE KEY `service_sla_service_sla` (`service_id`,`sla_id`),
  KEY `FK_service_sla_sla_id_id` (`sla_id`),
  CONSTRAINT `FK_service_sla_service_id_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`),
  CONSTRAINT `FK_service_sla_sla_id_id` FOREIGN KEY (`sla_id`) REFERENCES `sla` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `sessions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `session_id` varchar(100) NOT NULL,
  `data_key` varchar(100) NOT NULL,
  `data_value` text,
  `serialized` smallint(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `sessions_data_key` (`data_key`),
  KEY `sessions_session_id_data_key` (`session_id`,`data_key`)
) ENGINE=InnoDB AUTO_INCREMENT=699 DEFAULT CHARSET=utf8;

CREATE TABLE `signature` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `text` text NOT NULL,
  `content_type` varchar(250) DEFAULT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `signature_name` (`name`),
  KEY `FK_signature_create_by_id` (`create_by`),
  KEY `FK_signature_change_by_id` (`change_by`),
  KEY `FK_signature_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_signature_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_signature_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_signature_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8;

CREATE TABLE `sla` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `calendar_name` varchar(100) DEFAULT NULL,
  `first_response_time` int(11) NOT NULL,
  `first_response_notify` smallint(6) DEFAULT NULL,
  `update_time` int(11) NOT NULL,
  `update_notify` smallint(6) DEFAULT NULL,
  `solution_time` int(11) NOT NULL,
  `solution_notify` smallint(6) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `sla_name` (`name`),
  KEY `FK_sla_create_by_id` (`create_by`),
  KEY `FK_sla_change_by_id` (`change_by`),
  CONSTRAINT `FK_sla_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sla_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `sla_preferences` (
  `sla_id` int(11) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` varchar(250) DEFAULT NULL,
  KEY `sla_preferences_sla_id` (`sla_id`),
  CONSTRAINT `FK_sla_preferences_sla_id_id` FOREIGN KEY (`sla_id`) REFERENCES `sla` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `smime_signer_cert_relations` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `cert_hash` varchar(8) NOT NULL,
  `cert_fingerprint` varchar(59) NOT NULL,
  `ca_hash` varchar(8) NOT NULL,
  `ca_fingerprint` varchar(59) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_smime_signer_cert_relations_create_by_id` (`create_by`),
  KEY `FK_smime_signer_cert_relations_change_by_id` (`change_by`),
  CONSTRAINT `FK_smime_signer_cert_relations_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_smime_signer_cert_relations_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `standard_attachment` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `content_type` varchar(250) NOT NULL,
  `content` longblob NOT NULL,
  `filename` varchar(250) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `standard_attachment_name` (`name`),
  KEY `FK_standard_attachment_create_by_id` (`create_by`),
  KEY `FK_standard_attachment_change_by_id` (`change_by`),
  KEY `FK_standard_attachment_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_standard_attachment_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_standard_attachment_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_standard_attachment_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8;

CREATE TABLE `standard_template` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `text` text,
  `content_type` varchar(250) DEFAULT NULL,
  `template_type` varchar(100) NOT NULL DEFAULT 'Answer',
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `standard_template_name` (`name`),
  KEY `FK_standard_template_create_by_id` (`create_by`),
  KEY `FK_standard_template_change_by_id` (`change_by`),
  KEY `FK_standard_template_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_standard_template_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_standard_template_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_standard_template_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `standard_template_attachment` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `standard_attachment_id` int(11) NOT NULL,
  `standard_template_id` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_standard_template_attachment_standard_attachment_id_id` (`standard_attachment_id`),
  KEY `FK_standard_template_attachment_standard_template_id_id` (`standard_template_id`),
  KEY `FK_standard_template_attachment_create_by_id` (`create_by`),
  KEY `FK_standard_template_attachment_change_by_id` (`change_by`),
  CONSTRAINT `FK_standard_template_attachment_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_standard_template_attachment_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_standard_template_attachment_standard_attachment_id_id` FOREIGN KEY (`standard_attachment_id`) REFERENCES `standard_attachment` (`id`),
  CONSTRAINT `FK_standard_template_attachment_standard_template_id_id` FOREIGN KEY (`standard_template_id`) REFERENCES `standard_template` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `sysconfig_default` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(250) NOT NULL,
  `description` longblob NOT NULL,
  `navigation` varchar(200) NOT NULL,
  `is_invisible` smallint(6) NOT NULL,
  `is_readonly` smallint(6) NOT NULL,
  `is_required` smallint(6) NOT NULL,
  `is_valid` smallint(6) NOT NULL,
  `has_configlevel` smallint(6) NOT NULL,
  `user_modification_possible` smallint(6) NOT NULL,
  `user_modification_active` smallint(6) NOT NULL,
  `user_preferences_group` varchar(250) DEFAULT NULL,
  `xml_content_raw` longblob NOT NULL,
  `xml_content_parsed` longblob NOT NULL,
  `xml_filename` varchar(250) NOT NULL,
  `effective_value` longblob NOT NULL,
  `is_dirty` smallint(6) NOT NULL,
  `exclusive_lock_guid` varchar(32) NOT NULL,
  `exclusive_lock_user_id` int(11) DEFAULT NULL,
  `exclusive_lock_expiry_time` datetime DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `sysconfig_default_name` (`name`),
  KEY `FK_sysconfig_default_create_by_id` (`create_by`),
  KEY `FK_sysconfig_default_change_by_id` (`change_by`),
  KEY `FK_sysconfig_default_exclusive_lock_user_id_id` (`exclusive_lock_user_id`),
  CONSTRAINT `FK_sysconfig_default_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_default_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_default_exclusive_lock_user_id_id` FOREIGN KEY (`exclusive_lock_user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1925 DEFAULT CHARSET=utf8;

CREATE TABLE `sysconfig_default_version` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `sysconfig_default_id` int(11) DEFAULT NULL,
  `name` varchar(250) NOT NULL,
  `description` longblob NOT NULL,
  `navigation` varchar(200) NOT NULL,
  `is_invisible` smallint(6) NOT NULL,
  `is_readonly` smallint(6) NOT NULL,
  `is_required` smallint(6) NOT NULL,
  `is_valid` smallint(6) NOT NULL,
  `has_configlevel` smallint(6) NOT NULL,
  `user_modification_possible` smallint(6) NOT NULL,
  `user_modification_active` smallint(6) NOT NULL,
  `user_preferences_group` varchar(250) DEFAULT NULL,
  `xml_content_raw` longblob NOT NULL,
  `xml_content_parsed` longblob NOT NULL,
  `xml_filename` varchar(250) NOT NULL,
  `effective_value` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `scfv_sysconfig_default_id_name` (`sysconfig_default_id`,`name`),
  KEY `FK_sysconfig_default_version_create_by_id` (`create_by`),
  KEY `FK_sysconfig_default_version_change_by_id` (`change_by`),
  CONSTRAINT `FK_sysconfig_default_version_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_default_version_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_default_version_sysconfig_default_id_id` FOREIGN KEY (`sysconfig_default_id`) REFERENCES `sysconfig_default` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1925 DEFAULT CHARSET=utf8;

CREATE TABLE `sysconfig_deployment` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `comments` varchar(250) DEFAULT NULL,
  `user_id` int(11) DEFAULT NULL,
  `effective_value` longblob NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_sysconfig_deployment_user_id_id` (`user_id`),
  KEY `FK_sysconfig_deployment_create_by_id` (`create_by`),
  CONSTRAINT `FK_sysconfig_deployment_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_deployment_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8;

CREATE TABLE `sysconfig_deployment_lock` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `exclusive_lock_guid` varchar(32) DEFAULT NULL,
  `exclusive_lock_user_id` int(11) DEFAULT NULL,
  `exclusive_lock_expiry_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_sysconfig_deployment_lock_exclusive_lock_user_id_id` (`exclusive_lock_user_id`),
  CONSTRAINT `FK_sysconfig_deployment_lock_exclusive_lock_user_id_id` FOREIGN KEY (`exclusive_lock_user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `sysconfig_modified` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `sysconfig_default_id` int(11) NOT NULL,
  `name` varchar(250) NOT NULL,
  `user_id` int(11) DEFAULT NULL,
  `is_valid` smallint(6) NOT NULL,
  `user_modification_active` smallint(6) NOT NULL,
  `effective_value` longblob NOT NULL,
  `is_dirty` smallint(6) NOT NULL,
  `reset_to_default` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `sysconfig_modified_per_user` (`sysconfig_default_id`,`user_id`),
  KEY `FK_sysconfig_modified_user_id_id` (`user_id`),
  KEY `FK_sysconfig_modified_create_by_id` (`create_by`),
  KEY `FK_sysconfig_modified_change_by_id` (`change_by`),
  CONSTRAINT `FK_sysconfig_modified_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_modified_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_modified_sysconfig_default_id_id` FOREIGN KEY (`sysconfig_default_id`) REFERENCES `sysconfig_default` (`id`),
  CONSTRAINT `FK_sysconfig_modified_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `sysconfig_modified_version` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `sysconfig_default_version_id` int(11) NOT NULL,
  `name` varchar(250) NOT NULL,
  `user_id` int(11) DEFAULT NULL,
  `is_valid` smallint(6) NOT NULL,
  `user_modification_active` smallint(6) NOT NULL,
  `effective_value` longblob NOT NULL,
  `reset_to_default` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_sysconfig_modified_version_sysconfig_default_version_idaf` (`sysconfig_default_version_id`),
  KEY `FK_sysconfig_modified_version_user_id_id` (`user_id`),
  KEY `FK_sysconfig_modified_version_create_by_id` (`create_by`),
  KEY `FK_sysconfig_modified_version_change_by_id` (`change_by`),
  CONSTRAINT `FK_sysconfig_modified_version_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_modified_version_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_sysconfig_modified_version_sysconfig_default_version_idaf` FOREIGN KEY (`sysconfig_default_version_id`) REFERENCES `sysconfig_default_version` (`id`),
  CONSTRAINT `FK_sysconfig_modified_version_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `system_address` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `value0` varchar(200) NOT NULL,
  `value1` varchar(200) NOT NULL,
  `value2` varchar(200) DEFAULT NULL,
  `value3` varchar(200) DEFAULT NULL,
  `queue_id` int(11) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_system_address_create_by_id` (`create_by`),
  KEY `FK_system_address_change_by_id` (`change_by`),
  KEY `FK_system_address_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_system_address_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_system_address_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_system_address_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8;

CREATE TABLE `system_data` (
  `data_key` varchar(160) NOT NULL,
  `data_value` longblob,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`data_key`),
  KEY `FK_system_data_create_by_id` (`create_by`),
  KEY `FK_system_data_change_by_id` (`change_by`),
  CONSTRAINT `FK_system_data_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_system_data_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `system_maintenance` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `start_date` int(11) NOT NULL,
  `stop_date` int(11) NOT NULL,
  `comments` varchar(250) NOT NULL,
  `login_message` varchar(250) DEFAULT NULL,
  `show_login_message` smallint(6) DEFAULT NULL,
  `notify_message` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `FK_system_maintenance_create_by_id` (`create_by`),
  KEY `FK_system_maintenance_change_by_id` (`change_by`),
  KEY `FK_system_maintenance_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_system_maintenance_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_system_maintenance_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_system_maintenance_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `ticket` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `tn` varchar(50) NOT NULL,
  `title` varchar(255) DEFAULT NULL,
  `queue_id` int(11) NOT NULL,
  `ticket_lock_id` smallint(6) NOT NULL,
  `type_id` smallint(6) DEFAULT NULL,
  `service_id` int(11) DEFAULT NULL,
  `sla_id` int(11) DEFAULT NULL,
  `user_id` int(11) NOT NULL,
  `responsible_user_id` int(11) NOT NULL,
  `ticket_priority_id` smallint(6) NOT NULL,
  `ticket_state_id` smallint(6) NOT NULL,
  `customer_id` varchar(150) DEFAULT NULL,
  `customer_user_id` varchar(250) DEFAULT NULL,
  `timeout` int(11) NOT NULL,
  `until_time` int(11) NOT NULL,
  `escalation_time` int(11) NOT NULL,
  `escalation_update_time` int(11) NOT NULL,
  `escalation_response_time` int(11) NOT NULL,
  `escalation_solution_time` int(11) NOT NULL,
  `archive_flag` smallint(6) NOT NULL DEFAULT '0',
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_tn` (`tn`),
  KEY `ticket_archive_flag` (`archive_flag`),
  KEY `ticket_create_time` (`create_time`),
  KEY `ticket_customer_id` (`customer_id`),
  KEY `ticket_customer_user_id` (`customer_user_id`),
  KEY `ticket_escalation_response_time` (`escalation_response_time`),
  KEY `ticket_escalation_solution_time` (`escalation_solution_time`),
  KEY `ticket_escalation_time` (`escalation_time`),
  KEY `ticket_escalation_update_time` (`escalation_update_time`),
  KEY `ticket_queue_id` (`queue_id`),
  KEY `ticket_queue_view` (`ticket_state_id`,`ticket_lock_id`),
  KEY `ticket_responsible_user_id` (`responsible_user_id`),
  KEY `ticket_ticket_lock_id` (`ticket_lock_id`),
  KEY `ticket_ticket_priority_id` (`ticket_priority_id`),
  KEY `ticket_ticket_state_id` (`ticket_state_id`),
  KEY `ticket_timeout` (`timeout`),
  KEY `ticket_title` (`title`),
  KEY `ticket_type_id` (`type_id`),
  KEY `ticket_until_time` (`until_time`),
  KEY `ticket_user_id` (`user_id`),
  KEY `FK_ticket_service_id_id` (`service_id`),
  KEY `FK_ticket_sla_id_id` (`sla_id`),
  KEY `FK_ticket_create_by_id` (`create_by`),
  KEY `FK_ticket_change_by_id` (`change_by`),
  CONSTRAINT `FK_ticket_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`),
  CONSTRAINT `FK_ticket_responsible_user_id_id` FOREIGN KEY (`responsible_user_id`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_service_id_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`),
  CONSTRAINT `FK_ticket_sla_id_id` FOREIGN KEY (`sla_id`) REFERENCES `sla` (`id`),
  CONSTRAINT `FK_ticket_ticket_lock_id_id` FOREIGN KEY (`ticket_lock_id`) REFERENCES `ticket_lock_type` (`id`),
  CONSTRAINT `FK_ticket_ticket_priority_id_id` FOREIGN KEY (`ticket_priority_id`) REFERENCES `ticket_priority` (`id`),
  CONSTRAINT `FK_ticket_ticket_state_id_id` FOREIGN KEY (`ticket_state_id`) REFERENCES `ticket_state` (`id`),
  CONSTRAINT `FK_ticket_type_id_id` FOREIGN KEY (`type_id`) REFERENCES `ticket_type` (`id`),
  CONSTRAINT `FK_ticket_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_flag` (
  `ticket_id` bigint(20) NOT NULL,
  `ticket_key` varchar(50) NOT NULL,
  `ticket_value` varchar(50) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  UNIQUE KEY `ticket_flag_per_user` (`ticket_id`,`ticket_key`,`create_by`),
  KEY `ticket_flag_ticket_id` (`ticket_id`),
  KEY `ticket_flag_ticket_id_create_by` (`ticket_id`,`create_by`),
  KEY `ticket_flag_ticket_id_ticket_key` (`ticket_id`,`ticket_key`),
  KEY `FK_ticket_flag_create_by_id` (`create_by`),
  CONSTRAINT `FK_ticket_flag_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_flag_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_history` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `history_type_id` smallint(6) NOT NULL,
  `ticket_id` bigint(20) NOT NULL,
  `article_id` bigint(20) DEFAULT NULL,
  `type_id` smallint(6) NOT NULL,
  `queue_id` int(11) NOT NULL,
  `owner_id` int(11) NOT NULL,
  `priority_id` smallint(6) NOT NULL,
  `state_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ticket_history_article_id` (`article_id`),
  KEY `ticket_history_create_time` (`create_time`),
  KEY `ticket_history_history_type_id` (`history_type_id`),
  KEY `ticket_history_owner_id` (`owner_id`),
  KEY `ticket_history_priority_id` (`priority_id`),
  KEY `ticket_history_queue_id` (`queue_id`),
  KEY `ticket_history_state_id` (`state_id`),
  KEY `ticket_history_ticket_id` (`ticket_id`),
  KEY `ticket_history_type_id` (`type_id`),
  KEY `FK_ticket_history_create_by_id` (`create_by`),
  KEY `FK_ticket_history_change_by_id` (`change_by`),
  CONSTRAINT `FK_ticket_history_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`),
  CONSTRAINT `FK_ticket_history_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_history_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_history_history_type_id_id` FOREIGN KEY (`history_type_id`) REFERENCES `ticket_history_type` (`id`),
  CONSTRAINT `FK_ticket_history_owner_id_id` FOREIGN KEY (`owner_id`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_history_priority_id_id` FOREIGN KEY (`priority_id`) REFERENCES `ticket_priority` (`id`),
  CONSTRAINT `FK_ticket_history_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`),
  CONSTRAINT `FK_ticket_history_state_id_id` FOREIGN KEY (`state_id`) REFERENCES `ticket_state` (`id`),
  CONSTRAINT `FK_ticket_history_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`),
  CONSTRAINT `FK_ticket_history_type_id_id` FOREIGN KEY (`type_id`) REFERENCES `ticket_type` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=51 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_history_type` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_history_type_name` (`name`),
  KEY `FK_ticket_history_type_create_by_id` (`create_by`),
  KEY `FK_ticket_history_type_change_by_id` (`change_by`),
  KEY `FK_ticket_history_type_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_ticket_history_type_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_history_type_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_history_type_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=52 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_index` (
  `ticket_id` bigint(20) NOT NULL,
  `queue_id` int(11) NOT NULL,
  `queue` varchar(200) NOT NULL,
  `group_id` int(11) NOT NULL,
  `s_lock` varchar(200) NOT NULL,
  `s_state` varchar(200) NOT NULL,
  `create_time` datetime NOT NULL,
  KEY `ticket_index_group_id` (`group_id`),
  KEY `ticket_index_queue_id` (`queue_id`),
  KEY `ticket_index_ticket_id` (`ticket_id`),
  CONSTRAINT `FK_ticket_index_group_id_id` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`),
  CONSTRAINT `FK_ticket_index_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`),
  CONSTRAINT `FK_ticket_index_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_lock_index` (
  `ticket_id` bigint(20) NOT NULL,
  KEY `ticket_lock_index_ticket_id` (`ticket_id`),
  CONSTRAINT `FK_ticket_lock_index_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_lock_type` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_lock_type_name` (`name`),
  KEY `FK_ticket_lock_type_create_by_id` (`create_by`),
  KEY `FK_ticket_lock_type_change_by_id` (`change_by`),
  KEY `FK_ticket_lock_type_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_ticket_lock_type_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_lock_type_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_lock_type_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_loop_protection` (
  `sent_to` varchar(250) NOT NULL,
  `sent_date` varchar(150) NOT NULL,
  KEY `ticket_loop_protection_sent_date` (`sent_date`),
  KEY `ticket_loop_protection_sent_to` (`sent_to`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_number_counter` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `counter` bigint(20) NOT NULL,
  `counter_uid` varchar(32) NOT NULL,
  `create_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_number_counter_uid` (`counter_uid`),
  KEY `ticket_number_counter_create_time` (`create_time`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_priority` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_priority_name` (`name`),
  KEY `FK_ticket_priority_create_by_id` (`create_by`),
  KEY `FK_ticket_priority_change_by_id` (`change_by`),
  CONSTRAINT `FK_ticket_priority_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_priority_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_state` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `type_id` smallint(6) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_state_name` (`name`),
  KEY `FK_ticket_state_type_id_id` (`type_id`),
  KEY `FK_ticket_state_create_by_id` (`create_by`),
  KEY `FK_ticket_state_change_by_id` (`change_by`),
  KEY `FK_ticket_state_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_ticket_state_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_state_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_state_type_id_id` FOREIGN KEY (`type_id`) REFERENCES `ticket_state_type` (`id`),
  CONSTRAINT `FK_ticket_state_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=11 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_state_type` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_state_type_name` (`name`),
  KEY `FK_ticket_state_type_create_by_id` (`create_by`),
  KEY `FK_ticket_state_type_change_by_id` (`change_by`),
  CONSTRAINT `FK_ticket_state_type_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_state_type_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_type` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ticket_type_name` (`name`),
  KEY `FK_ticket_type_create_by_id` (`create_by`),
  KEY `FK_ticket_type_change_by_id` (`change_by`),
  KEY `FK_ticket_type_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_ticket_type_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_type_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_type_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8;

CREATE TABLE `ticket_watcher` (
  `ticket_id` bigint(20) NOT NULL,
  `user_id` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  KEY `ticket_watcher_ticket_id` (`ticket_id`),
  KEY `ticket_watcher_user_id` (`user_id`),
  KEY `FK_ticket_watcher_create_by_id` (`create_by`),
  KEY `FK_ticket_watcher_change_by_id` (`change_by`),
  CONSTRAINT `FK_ticket_watcher_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_watcher_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_ticket_watcher_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`),
  CONSTRAINT `FK_ticket_watcher_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `time_accounting` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `ticket_id` bigint(20) NOT NULL,
  `article_id` bigint(20) DEFAULT NULL,
  `time_unit` decimal(10,2) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `time_accounting_ticket_id` (`ticket_id`),
  KEY `FK_time_accounting_article_id_id` (`article_id`),
  KEY `FK_time_accounting_create_by_id` (`create_by`),
  KEY `FK_time_accounting_change_by_id` (`change_by`),
  CONSTRAINT `FK_time_accounting_article_id_id` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`),
  CONSTRAINT `FK_time_accounting_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_time_accounting_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_time_accounting_ticket_id_id` FOREIGN KEY (`ticket_id`) REFERENCES `ticket` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8;

CREATE TABLE `user_preferences` (
  `user_id` int(11) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` longblob,
  KEY `user_preferences_user_id` (`user_id`),
  CONSTRAINT `FK_user_preferences_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `users` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `login` varchar(200) NOT NULL,
  `pw` varchar(128) NOT NULL,
  `title` varchar(50) DEFAULT NULL,
  `first_name` varchar(100) NOT NULL,
  `last_name` varchar(100) NOT NULL,
  `valid_id` smallint(6) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `users_login` (`login`),
  KEY `FK_users_create_by_id` (`create_by`),
  KEY `FK_users_change_by_id` (`change_by`),
  KEY `FK_users_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_users_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_users_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_users_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8;

CREATE TABLE `valid` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `valid_name` (`name`),
  KEY `FK_valid_create_by_id` (`create_by`),
  KEY `FK_valid_change_by_id` (`change_by`),
  CONSTRAINT `FK_valid_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_valid_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8;

CREATE TABLE `virtual_fs` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `filename` text NOT NULL,
  `backend` varchar(60) NOT NULL,
  `backend_key` varchar(160) NOT NULL,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `virtual_fs_backend` (`backend`),
  KEY `virtual_fs_filename` (`filename`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `virtual_fs_db` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `filename` text NOT NULL,
  `content` longblob,
  `create_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `virtual_fs_db_filename` (`filename`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `virtual_fs_preferences` (
  `virtual_fs_id` bigint(20) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` text,
  KEY `virtual_fs_preferences_key_value` (`preferences_key`,`preferences_value`(150)),
  KEY `virtual_fs_preferences_virtual_fs_id` (`virtual_fs_id`),
  CONSTRAINT `FK_virtual_fs_preferences_virtual_fs_id_id` FOREIGN KEY (`virtual_fs_id`) REFERENCES `virtual_fs` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `web_upload_cache` (
  `form_id` varchar(250) DEFAULT NULL,
  `filename` varchar(250) DEFAULT NULL,
  `content_id` varchar(250) DEFAULT NULL,
  `content_size` varchar(30) DEFAULT NULL,
  `content_type` varchar(250) DEFAULT NULL,
  `disposition` varchar(15) DEFAULT NULL,
  `content` longblob NOT NULL,
  `create_time_unix` bigint(20) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `xml_storage` (
  `xml_type` varchar(200) NOT NULL,
  `xml_key` varchar(250) NOT NULL,
  `xml_content_key` varchar(250) NOT NULL,
  `xml_content_value` mediumtext,
  KEY `xml_storage_key_type` (`xml_key`(10),`xml_type`(10)),
  KEY `xml_storage_xml_content_key` (`xml_content_key`(100))
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

