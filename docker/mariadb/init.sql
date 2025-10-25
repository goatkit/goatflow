-- MariaDB initialization script for GOTRS
-- Creates database and user if they don't exist

-- Create database if it doesn't exist
CREATE DATABASE IF NOT EXISTS otrs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Create user if it doesn't exist (compatible with both MySQL and MariaDB)
-- Using IF NOT EXISTS syntax for MariaDB 10.1.3+
CREATE USER IF NOT EXISTS 'otrs'@'%' IDENTIFIED BY 'CHANGEME';

-- Grant all privileges on the database
GRANT ALL PRIVILEGES ON otrs.* TO 'otrs'@'%';

-- Also create from localhost for compatibility
CREATE USER IF NOT EXISTS 'otrs'@'localhost' IDENTIFIED BY 'CHANGEME';
GRANT ALL PRIVILEGES ON otrs.* TO 'otrs'@'localhost';

-- Flush privileges to ensure they take effect
FLUSH PRIVILEGES;

-- Optional: Set some recommended settings for OTRS compatibility
SET GLOBAL max_allowed_packet = 67108864; -- 64MB
SET GLOBAL innodb_log_file_size = 268435456; -- 256MB
SET GLOBAL query_cache_size = 33554432; -- 32MB