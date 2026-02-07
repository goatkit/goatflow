-- GoatFlow PostgreSQL Initialization Script
-- This script creates all necessary databases and prevents connection errors
-- It runs ONLY on first container initialization (not on restarts)

-- SAFETY: This script only runs in Docker/Podman containers during initialization
-- It will NOT affect existing databases or production systems

\echo 'Starting GoatFlow database initialization...'

-- Create a database matching the username to prevent "database does not exist" errors
-- This handles any DB_USER value (goatflow, goatflow_user, custom_user, etc.)
SELECT 'CREATE DATABASE ' || current_user
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = current_user)\gexec

-- Create the main application database (if different from username)
-- Uses the DB_NAME environment variable (defaults to 'goatflow')
\set db_name `echo "${POSTGRES_DB:-goatflow}"`
SELECT 'CREATE DATABASE ' || :'db_name'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'db_name')\gexec

-- Create Temporal workflow engine databases
\echo 'Creating Temporal databases...'
SELECT 'CREATE DATABASE temporal'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'temporal')\gexec

SELECT 'CREATE DATABASE temporal_visibility'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'temporal_visibility')\gexec

-- Create test database ONLY in development/test environments
-- NEVER create test database in production
\set app_env `echo "${APP_ENV:-development}"`
\if :app_env != 'production'
    \echo 'Creating test database (non-production environment detected)...'
    
    -- Create test database with '_test' suffix
    \set test_db_name `echo "${POSTGRES_DB:-goatflow}_test"`
    SELECT 'CREATE DATABASE ' || :'test_db_name'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'test_db_name')\gexec
    
    \echo 'Test database created successfully.'
\else
    \echo 'PRODUCTION ENVIRONMENT - Skipping test database creation for safety.'
\endif

-- Grant privileges to the user for all databases
DO $$
DECLARE
    db_record RECORD;
BEGIN
    -- Grant privileges on main database
    EXECUTE format('GRANT ALL PRIVILEGES ON DATABASE %I TO %I', 
                   '${POSTGRES_DB:-goatflow}', current_user);
    
    -- Grant privileges on user-named database
    EXECUTE format('GRANT ALL PRIVILEGES ON DATABASE %I TO %I', 
                   current_user, current_user);
    
    -- Grant privileges on Temporal databases
    EXECUTE format('GRANT ALL PRIVILEGES ON DATABASE temporal TO %I', current_user);
    EXECUTE format('GRANT ALL PRIVILEGES ON DATABASE temporal_visibility TO %I', current_user);
    
    -- Grant privileges on test database (if not production)
    IF '${APP_ENV:-development}' != 'production' THEN
        EXECUTE format('GRANT ALL PRIVILEGES ON DATABASE %I TO %I', 
                       '${POSTGRES_DB:-goatflow}' || '_test', current_user);
    END IF;
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Some grants may have failed (databases might not exist): %', SQLERRM;
END$$;

\echo 'Database initialization complete!'
\echo 'Created databases:'
SELECT datname FROM pg_database 
WHERE datname NOT IN ('postgres', 'template0', 'template1')
ORDER BY datname;