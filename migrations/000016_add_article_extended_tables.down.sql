-- Rollback: Remove extended article and ticket tables
DROP TABLE IF EXISTS time_accounting CASCADE;
DROP TABLE IF EXISTS ticket_watcher CASCADE;
DROP TABLE IF EXISTS ticket_number_counter CASCADE;
DROP TABLE IF EXISTS ticket_loop_protection CASCADE;
DROP TABLE IF EXISTS ticket_lock_index CASCADE;
DROP TABLE IF EXISTS ticket_index CASCADE;
DROP TABLE IF EXISTS ticket_flag CASCADE;
DROP TABLE IF EXISTS article_search_index CASCADE;
DROP TABLE IF EXISTS article_flag CASCADE;
DROP TABLE IF EXISTS article_data_otrs_chat CASCADE;
DROP TABLE IF EXISTS article_data_mime_send_error CASCADE;
DROP TABLE IF EXISTS article_data_mime_plain CASCADE;