-- Replace ticket table with exact OTRS schema
-- This migration completely recreates the ticket table to match OTRS exactly

-- Drop existing ticket table and recreate with exact OTRS schema
DROP TABLE IF EXISTS ticket CASCADE;

CREATE TABLE ticket (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  tn varchar(50) NOT NULL,
  title varchar(255) DEFAULT NULL,
  queue_id int NOT NULL,
  ticket_lock_id smallint NOT NULL,
  type_id smallint DEFAULT NULL,
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
  CONSTRAINT ticket_tn_key UNIQUE (tn),
  CONSTRAINT ticket_queue_id_fkey FOREIGN KEY (queue_id) REFERENCES queue(id),
  CONSTRAINT ticket_ticket_priority_id_fkey FOREIGN KEY (ticket_priority_id) REFERENCES ticket_priority(id),
  CONSTRAINT ticket_ticket_state_id_fkey FOREIGN KEY (ticket_state_id) REFERENCES ticket_state(id),
  CONSTRAINT ticket_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT ticket_responsible_user_id_fkey FOREIGN KEY (responsible_user_id) REFERENCES users(id)
);

-- Create indexes matching OTRS
CREATE INDEX ticket_archive_flag ON ticket(archive_flag);
CREATE INDEX ticket_create_time ON ticket(create_time);
CREATE INDEX ticket_customer_id ON ticket(customer_id);
CREATE INDEX ticket_customer_user_id ON ticket(customer_user_id);
CREATE INDEX ticket_escalation_response_time ON ticket(escalation_response_time);
CREATE INDEX ticket_escalation_solution_time ON ticket(escalation_solution_time);
CREATE INDEX ticket_escalation_time ON ticket(escalation_time);
CREATE INDEX ticket_escalation_update_time ON ticket(escalation_update_time);
CREATE INDEX ticket_queue_id ON ticket(queue_id);
CREATE INDEX ticket_queue_view ON ticket(ticket_state_id, ticket_lock_id);
CREATE INDEX ticket_responsible_user_id ON ticket(responsible_user_id);
CREATE INDEX ticket_ticket_lock_id ON ticket(ticket_lock_id);
CREATE INDEX ticket_ticket_priority_id ON ticket(ticket_priority_id);
CREATE INDEX ticket_ticket_state_id ON ticket(ticket_state_id);
CREATE INDEX ticket_timeout ON ticket(timeout);
CREATE INDEX ticket_title ON ticket(title);
CREATE INDEX ticket_type_id ON ticket(type_id);
CREATE INDEX ticket_until_time ON ticket(until_time);
CREATE INDEX ticket_user_id ON ticket(user_id);
CREATE INDEX ticket_tn ON ticket(tn);

-- Recreate dependent tables

-- Drop and recreate article table
DROP TABLE IF EXISTS article CASCADE;
CREATE TABLE article (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  ticket_id bigint NOT NULL,
  article_type_id smallint NOT NULL,
  article_sender_type_id smallint NOT NULL,
  article_is_visible_for_customer smallint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL,
  create_by int NOT NULL,
  change_time timestamp NOT NULL,
  change_by int NOT NULL,
  
  CONSTRAINT article_pkey PRIMARY KEY (id),
  CONSTRAINT article_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES ticket(id)
);

CREATE INDEX article_ticket_id ON article(ticket_id);
CREATE INDEX article_create_time ON article(create_time);

-- Drop and recreate ticket_history table  
DROP TABLE IF EXISTS ticket_history CASCADE;
CREATE TABLE ticket_history (
  id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  name varchar(200) NOT NULL,
  history_type_id smallint NOT NULL,
  ticket_id bigint NOT NULL,
  article_id bigint DEFAULT NULL,
  type_id smallint NOT NULL,
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

CREATE INDEX ticket_history_article_id ON ticket_history(article_id);
CREATE INDEX ticket_history_create_time ON ticket_history(create_time);
CREATE INDEX ticket_history_history_type_id ON ticket_history(history_type_id);
CREATE INDEX ticket_history_owner_id ON ticket_history(owner_id);
CREATE INDEX ticket_history_priority_id ON ticket_history(priority_id);
CREATE INDEX ticket_history_queue_id ON ticket_history(queue_id);
CREATE INDEX ticket_history_state_id ON ticket_history(state_id);
CREATE INDEX ticket_history_ticket_id ON ticket_history(ticket_id);
CREATE INDEX ticket_history_type_id ON ticket_history(type_id);