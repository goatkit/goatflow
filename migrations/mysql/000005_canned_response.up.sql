-- Canned responses (pre-written response templates for agents)
CREATE TABLE `canned_response` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `category` varchar(100) DEFAULT NULL,
  `content` text NOT NULL,
  `content_type` varchar(50) NOT NULL DEFAULT 'text',
  `tags` text DEFAULT NULL,
  `scope` varchar(20) NOT NULL DEFAULT 'personal',
  `owner_id` int(11) NOT NULL,
  `team_id` int(11) DEFAULT NULL,
  `placeholders` text DEFAULT NULL,
  `usage_count` int(11) NOT NULL DEFAULT 0,
  `last_used` datetime DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL DEFAULT 1,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `canned_response_name` (`name`),
  KEY `canned_response_scope` (`scope`),
  KEY `canned_response_owner_id` (`owner_id`),
  KEY `canned_response_team_id` (`team_id`),
  KEY `canned_response_category` (`category`),
  KEY `FK_canned_response_create_by_id` (`create_by`),
  KEY `FK_canned_response_change_by_id` (`change_by`),
  KEY `FK_canned_response_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_canned_response_owner_id` FOREIGN KEY (`owner_id`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_canned_response_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_canned_response_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_canned_response_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

-- Canned response categories
CREATE TABLE `canned_response_category` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL,
  `description` varchar(250) DEFAULT NULL,
  `parent_id` int(11) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL DEFAULT 1,
  `create_time` datetime NOT NULL,
  `create_by` int(11) NOT NULL,
  `change_time` datetime NOT NULL,
  `change_by` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `canned_response_category_name` (`name`),
  KEY `FK_canned_response_category_parent_id` (`parent_id`),
  KEY `FK_canned_response_category_create_by_id` (`create_by`),
  KEY `FK_canned_response_category_change_by_id` (`change_by`),
  KEY `FK_canned_response_category_valid_id_id` (`valid_id`),
  CONSTRAINT `FK_canned_response_category_parent_id` FOREIGN KEY (`parent_id`) REFERENCES `canned_response_category` (`id`),
  CONSTRAINT `FK_canned_response_category_create_by_id` FOREIGN KEY (`create_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_canned_response_category_change_by_id` FOREIGN KEY (`change_by`) REFERENCES `users` (`id`),
  CONSTRAINT `FK_canned_response_category_valid_id_id` FOREIGN KEY (`valid_id`) REFERENCES `valid` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

-- Insert default categories
INSERT INTO `canned_response_category` (`name`, `description`, `valid_id`, `create_time`, `create_by`, `change_time`, `change_by`)
VALUES
  ('General', 'General purpose responses', 1, NOW(), 1, NOW(), 1),
  ('Account', 'Account-related responses', 1, NOW(), 1, NOW(), 1),
  ('Technical', 'Technical support responses', 1, NOW(), 1, NOW(), 1),
  ('Billing', 'Billing and payment responses', 1, NOW(), 1, NOW(), 1),
  ('System', 'System notifications and alerts', 1, NOW(), 1, NOW(), 1);
