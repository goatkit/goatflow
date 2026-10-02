-- Position of the ticket notification evaluator (runner task
-- "notification-events") in the ticket, article and ticket_history tables.
-- Moving a cursor and queueing the resulting mail_queue rows happen in one
-- transaction, so every event is evaluated exactly once.
CREATE TABLE IF NOT EXISTS gk_notification_event_cursor (
    source      VARCHAR(50) PRIMARY KEY,
    last_id     BIGINT NOT NULL,
    change_time TIMESTAMP NOT NULL
);
