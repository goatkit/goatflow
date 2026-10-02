-- GoatFlow importer fixture: OTRS 6 / Znuny 6 schema dumped by mariadb-dump.
-- Extended INSERTs are one line per table (mysqldump / MariaDB < 11), except ticket_history
-- and the permission, preference, template, notification, dynamic field, link, sysconfig and
-- session tables, which keep the one-row-per-line layout of mariadb-dump 11. Attachment 31 is
-- encoded as mysqldump --hex-blob writes it (0x...), attachment 32 as MySQL 8 writes binary
-- strings (_binary '...'); every other blob is a raw escaped string.
/*M!999999\- enable the sandbox mode */ 
-- MariaDB dump 10.19-11.8.8-MariaDB, for debian-linux-gnu (x86_64)
--
-- Host: localhost    Database: otrs_fixture_importer
-- ------------------------------------------------------
-- Server version	11.8.8-MariaDB-ubu2404

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*M!100616 SET @OLD_NOTE_VERBOSITY=@@NOTE_VERBOSITY, NOTE_VERBOSITY=0 */;

--
-- Table structure for table `article`
--

DROP TABLE IF EXISTS `article`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `article` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `ticket_id` bigint(20) NOT NULL,
  `article_sender_type_id` smallint(6) NOT NULL,
  `communication_channel_id` bigint(20) NOT NULL,
  `is_visible_for_customer` smallint(6) NOT NULL,
  `search_index_needs_rebuild` smallint(6) NOT NULL DEFAULT 1,
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
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `article`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `article` WRITE;
/*!40000 ALTER TABLE `article` DISABLE KEYS */;
INSERT INTO `article` VALUES (7,3,3,1,1,0,NULL,'2014-03-06 00:10:00',1,'2014-03-06 00:10:00',1),(12,9,1,1,1,0,NULL,'2014-03-06 10:30:00',3,'2014-03-06 10:30:00',3);
/*!40000 ALTER TABLE `article` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `article_data_mime`
--

DROP TABLE IF EXISTS `article_data_mime`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `article_data_mime` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `article_id` bigint(20) NOT NULL,
  `a_from` mediumtext DEFAULT NULL,
  `a_reply_to` mediumtext DEFAULT NULL,
  `a_to` mediumtext DEFAULT NULL,
  `a_cc` mediumtext DEFAULT NULL,
  `a_bcc` mediumtext DEFAULT NULL,
  `a_subject` text DEFAULT NULL,
  `a_message_id` text DEFAULT NULL,
  `a_message_id_md5` varchar(32) DEFAULT NULL,
  `a_in_reply_to` mediumtext DEFAULT NULL,
  `a_references` mediumtext DEFAULT NULL,
  `a_content_type` varchar(250) DEFAULT NULL,
  `a_body` mediumtext DEFAULT NULL,
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
) ENGINE=InnoDB AUTO_INCREMENT=22 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `article_data_mime`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `article_data_mime` WRITE;
/*!40000 ALTER TABLE `article_data_mime` DISABLE KEYS */;
INSERT INTO `article_data_mime` VALUES (20,7,'\"Jane Customer\" <jane@acme.example>',NULL,'support@example.com',NULL,NULL,'Printer on fire','<20140305.abc@acme.example>','a4b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8',NULL,NULL,'text/plain; charset=utf-8','Hello,\n\nthe printer\'s on fire \\ again.\r\nIt says \"PC LOAD LETTER\" — ünïcödé ✓	and a \Z sub.',1394064600,'2014/03/05','2014-03-06 00:10:00',1,'2014-03-06 00:10:00',1),(21,12,'\"Alice Smith\" <support@example.com>',NULL,'\"Bob O\'Brien\" <bob@acme.example>',NULL,NULL,'RE: Invoice question','<20140306.def@example.com>','0123456789abcdef0123456789abcdef',NULL,NULL,'text/plain; charset=utf-8','Dear Bob,\n\nplease find the invoice attached.\n\n-- \nAlice Smith\nExample Support',1394101800,'2014/03/06','2014-03-06 10:30:00',3,'2014-03-06 10:30:00',3);
/*!40000 ALTER TABLE `article_data_mime` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `article_data_mime_attachment`
--

DROP TABLE IF EXISTS `article_data_mime_attachment`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `article_data_mime_attachment` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `article_id` bigint(20) NOT NULL,
  `filename` varchar(250) DEFAULT NULL,
  `content_size` varchar(30) DEFAULT NULL,
  `content_type` text DEFAULT NULL,
  `content_id` varchar(250) DEFAULT NULL,
  `content_alternative` varchar(50) DEFAULT NULL,
  `disposition` varchar(15) DEFAULT NULL,
  `content` longblob DEFAULT NULL,
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
) ENGINE=InnoDB AUTO_INCREMENT=34 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `article_data_mime_attachment`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `article_data_mime_attachment` WRITE;
/*!40000 ALTER TABLE `article_data_mime_attachment` DISABLE KEYS */;
INSERT INTO `article_data_mime_attachment` VALUES (31,12,'invoice.pdf','20','application/pdf','','','attachment',0x255044462D312E340A00275CFF0D1A22656E640A,'2014-03-06 10:30:00',3,'2014-03-06 10:30:00',3),(32,12,'logo.png','21','image/png','<logo.png@01D03F>','','inline',_binary '�PNG\r\n\Z\n\0\0\'\'\\\\��\0IEND','2014-03-06 10:30:00',3,'2014-03-06 10:30:00',3),(33,12,'notes.txt','33','text/plain; charset=utf-8','',NULL,'attachment','notes: it\'s a \\ backslash\näöü\n','2014-03-06 10:30:00',3,'2014-03-06 10:30:00',3);
/*!40000 ALTER TABLE `article_data_mime_attachment` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `article_data_mime_plain`
--

DROP TABLE IF EXISTS `article_data_mime_plain`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=41 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `article_data_mime_plain`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `article_data_mime_plain` WRITE;
/*!40000 ALTER TABLE `article_data_mime_plain` DISABLE KEYS */;
INSERT INTO `article_data_mime_plain` VALUES (40,7,'From: \"Jane Customer\" <jane@acme.example>\r\nTo: support@example.com\r\nSubject: Printer on fire\r\nMessage-ID: <20140305.abc@acme.example>\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nHello,\r\n\r\nthe printer\'s on fire \\ again.\r\n\0�\r\n','2014-03-06 00:10:00',1,'2014-03-06 00:10:00',1);
/*!40000 ALTER TABLE `article_data_mime_plain` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `article_sender_type`
--

DROP TABLE IF EXISTS `article_sender_type`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `article_sender_type`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `article_sender_type` WRITE;
/*!40000 ALTER TABLE `article_sender_type` DISABLE KEYS */;
INSERT INTO `article_sender_type` VALUES (1,'agent','Sender type agent.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'system','Sender type system.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'customer','Sender type customer.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `article_sender_type` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `communication_channel`
--

DROP TABLE IF EXISTS `communication_channel`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `communication_channel`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `communication_channel` WRITE;
/*!40000 ALTER TABLE `communication_channel` DISABLE KEYS */;
INSERT INTO `communication_channel` VALUES (1,'Email','Kernel::System::CommunicationChannel::Email','Framework','---\nArticleDataArticleIDField: article_id\nArticleDataTables:\n- article_data_mime\n- article_data_mime_plain\n- article_data_mime_attachment\n',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'Phone','Kernel::System::CommunicationChannel::Phone','Framework','---\nArticleDataArticleIDField: article_id\nArticleDataTables:\n- article_data_mime\n- article_data_mime_plain\n- article_data_mime_attachment\n',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'Internal','Kernel::System::CommunicationChannel::Internal','Framework','---\nArticleDataArticleIDField: article_id\nArticleDataTables:\n- article_data_mime\n- article_data_mime_plain\n- article_data_mime_attachment\n',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(4,'Chat','Kernel::System::CommunicationChannel::Chat','Framework','---\nArticleDataArticleIDField: article_id\n',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `communication_channel` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `customer_company`
--

DROP TABLE IF EXISTS `customer_company`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `customer_company`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `customer_company` WRITE;
/*!40000 ALTER TABLE `customer_company` DISABLE KEYS */;
INSERT INTO `customer_company` VALUES ('ACME','ACME Corp.','Main St 1','12345','Springfield','US','https://acme.example','Key account',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `customer_company` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `customer_user`
--

DROP TABLE IF EXISTS `customer_user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `customer_user`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `customer_user` WRITE;
/*!40000 ALTER TABLE `customer_user` DISABLE KEYS */;
INSERT INTO `customer_user` VALUES (5,'jane','jane@acme.example','ACME',NULL,'Ms','Jane','Customer','+1 555 0100',NULL,NULL,'Main St 1','12345','Springfield','US','',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(8,'bob','bob@acme.example','ACME',NULL,NULL,'Bob','O\'Brien',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `customer_user` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `follow_up_possible`
--

DROP TABLE IF EXISTS `follow_up_possible`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `follow_up_possible`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `follow_up_possible` WRITE;
/*!40000 ALTER TABLE `follow_up_possible` DISABLE KEYS */;
INSERT INTO `follow_up_possible` VALUES (1,'possible','Follow-ups for closed tickets are possible. Ticket will be reopened.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'reject','Follow-ups for closed tickets are not possible. No new ticket will be created.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'new ticket','Follow-ups for closed tickets are not possible. A new ticket will be created.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `follow_up_possible` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `groups`
--

DROP TABLE IF EXISTS `groups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `groups`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `groups` WRITE;
/*!40000 ALTER TABLE `groups` DISABLE KEYS */;
INSERT INTO `groups` VALUES (1,'users','Group for default access.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'admin','Group of all administrators.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'stats','Group for statistics access.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(4,'support','Support agents',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),(5,'billing','Billing agents',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `groups` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `queue`
--

DROP TABLE IF EXISTS `queue`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `queue`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `queue` WRITE;
/*!40000 ALTER TABLE `queue` DISABLE KEYS */;
INSERT INTO `queue` VALUES (1,'Postmaster',1,0,0,0,0,0,0,0,1,NULL,NULL,1,1,1,0,'Postmaster queue.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'Raw',1,0,0,0,0,0,0,0,1,NULL,NULL,1,1,1,0,'All default incoming tickets.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'Junk',1,0,0,0,0,0,0,0,1,NULL,NULL,1,1,1,0,'All junk tickets.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(4,'Misc',1,0,0,0,0,0,0,0,1,NULL,NULL,1,1,1,0,'All misc tickets.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(5,'Support',4,0,60,0,0,0,480,0,2,'',NULL,1,1,3,1,'Customer support.',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),(6,'Billing',5,0,0,0,0,0,0,0,2,'',NULL,1,1,1,0,'Invoices and refunds.',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `queue` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `salutation`
--

DROP TABLE IF EXISTS `salutation`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `salutation`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `salutation` WRITE;
/*!40000 ALTER TABLE `salutation` DISABLE KEYS */;
INSERT INTO `salutation` VALUES (1,'system standard salutation (en)','Dear <OTRS_CUSTOMER_REALNAME>,\n\nThank you for your request.\n','text/plain; charset=utf-8','Standard Salutation.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `salutation` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `signature`
--

DROP TABLE IF EXISTS `signature`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `signature`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `signature` WRITE;
/*!40000 ALTER TABLE `signature` DISABLE KEYS */;
INSERT INTO `signature` VALUES (1,'system standard signature (en)','\nYour Ticket-Team\n\n <OTRS_Agent_UserFirstname> <OTRS_Agent_UserLastname>\n','text/plain; charset=utf-8','Standard Signature.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `signature` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `system_address`
--

DROP TABLE IF EXISTS `system_address`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `system_address`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `system_address` WRITE;
/*!40000 ALTER TABLE `system_address` DISABLE KEYS */;
INSERT INTO `system_address` VALUES (1,'otrs@localhost','OTRS System',NULL,NULL,1,'Standard Address.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'support@example.com','Example Support',NULL,NULL,5,'',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `system_address` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket`
--

DROP TABLE IF EXISTS `ticket`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
  `archive_flag` smallint(6) NOT NULL DEFAULT 0,
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
) ENGINE=InnoDB AUTO_INCREMENT=10 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket` WRITE;
/*!40000 ALTER TABLE `ticket` DISABLE KEYS */;
INSERT INTO `ticket` VALUES (3,'2014030510000013','Printer on fire',1,1,1,NULL,NULL,2,2,3,4,'ACME','jane',0,0,0,0,0,0,0,'2014-03-06 00:10:00',1,'2014-03-06 00:10:00',1),(9,'2014030610000091','Invoice question',5,1,2,NULL,NULL,3,3,4,2,'ACME','bob',0,0,0,0,0,0,0,'2014-03-06 10:00:00',2,'2014-03-06 11:00:00',3),(15,'2014030710000024','Refund request',6,1,1,1,1,5,5,3,4,'ACME','jane',0,0,0,0,0,0,0,'2014-03-07 09:00:00',5,'2014-03-07 09:00:00',5);
/*!40000 ALTER TABLE `ticket` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_history`
--

DROP TABLE IF EXISTS `ticket_history`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=103 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_history`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_history` WRITE;
/*!40000 ALTER TABLE `ticket_history` DISABLE KEYS */;
INSERT INTO `ticket_history` VALUES
(100,'%%2014030510000013%%Postmaster%%3 normal%%open%%3',1,3,7,1,1,2,3,4,'2014-03-06 00:10:00',1,'2014-03-06 00:10:00',1),
(101,'%%bob@acme.example, ',8,9,12,2,5,3,4,2,'2014-03-06 10:30:00',3,'2014-03-06 10:30:00',3),
(102,'%%open%%closed successful%%',27,9,NULL,2,5,3,4,2,'2014-03-06 11:00:00',3,'2014-03-06 11:00:00',3);
/*!40000 ALTER TABLE `ticket_history` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_history_type`
--

DROP TABLE IF EXISTS `ticket_history_type`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=28 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_history_type`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_history_type` WRITE;
/*!40000 ALTER TABLE `ticket_history_type` DISABLE KEYS */;
INSERT INTO `ticket_history_type` VALUES (1,'NewTicket','',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(8,'SendAnswer','',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(12,'EmailCustomer','',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(16,'Move','',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(27,'StateUpdate','',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `ticket_history_type` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_lock_type`
--

DROP TABLE IF EXISTS `ticket_lock_type`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_lock_type`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_lock_type` WRITE;
/*!40000 ALTER TABLE `ticket_lock_type` DISABLE KEYS */;
INSERT INTO `ticket_lock_type` VALUES (1,'unlock',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'lock',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'tmp_lock',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `ticket_lock_type` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_priority`
--

DROP TABLE IF EXISTS `ticket_priority`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_priority`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_priority` WRITE;
/*!40000 ALTER TABLE `ticket_priority` DISABLE KEYS */;
INSERT INTO `ticket_priority` VALUES (1,'1 very low',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'2 low',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'3 normal',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(4,'4 high',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(5,'5 very high',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `ticket_priority` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_state`
--

DROP TABLE IF EXISTS `ticket_state`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=10 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_state`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_state` WRITE;
/*!40000 ALTER TABLE `ticket_state` DISABLE KEYS */;
INSERT INTO `ticket_state` VALUES (1,'new','New ticket created by customer.',1,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'closed successful','Ticket is closed successful.',3,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'closed unsuccessful','Ticket is closed unsuccessful.',3,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(4,'open','Open tickets.',2,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(5,'removed','Customer removed ticket.',6,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(6,'pending reminder','Ticket is pending for agent reminder.',4,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(7,'pending auto close+','Ticket is pending for automatic close.',5,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(8,'pending auto close-','Ticket is pending for automatic close.',5,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(9,'merged','State for merged tickets.',7,1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `ticket_state` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_state_type`
--

DROP TABLE IF EXISTS `ticket_state_type`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_state_type`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_state_type` WRITE;
/*!40000 ALTER TABLE `ticket_state_type` DISABLE KEYS */;
INSERT INTO `ticket_state_type` VALUES (1,'new','All new state types (default: viewable).','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'open','All open state types (default: viewable).','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'closed','All closed state types (default: not viewable).','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(4,'pending reminder','All \'pending reminder\' state types (default: viewable).','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(5,'pending auto','All \'pending auto *\' state types (default: viewable).','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(6,'removed','All \'removed\' state types (default: not viewable).','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(7,'merged','State type for merged tickets (default: not viewable).','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `ticket_state_type` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_type`
--

DROP TABLE IF EXISTS `ticket_type`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_type`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_type` WRITE;
/*!40000 ALTER TABLE `ticket_type` DISABLE KEYS */;
INSERT INTO `ticket_type` VALUES (1,'Unclassified',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'Problem',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `ticket_type` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `users`
--

DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `users`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `users` WRITE;
/*!40000 ALTER TABLE `users` DISABLE KEYS */;
INSERT INTO `users` VALUES (1,'root@localhost','$2a$12$K9rNSSLSkFjYCa7PeUGJFurWvW7o.0QZoYHqpm2b7r1n7Vf2xQ1l2','','Admin','OTRS',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'jdoe','$2a$12$3m0Q4Vn0RYnqK0q9mJ9oP.4QJ8pX8m5n5Q0QXUQ6qX2bB0KZbY5uO',NULL,'John','Doe',1,'2014-02-01 09:00:00',1,'2014-03-01 12:00:00',3),(3,'asmith','$2a$12$Wl1v8H9m1K8a5w0cBv8xOe2rq8Q8yqXJm0W8o6s2Q5R0s1a2b3c4d',NULL,'Alice','Smith',1,'2014-02-02 09:00:00',2,'2014-02-02 09:00:00',2),(4,'kgroup','0f7a28def39f55f118b12fa07c73885e9ed4bbb09598bb51a33f682857948cdb',NULL,'Karl','Group',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),(5,'lrole','BCRYPT:5:Q7rTx2LmP9vKa4Zc:GaMb8oyK3GyY0ZLg4MKgt8VZP5.ZARi',NULL,'Lena','Role',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),(6,'mnone','bf1da987e3ecd80090eb6821b65df98d2820d9454fa5cd10137421d5f6fcf886',NULL,'Max','None',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `users` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `valid`
--

DROP TABLE IF EXISTS `valid`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `valid`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `valid` WRITE;
/*!40000 ALTER TABLE `valid` DISABLE KEYS */;
INSERT INTO `valid` VALUES (1,'valid','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(2,'invalid','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),(3,'invalid-temporarily','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `valid` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;
--
-- Table structure for table `roles`
--

DROP TABLE IF EXISTS `roles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `roles`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `roles` WRITE;
/*!40000 ALTER TABLE `roles` DISABLE KEYS */;
INSERT INTO `roles` VALUES
(1,'Support Team','First-line support',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(2,'Billing Team','Invoices and refunds',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `roles` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `role_user`
--

DROP TABLE IF EXISTS `role_user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `role_user`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `role_user` WRITE;
/*!40000 ALTER TABLE `role_user` DISABLE KEYS */;
INSERT INTO `role_user` VALUES
(3,1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(5,2,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `role_user` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `group_role`
--

DROP TABLE IF EXISTS `group_role`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `group_role`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `group_role` WRITE;
/*!40000 ALTER TABLE `group_role` DISABLE KEYS */;
INSERT INTO `group_role` VALUES
(1,4,'ro',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(2,5,'rw',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(2,4,'rw',0,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `group_role` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `group_user`
--

DROP TABLE IF EXISTS `group_user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `group_user`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `group_user` WRITE;
/*!40000 ALTER TABLE `group_user` DISABLE KEYS */;
INSERT INTO `group_user` VALUES
(1,1,'rw','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(1,2,'rw','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(1,3,'rw','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(1,4,'rw','2014-02-03 10:00:00',1,'2014-02-03 10:00:00',1),
(2,1,'rw','2014-02-03 10:00:00',1,'2014-02-03 10:00:00',1),
(3,4,'rw','2014-02-03 10:00:00',1,'2014-02-03 10:00:00',1),
(4,4,'rw','2014-02-03 10:00:00',1,'2014-02-03 10:00:00',1),
(4,4,'note','2014-02-03 10:00:00',1,'2014-02-03 10:00:00',1);
/*!40000 ALTER TABLE `group_user` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `group_customer_user`
--

DROP TABLE IF EXISTS `group_customer_user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `group_customer_user`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `group_customer_user` WRITE;
/*!40000 ALTER TABLE `group_customer_user` DISABLE KEYS */;
INSERT INTO `group_customer_user` VALUES
('jane',4,'ro',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
('jane',4,'rw',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `group_customer_user` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `user_preferences`
--

DROP TABLE IF EXISTS `user_preferences`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `user_preferences` (
  `user_id` int(11) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` longblob DEFAULT NULL,
  KEY `user_preferences_user_id` (`user_id`),
  CONSTRAINT `FK_user_preferences_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `user_preferences`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `user_preferences` WRITE;
/*!40000 ALTER TABLE `user_preferences` DISABLE KEYS */;
INSERT INTO `user_preferences` VALUES
(1,'UserLanguage','en'),
(4,'UserLanguage','de'),
(4,'UserSignature','Grüße,\nKarl');
/*!40000 ALTER TABLE `user_preferences` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `customer_preferences`
--

DROP TABLE IF EXISTS `customer_preferences`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `customer_preferences` (
  `user_id` varchar(250) NOT NULL,
  `preferences_key` varchar(150) NOT NULL,
  `preferences_value` varchar(250) DEFAULT NULL,
  KEY `customer_preferences_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `customer_preferences`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `customer_preferences` WRITE;
/*!40000 ALTER TABLE `customer_preferences` DISABLE KEYS */;
INSERT INTO `customer_preferences` VALUES
('jane','UserLanguage','fr'),
('bob','UserShowTickets','25');
/*!40000 ALTER TABLE `customer_preferences` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `customer_user_customer`
--

DROP TABLE IF EXISTS `customer_user_customer`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `customer_user_customer`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `customer_user_customer` WRITE;
/*!40000 ALTER TABLE `customer_user_customer` DISABLE KEYS */;
INSERT INTO `customer_user_customer` VALUES
('bob','ACME','2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `customer_user_customer` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `dynamic_field`
--

DROP TABLE IF EXISTS `dynamic_field`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `dynamic_field` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `internal_field` smallint(6) NOT NULL DEFAULT 0,
  `name` varchar(200) NOT NULL,
  `label` varchar(200) NOT NULL,
  `field_order` int(11) NOT NULL,
  `field_type` varchar(200) NOT NULL,
  `object_type` varchar(100) NOT NULL,
  `config` longblob DEFAULT NULL,
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `dynamic_field`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `dynamic_field` WRITE;
/*!40000 ALTER TABLE `dynamic_field` DISABLE KEYS */;
INSERT INTO `dynamic_field` VALUES
(1,0,'CustomerReference','Customer reference',1,'Text','Ticket','---\nDefaultValue: \'\'\nLink: \'\'\n',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(2,0,'NoteKind','Note kind',2,'Dropdown','Article','---\nPossibleValues:\n  internal: Internal\n  public: Public\n',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `dynamic_field` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `dynamic_field_value`
--

DROP TABLE IF EXISTS `dynamic_field_value`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `dynamic_field_value` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `field_id` int(11) NOT NULL,
  `object_id` bigint(20) NOT NULL,
  `value_text` text DEFAULT NULL,
  `value_date` datetime DEFAULT NULL,
  `value_int` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `dynamic_field_value_field_values` (`object_id`,`field_id`),
  KEY `dynamic_field_value_search_date` (`field_id`,`value_date`),
  KEY `dynamic_field_value_search_int` (`field_id`,`value_int`),
  KEY `dynamic_field_value_search_text` (`field_id`,`value_text`(150)),
  CONSTRAINT `FK_dynamic_field_value_field_id_id` FOREIGN KEY (`field_id`) REFERENCES `dynamic_field` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `dynamic_field_value`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `dynamic_field_value` WRITE;
/*!40000 ALTER TABLE `dynamic_field_value` DISABLE KEYS */;
INSERT INTO `dynamic_field_value` VALUES
(1,1,9,'PO-4711',NULL,NULL),
(2,2,12,'internal',NULL,NULL),
(3,1,3,'REF-0815',NULL,NULL);
/*!40000 ALTER TABLE `dynamic_field_value` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `time_accounting`
--

DROP TABLE IF EXISTS `time_accounting`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `time_accounting`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `time_accounting` WRITE;
/*!40000 ALTER TABLE `time_accounting` DISABLE KEYS */;
INSERT INTO `time_accounting` VALUES
(1,9,12,15.50,'2014-03-06 10:30:00',3,'2014-03-06 10:30:00',3),
(2,3,NULL,0.25,'2014-03-06 10:30:00',2,'2014-03-06 10:30:00',2);
/*!40000 ALTER TABLE `time_accounting` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_flag`
--

DROP TABLE IF EXISTS `ticket_flag`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_flag`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_flag` WRITE;
/*!40000 ALTER TABLE `ticket_flag` DISABLE KEYS */;
INSERT INTO `ticket_flag` VALUES
(3,'Seen','1','2014-03-06 10:30:00',2),
(9,'Seen','1','2014-03-06 10:30:00',3);
/*!40000 ALTER TABLE `ticket_flag` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `article_flag`
--

DROP TABLE IF EXISTS `article_flag`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `article_flag`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `article_flag` WRITE;
/*!40000 ALTER TABLE `article_flag` DISABLE KEYS */;
INSERT INTO `article_flag` VALUES
(12,'Seen','1','2014-03-06 10:30:00',3);
/*!40000 ALTER TABLE `article_flag` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `link_object`
--

DROP TABLE IF EXISTS `link_object`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `link_object` (
  `id` smallint(6) NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `link_object_name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `link_object`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `link_object` WRITE;
/*!40000 ALTER TABLE `link_object` DISABLE KEYS */;
INSERT INTO `link_object` VALUES
(1,'Ticket');
/*!40000 ALTER TABLE `link_object` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `link_type`
--

DROP TABLE IF EXISTS `link_type`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `link_type`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `link_type` WRITE;
/*!40000 ALTER TABLE `link_type` DISABLE KEYS */;
INSERT INTO `link_type` VALUES
(1,'Normal',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(2,'ParentChild',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `link_type` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `link_state`
--

DROP TABLE IF EXISTS `link_state`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `link_state`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `link_state` WRITE;
/*!40000 ALTER TABLE `link_state` DISABLE KEYS */;
INSERT INTO `link_state` VALUES
(1,'Valid',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(2,'Temporary',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `link_state` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `link_relation`
--

DROP TABLE IF EXISTS `link_relation`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `link_relation`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `link_relation` WRITE;
/*!40000 ALTER TABLE `link_relation` DISABLE KEYS */;
INSERT INTO `link_relation` VALUES
(1,'3',1,'9',2,1,'2014-03-06 10:30:00',3);
/*!40000 ALTER TABLE `link_relation` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `ticket_watcher`
--

DROP TABLE IF EXISTS `ticket_watcher`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ticket_watcher`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `ticket_watcher` WRITE;
/*!40000 ALTER TABLE `ticket_watcher` DISABLE KEYS */;
INSERT INTO `ticket_watcher` VALUES
(9,2,'2014-03-06 10:30:00',2,'2014-03-06 10:30:00',2);
/*!40000 ALTER TABLE `ticket_watcher` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `standard_template`
--

DROP TABLE IF EXISTS `standard_template`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `standard_template` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `text` text DEFAULT NULL,
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `standard_template`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `standard_template` WRITE;
/*!40000 ALTER TABLE `standard_template` DISABLE KEYS */;
INSERT INTO `standard_template` VALUES
(1,'empty answer','','text/plain; charset=utf-8','Answer','',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(2,'Invoice copy','Dear customer,\n\nplease find a copy of your invoice attached.\n','text/plain; charset=utf-8','Answer','',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `standard_template` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `standard_attachment`
--

DROP TABLE IF EXISTS `standard_attachment`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `standard_attachment`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `standard_attachment` WRITE;
/*!40000 ALTER TABLE `standard_attachment` DISABLE KEYS */;
INSERT INTO `standard_attachment` VALUES
(1,'Terms','application/pdf','%PDF-1.3\n\0�\nterms\n','terms.pdf','',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `standard_attachment` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `standard_template_attachment`
--

DROP TABLE IF EXISTS `standard_template_attachment`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `standard_template_attachment`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `standard_template_attachment` WRITE;
/*!40000 ALTER TABLE `standard_template_attachment` DISABLE KEYS */;
INSERT INTO `standard_template_attachment` VALUES
(1,1,2,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `standard_template_attachment` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `queue_standard_template`
--

DROP TABLE IF EXISTS `queue_standard_template`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `queue_standard_template`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `queue_standard_template` WRITE;
/*!40000 ALTER TABLE `queue_standard_template` DISABLE KEYS */;
INSERT INTO `queue_standard_template` VALUES
(5,1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(5,2,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(6,2,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `queue_standard_template` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `auto_response_type`
--

DROP TABLE IF EXISTS `auto_response_type`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `auto_response_type`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `auto_response_type` WRITE;
/*!40000 ALTER TABLE `auto_response_type` DISABLE KEYS */;
INSERT INTO `auto_response_type` VALUES
(1,'auto reply','Automatic reply which will be sent out after a new ticket has been created.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(2,'auto reject','Automatic reject which will be sent out after a follow-up has been rejected.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(3,'auto follow up','Automatic confirmation which is sent out after a follow-up has been received.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(4,'auto reply/new ticket','Automatic response which will be sent out after a follow-up has been rejected and a new ticket has been created.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(5,'auto remove','Auto remove will be sent out after a customer removed the request.',1,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `auto_response_type` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `auto_response`
--

DROP TABLE IF EXISTS `auto_response`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `auto_response` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `text0` text DEFAULT NULL,
  `text1` text DEFAULT NULL,
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
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `auto_response`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `auto_response` WRITE;
/*!40000 ALTER TABLE `auto_response` DISABLE KEYS */;
INSERT INTO `auto_response` VALUES
(1,'Support auto reply','Thanks for your mail, we are on it.','RE: <OTRS_TICKET_Title>',1,2,'text/plain','',1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `auto_response` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `queue_auto_response`
--

DROP TABLE IF EXISTS `queue_auto_response`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `queue_auto_response`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `queue_auto_response` WRITE;
/*!40000 ALTER TABLE `queue_auto_response` DISABLE KEYS */;
INSERT INTO `queue_auto_response` VALUES
(1,5,1,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `queue_auto_response` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `notification_event`
--

DROP TABLE IF EXISTS `notification_event`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `notification_event`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `notification_event` WRITE;
/*!40000 ALTER TABLE `notification_event` DISABLE KEYS */;
INSERT INTO `notification_event` VALUES
(1,'Ticket create notification',1,'','2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `notification_event` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `notification_event_item`
--

DROP TABLE IF EXISTS `notification_event_item`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `notification_event_item` (
  `notification_id` int(11) NOT NULL,
  `event_key` varchar(200) NOT NULL,
  `event_value` varchar(200) NOT NULL,
  KEY `notification_event_item_event_key` (`event_key`),
  KEY `notification_event_item_event_value` (`event_value`),
  KEY `notification_event_item_notification_id` (`notification_id`),
  CONSTRAINT `FK_notification_event_item_notification_id_id` FOREIGN KEY (`notification_id`) REFERENCES `notification_event` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `notification_event_item`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `notification_event_item` WRITE;
/*!40000 ALTER TABLE `notification_event_item` DISABLE KEYS */;
INSERT INTO `notification_event_item` VALUES
(1,'Events','NotificationNewTicket'),
(1,'Recipients','AgentMyQueues');
/*!40000 ALTER TABLE `notification_event_item` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `notification_event_message`
--

DROP TABLE IF EXISTS `notification_event_message`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=112 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `notification_event_message`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `notification_event_message` WRITE;
/*!40000 ALTER TABLE `notification_event_message` DISABLE KEYS */;
INSERT INTO `notification_event_message` VALUES
(1,1,'Ticket Created: <OTRS_TICKET_Title>','A new ticket was created.','text/plain','en'),
(2,1,'Ticket erstellt: <OTRS_TICKET_Title>','Ein neues Ticket wurde erstellt.','text/plain','de');
/*!40000 ALTER TABLE `notification_event_message` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `generic_agent_jobs`
--

DROP TABLE IF EXISTS `generic_agent_jobs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `generic_agent_jobs` (
  `job_name` varchar(200) NOT NULL,
  `job_key` varchar(200) NOT NULL,
  `job_value` varchar(200) DEFAULT NULL,
  KEY `generic_agent_jobs_job_name` (`job_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `generic_agent_jobs`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `generic_agent_jobs` WRITE;
/*!40000 ALTER TABLE `generic_agent_jobs` DISABLE KEYS */;
INSERT INTO `generic_agent_jobs` VALUES
('close stale pending','Valid','1'),
('close stale pending','NewStateID','2'),
('close stale pending','ScheduleDays','1');
/*!40000 ALTER TABLE `generic_agent_jobs` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `acl`
--

DROP TABLE IF EXISTS `acl`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(200) NOT NULL,
  `comments` varchar(250) DEFAULT NULL,
  `description` varchar(250) DEFAULT NULL,
  `valid_id` smallint(6) NOT NULL,
  `stop_after_match` smallint(6) DEFAULT NULL,
  `config_match` longblob DEFAULT NULL,
  `config_change` longblob DEFAULT NULL,
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `acl` WRITE;
/*!40000 ALTER TABLE `acl` DISABLE KEYS */;
INSERT INTO `acl` VALUES
(1,'billing-no-close','','',1,0,'---\nProperties:\n  Queue:\n    Name:\n    - Billing\n','---\nPossibleNot:\n  Action:\n  - AgentTicketClose\n','2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `acl` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `service`
--

DROP TABLE IF EXISTS `service`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `service` WRITE;
/*!40000 ALTER TABLE `service` DISABLE KEYS */;
INSERT INTO `service` VALUES
(1,'Printing',1,'','2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `service` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `sla`
--

DROP TABLE IF EXISTS `sla`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `sla`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `sla` WRITE;
/*!40000 ALTER TABLE `sla` DISABLE KEYS */;
INSERT INTO `sla` VALUES
(1,'Gold','1',60,80,0,0,480,80,1,'','2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `sla` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `service_sla`
--

DROP TABLE IF EXISTS `service_sla`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `service_sla` (
  `service_id` int(11) NOT NULL,
  `sla_id` int(11) NOT NULL,
  UNIQUE KEY `service_sla_service_sla` (`service_id`,`sla_id`),
  KEY `FK_service_sla_sla_id_id` (`sla_id`),
  CONSTRAINT `FK_service_sla_service_id_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`),
  CONSTRAINT `FK_service_sla_sla_id_id` FOREIGN KEY (`sla_id`) REFERENCES `sla` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service_sla`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `service_sla` WRITE;
/*!40000 ALTER TABLE `service_sla` DISABLE KEYS */;
INSERT INTO `service_sla` VALUES
(1,1);
/*!40000 ALTER TABLE `service_sla` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `service_customer_user`
--

DROP TABLE IF EXISTS `service_customer_user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service_customer_user`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `service_customer_user` WRITE;
/*!40000 ALTER TABLE `service_customer_user` DISABLE KEYS */;
INSERT INTO `service_customer_user` VALUES
('jane',1,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `service_customer_user` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `personal_queues`
--

DROP TABLE IF EXISTS `personal_queues`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `personal_queues` (
  `user_id` int(11) NOT NULL,
  `queue_id` int(11) NOT NULL,
  KEY `personal_queues_queue_id` (`queue_id`),
  KEY `personal_queues_user_id` (`user_id`),
  CONSTRAINT `FK_personal_queues_queue_id_id` FOREIGN KEY (`queue_id`) REFERENCES `queue` (`id`),
  CONSTRAINT `FK_personal_queues_user_id_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `personal_queues`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `personal_queues` WRITE;
/*!40000 ALTER TABLE `personal_queues` DISABLE KEYS */;
INSERT INTO `personal_queues` VALUES
(4,5);
/*!40000 ALTER TABLE `personal_queues` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `sessions`
--

DROP TABLE IF EXISTS `sessions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `sessions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `session_id` varchar(100) NOT NULL,
  `data_key` varchar(100) NOT NULL,
  `data_value` text DEFAULT NULL,
  `serialized` smallint(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `sessions_data_key` (`data_key`),
  KEY `sessions_session_id_data_key` (`session_id`,`data_key`)
) ENGINE=InnoDB AUTO_INCREMENT=699 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `sessions`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `sessions` WRITE;
/*!40000 ALTER TABLE `sessions` DISABLE KEYS */;
INSERT INTO `sessions` VALUES
(1,'a1b2c3','UserLogin','jdoe',0),
(2,'a1b2c3','UserID','2',0);
/*!40000 ALTER TABLE `sessions` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `sysconfig_default`
--

DROP TABLE IF EXISTS `sysconfig_default`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=1925 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `sysconfig_default`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `sysconfig_default` WRITE;
/*!40000 ALTER TABLE `sysconfig_default` DISABLE KEYS */;
INSERT INTO `sysconfig_default` VALUES
(601,'TimeWorkingHours','Defines the hours and week days to count the working time.','Core::Time',0,0,1,1,100,0,0,NULL,'<Setting Name=\"TimeWorkingHours\" Required=\"1\" Valid=\"1\"></Setting>','---\nName: TimeWorkingHours\n','Calendar.xml','---\nMon:\n- \'8\'\n- \'9\'\n',0,'0',NULL,NULL,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(602,'TimeWorkingHours::Calendar1','Defines the hours and week days of the indicated calendar, to count the working time.','Core::Time::Calendar1',0,0,1,1,100,0,0,NULL,'<Setting Name=\"TimeWorkingHours::Calendar1\" Required=\"1\" Valid=\"1\"></Setting>','---\nName: TimeWorkingHours::Calendar1\n','Calendar.xml','---\nMon:\n- \'8\'\n- \'9\'\n',0,'0',NULL,NULL,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1),
(603,'Ticket::Hook','The identifier for a ticket, e.g. Ticket#, Call#, MyTicket#.','Core::Ticket',0,0,1,1,100,0,0,NULL,'<Setting Name=\"Ticket::Hook\" Required=\"1\" Valid=\"1\"></Setting>','---\nName: Ticket::Hook\n','Ticket.xml','--- Ticket#\n',0,'0',NULL,NULL,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);
/*!40000 ALTER TABLE `sysconfig_default` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;

--
-- Table structure for table `sysconfig_modified`
--

DROP TABLE IF EXISTS `sysconfig_modified`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
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
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `sysconfig_modified`
--

SET @OLD_AUTOCOMMIT=@@AUTOCOMMIT, @@AUTOCOMMIT=0;
LOCK TABLES `sysconfig_modified` WRITE;
/*!40000 ALTER TABLE `sysconfig_modified` DISABLE KEYS */;
INSERT INTO `sysconfig_modified` VALUES
(11,601,'TimeWorkingHours',NULL,1,0,'---\nMon:\n- \'9\'\n- \'10\'\n- \'11\'\nTue: []\n',0,0,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2),
(12,602,'TimeWorkingHours::Calendar1',NULL,1,0,'---\nSat:\n- \'10\'\n- \'11\'\n',0,0,'2014-02-03 10:00:00',2,'2014-02-03 10:00:00',2);
/*!40000 ALTER TABLE `sysconfig_modified` ENABLE KEYS */;
UNLOCK TABLES;
COMMIT;
SET AUTOCOMMIT=@OLD_AUTOCOMMIT;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*M!100616 SET NOTE_VERBOSITY=@OLD_NOTE_VERBOSITY */;

-- Dump completed
