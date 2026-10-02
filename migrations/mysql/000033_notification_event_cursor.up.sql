-- Position of the ticket notification evaluator (runner task
-- "notification-events") in the ticket, article and ticket_history tables.
-- Moving a cursor and queueing the resulting mail_queue rows happen in one
-- transaction, so every event is evaluated exactly once.
CREATE TABLE IF NOT EXISTS gk_notification_event_cursor (
    source      VARCHAR(50) NOT NULL,
    last_id     BIGINT NOT NULL,
    change_time DATETIME NOT NULL,

    PRIMARY KEY (source)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
