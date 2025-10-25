DO $$
DECLARE
    r RECORD;
    seq_value BIGINT;
    max_value BIGINT;
BEGIN
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
    LOOP
        EXECUTE format('SELECT COALESCE(MAX(%I), 0) FROM %I', r.column_name, r.table_name) INTO max_value;
        EXECUTE format('SELECT last_value FROM %I', r.sequence_name) INTO seq_value;
        IF max_value >= seq_value THEN
            EXECUTE format('SELECT setval(%L, %s, true)', r.sequence_name, max_value);
        END IF;
    END LOOP;
END;
$$;

