-- Test Data for Development
-- Only run in development/test environments
-- NOTE: Actual test data with passwords should be generated via 'make synthesize'

DO $$
BEGIN
    IF current_setting('app.env', true) = 'production' THEN
        RAISE EXCEPTION 'Test data migration cannot be run in production';
    END IF;
END $$;

-- Placeholder for test data
-- Run 'make synthesize' to generate actual test data with secure passwords
-- The generated file (000004_generated_test_data.up.sql) will contain:
-- - Test customer companies
-- - Test customer users with bcrypt hashed passwords  
-- - Test agents with bcrypt hashed passwords
-- - Test tickets and articles
