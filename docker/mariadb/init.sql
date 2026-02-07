-- MariaDB initialization script for GoatFlow
-- Creates database and user if they don't exist
-- NOTE: User password is set via MYSQL_PASSWORD env var in docker-compose.yml
-- This script only grants privileges; MariaDB image creates user automatically

-- Create database if it doesn't exist
CREATE DATABASE IF NOT EXISTS otrs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Grant all privileges on the database to the user created by MYSQL_USER env var
-- The user is automatically created by MariaDB image from MYSQL_USER/MYSQL_PASSWORD
-- Note: '%' wildcard covers all hosts including localhost
GRANT ALL PRIVILEGES ON otrs.* TO 'otrs'@'%';

-- Flush privileges to ensure they take effect
FLUSH PRIVILEGES;

-- Optional: Set some recommended settings for OTRS compatibility
SET GLOBAL max_allowed_packet = 67108864; -- 64MB
SET GLOBAL innodb_log_file_size = 268435456; -- 256MB
SET GLOBAL query_cache_size = 33554432; -- 32MB