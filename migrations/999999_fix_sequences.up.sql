-- Fix sequences after any data import
-- This migration ensures all sequences are properly synchronized with their table's max ID

-- Function to fix all sequences in the database
CREATE OR REPLACE FUNCTION fix_all_sequences() RETURNS void AS $$
DECLARE
    r RECORD;
    seq_value BIGINT;
    max_value BIGINT;
BEGIN
    -- Find all sequences and their associated tables
    FOR r IN 
        SELECT 
            sequencename AS sequence_name,
            tablename AS table_name,
            attname AS column_name
        FROM pg_sequences 
        JOIN pg_class c ON c.relname = sequencename
        JOIN pg_depend d ON d.objid = c.oid
        JOIN pg_attribute a ON a.attrelid = d.refobjid AND a.attnum = d.refobjsubid
        JOIN pg_tables t ON t.tablename::regclass::oid = d.refobjid
        WHERE schemaname = 'public'
            AND sequencename LIKE '%_seq'
    LOOP
        -- Get the maximum value from the table
        EXECUTE format('SELECT COALESCE(MAX(%I), 0) FROM %I', r.column_name, r.table_name) INTO max_value;
        
        -- Get current sequence value
        EXECUTE format('SELECT last_value FROM %I', r.sequence_name) INTO seq_value;
        
        -- If sequence is behind, update it
        IF max_value >= seq_value THEN
            EXECUTE format('SELECT setval(%L, %s, true)', r.sequence_name, max_value);
            RAISE NOTICE 'Fixed sequence % for table % (was %, now %)', 
                r.sequence_name, r.table_name, seq_value, max_value;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Run the fix
SELECT fix_all_sequences();

-- Add trigger to auto-fix sequences on large inserts (optional, can be removed if performance is a concern)
CREATE OR REPLACE FUNCTION ensure_sequence_sync() RETURNS trigger AS $$
DECLARE
    seq_name TEXT;
    max_id BIGINT;
    seq_value BIGINT;
BEGIN
    -- Only check on INSERT operations
    IF TG_OP = 'INSERT' THEN
        -- Get the sequence name for this table's id column
        SELECT pg_get_serial_sequence(TG_TABLE_NAME::text, 'id') INTO seq_name;
        
        IF seq_name IS NOT NULL THEN
            -- Get max ID from table
            EXECUTE format('SELECT MAX(id) FROM %I', TG_TABLE_NAME) INTO max_id;
            
            -- Get current sequence value
            EXECUTE format('SELECT last_value FROM %s', seq_name) INTO seq_value;
            
            -- If we detect a problem, fix it
            IF max_id > seq_value THEN
                EXECUTE format('SELECT setval(%L, %s, true)', seq_name, max_id);
                RAISE WARNING 'Auto-fixed sequence % for table % (was %, now %)', 
                    seq_name, TG_TABLE_NAME, seq_value, max_id;
            END IF;
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Note: We don't automatically add the trigger to all tables as it could impact performance
-- Instead, we'll just ensure sequences are correct at migration time
-- If you want to add the trigger to specific problem tables, uncomment below:

-- CREATE TRIGGER ensure_article_sequence_sync 
--     AFTER INSERT ON article 
--     FOR EACH STATEMENT 
--     EXECUTE FUNCTION ensure_sequence_sync();

-- CREATE TRIGGER ensure_ticket_sequence_sync 
--     AFTER INSERT ON ticket 
--     FOR EACH STATEMENT 
--     EXECUTE FUNCTION ensure_sequence_sync();