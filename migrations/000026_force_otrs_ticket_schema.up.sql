-- Force exact OTRS ticket schema matching
-- Drop all constraints first, then recreate table completely

-- Remove foreign key constraints
ALTER TABLE IF EXISTS article DROP CONSTRAINT IF EXISTS article_ticket_id_fkey;
ALTER TABLE IF EXISTS ticket_history DROP CONSTRAINT IF EXISTS ticket_history_ticket_id_fkey;

-- Drop and recreate ticket table with EXACT OTRS column names
DROP TABLE IF EXISTS ticket CASCADE;

CREATE TABLE ticket (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  tn varchar(50) NOT NULL,
  title varchar(255) DEFAULT NULL,
  queue_id int NOT NULL,
  ticket_lock_id smallint NOT NULL,
  type_id smallint DEFAULT NULL,  -- OTRS uses type_id, not ticket_type_id
  service_id int DEFAULT NULL,
  sla_id int DEFAULT NULL,
  user_id int NOT NULL,
  responsible_user_id int NOT NULL,
  ticket_priority_id smallint NOT NULL,
  ticket_state_id smallint NOT NULL,
  customer_id varchar(150) DEFAULT NULL,
  customer_user_id varchar(250) DEFAULT NULL,
  timeout int NOT NULL,
  until_time int NOT NULL,
  escalation_time int NOT NULL,
  escalation_update_time int NOT NULL,
  escalation_response_time int NOT NULL,
  escalation_solution_time int NOT NULL,
  archive_flag smallint NOT NULL DEFAULT 0,
  create_time timestamp NOT NULL,
  create_by int NOT NULL,
  change_time timestamp NOT NULL,
  change_by int NOT NULL,
  
  CONSTRAINT ticket_pkey PRIMARY KEY (id),
  CONSTRAINT ticket_tn_key UNIQUE (tn)
);

-- Create essential indexes
CREATE INDEX ticket_create_time ON ticket(create_time);
CREATE INDEX ticket_customer_id ON ticket(customer_id);
CREATE INDEX ticket_queue_id ON ticket(queue_id);
CREATE INDEX ticket_tn ON ticket(tn);

-- Recreate article table pointing to new ticket
CREATE TABLE article (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  ticket_id bigint NOT NULL,
  article_sender_type_id smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by int NOT NULL DEFAULT 1,
  change_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  change_by int NOT NULL DEFAULT 1,
  
  CONSTRAINT article_pkey PRIMARY KEY (id),
  CONSTRAINT article_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES ticket(id)
);

-- Recreate ticket_history table
CREATE TABLE ticket_history (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  history_type_id smallint NOT NULL DEFAULT 1,
  ticket_id bigint NOT NULL,
  article_id bigint DEFAULT NULL,
  type_id smallint NOT NULL DEFAULT 1,
  queue_id int NOT NULL,
  owner_id int NOT NULL,
  priority_id smallint NOT NULL,
  state_id smallint NOT NULL,
  create_time timestamp NOT NULL,
  create_by int NOT NULL,
  change_time timestamp NOT NULL,
  change_by int NOT NULL,
  
  CONSTRAINT ticket_history_pkey PRIMARY KEY (id),
  CONSTRAINT ticket_history_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES ticket(id),
  CONSTRAINT ticket_history_article_id_fkey FOREIGN KEY (article_id) REFERENCES article(id)
);