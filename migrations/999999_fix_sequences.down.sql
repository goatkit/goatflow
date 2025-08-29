-- Remove sequence fix functions
DROP FUNCTION IF EXISTS fix_all_sequences();
DROP FUNCTION IF EXISTS ensure_sequence_sync() CASCADE;