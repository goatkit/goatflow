-- MariaDB initialization script for GoatFlow
-- The goatflow database and user are created automatically by the MariaDB image
-- via MYSQL_DATABASE / MYSQL_USER / MYSQL_PASSWORD env vars in docker-compose.yml.
-- Nothing extra needed here.

-- Create database if it doesn't exist (for local development)
CREATE DATABASE IF NOT EXISTS otrs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

FLUSH PRIVILEGES;
