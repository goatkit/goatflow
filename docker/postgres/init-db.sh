#!/bin/bash
set -e

# This script ensures the GoatFlow database is properly initialized
# It runs when the PostgreSQL container starts for the first time

echo "Initializing GoatFlow database..."

# Create the database if it doesn't exist
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
    -- Create database if not exists
    SELECT 'CREATE DATABASE ${POSTGRES_DB}'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${POSTGRES_DB}')\gexec
    
    -- Grant all privileges to the user
    GRANT ALL PRIVILEGES ON DATABASE ${POSTGRES_DB} TO ${POSTGRES_USER};
EOSQL

echo "Database initialization complete!"