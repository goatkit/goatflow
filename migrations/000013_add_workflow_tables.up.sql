-- Batch 4: Process Management and Workflow tables
-- Based on OTRS v6 schema but recreated to avoid direct copying

-- Generic agent jobs
CREATE TABLE IF NOT EXISTS generic_agent_jobs (
    job_name VARCHAR(200) PRIMARY KEY,
    job_data TEXT NOT NULL,
    last_run_time TIMESTAMP WITHOUT TIME ZONE,
    next_run_time TIMESTAMP WITHOUT TIME ZONE,
    run_counter BIGINT DEFAULT 0,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_generic_agent_jobs_next_run_time ON generic_agent_jobs(next_run_time);

-- Process management processes
CREATE TABLE IF NOT EXISTS pm_process (
    id SERIAL PRIMARY KEY,
    entity_id VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    state_entity_id VARCHAR(50) NOT NULL,
    layout TEXT NOT NULL,
    config TEXT NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_pm_process_entity_id ON pm_process(entity_id);

-- Process activities
CREATE TABLE IF NOT EXISTS pm_activity (
    id SERIAL PRIMARY KEY,
    entity_id VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    config TEXT NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_pm_activity_entity_id ON pm_activity(entity_id);

-- Activity dialogs
CREATE TABLE IF NOT EXISTS pm_activity_dialog (
    id SERIAL PRIMARY KEY,
    entity_id VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    config TEXT NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_pm_activity_dialog_entity_id ON pm_activity_dialog(entity_id);

-- Process transitions
CREATE TABLE IF NOT EXISTS pm_transition (
    id SERIAL PRIMARY KEY,
    entity_id VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    config TEXT NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_pm_transition_entity_id ON pm_transition(entity_id);

-- Transition actions
CREATE TABLE IF NOT EXISTS pm_transition_action (
    id SERIAL PRIMARY KEY,
    entity_id VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    config TEXT NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_by INTEGER NOT NULL
);

CREATE INDEX idx_pm_transition_action_entity_id ON pm_transition_action(entity_id);

-- Process entity sync state
CREATE TABLE IF NOT EXISTS pm_entity_sync (
    entity_type VARCHAR(30) NOT NULL,
    entity_id VARCHAR(50) NOT NULL,
    sync_state VARCHAR(30) NOT NULL,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (entity_type, entity_id)
);

-- Process ID counter
CREATE TABLE IF NOT EXISTS process_id (
    process_name VARCHAR(200) NOT NULL,
    process_id VARCHAR(200) NOT NULL,
    process_host VARCHAR(200) NOT NULL,
    process_create INTEGER NOT NULL,
    process_change INTEGER NOT NULL
);

CREATE INDEX idx_process_id_process_name ON process_id(process_name);
CREATE INDEX idx_process_id_process_id ON process_id(process_id);
CREATE INDEX idx_process_id_process_host ON process_id(process_host);
CREATE INDEX idx_process_id_process_change ON process_id(process_change);

-- Scheduler tasks
CREATE TABLE IF NOT EXISTS scheduler_task (
    id BIGSERIAL PRIMARY KEY,
    ident BIGINT NOT NULL,
    name VARCHAR(255),
    task_type VARCHAR(255) NOT NULL,
    task_data TEXT NOT NULL,
    attempts SMALLINT NOT NULL DEFAULT 1,
    lock_key BIGINT NOT NULL DEFAULT 0,
    lock_time TIMESTAMP WITHOUT TIME ZONE,
    lock_update_time TIMESTAMP WITHOUT TIME ZONE,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_scheduler_task_ident ON scheduler_task(ident);
CREATE INDEX idx_scheduler_task_lock_key ON scheduler_task(lock_key);

-- Scheduler future tasks
CREATE TABLE IF NOT EXISTS scheduler_future_task (
    id BIGSERIAL PRIMARY KEY,
    ident BIGINT NOT NULL,
    execution_time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    name VARCHAR(255),
    task_type VARCHAR(255) NOT NULL,
    task_data TEXT NOT NULL,
    attempts SMALLINT NOT NULL DEFAULT 1,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_scheduler_future_task_ident ON scheduler_future_task(ident);
CREATE INDEX idx_scheduler_future_task_execution_time ON scheduler_future_task(execution_time);

-- Scheduler recurrent tasks
CREATE TABLE IF NOT EXISTS scheduler_recurrent_task (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    task_type VARCHAR(255) NOT NULL,
    last_execution_time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    last_worker_task_id BIGINT,
    last_worker_status SMALLINT,
    last_worker_running_time INTEGER,
    lock_key BIGINT NOT NULL DEFAULT 0,
    lock_time TIMESTAMP WITHOUT TIME ZONE,
    create_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_scheduler_recurrent_task_name ON scheduler_recurrent_task(name);
CREATE INDEX idx_scheduler_recurrent_task_task_type ON scheduler_recurrent_task(task_type);
CREATE INDEX idx_scheduler_recurrent_task_lock_key ON scheduler_recurrent_task(lock_key);