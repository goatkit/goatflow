-- Test Data Migration Placeholder
-- The actual test data is in 000004_generated_test_data.up.sql
-- Run 'make synthesize' or 'gotrs synthesize' to generate the test data first
-- This file exists to maintain migration order

DO $$
BEGIN
    -- Check if we're in production
    IF current_setting('app.env', true) = 'production' THEN
        RAISE EXCEPTION 'Test data migration cannot be run in production';
    END IF;
    
    -- This is just a placeholder
    -- The actual test data is loaded via migration 000004
    RAISE NOTICE 'Test data placeholder migration - actual data in 000004_generated_test_data.up.sql';
END $$;