-- Scheduler job lock: one row per scheduled job. Every backend replica runs
-- the same cron schedule; before a run a replica claims the job's row with
-- UPDATE ... WHERE locked_until <= now, so only one replica runs each tick.
-- locked_until is a UTC time written by the claiming replica.
CREATE TABLE IF NOT EXISTS gk_scheduler_job_lock (
    job_slug     VARCHAR(200) NOT NULL PRIMARY KEY,
    locked_until TIMESTAMP NOT NULL,
    locked_by    VARCHAR(255) NOT NULL,
    change_time  TIMESTAMP NOT NULL
);
