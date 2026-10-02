-- GoatFlow importer fixture: the rows of otrs6_dump.sql in an OTRS 6 / Znuny 6 PostgreSQL
-- database. Column types follow OTRS's PostgreSQL schema (otrs-schema.postgresql.sql: LONGBLOB
-- is TEXT, DATE is timestamp(0)), and binary article/attachment content is base64 text the way
-- OTRS writes it on databases without DirectBlob support (MIME::Base64, 76-character lines).

CREATE TABLE acl (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    description VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    stop_after_match SMALLINT,
    config_match TEXT,
    config_change TEXT,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO acl (id, name, comments, description, valid_id, stop_after_match, config_match, config_change, create_time, create_by, change_time, change_by) VALUES (1, 'billing-no-close', '', '', 1, 0, '---
Properties:
  Queue:
    Name:
    - Billing
', '---
PossibleNot:
  Action:
  - AgentTicketClose
', '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE article (
    id bigserial,
    ticket_id BIGINT NOT NULL,
    article_sender_type_id SMALLINT NOT NULL,
    communication_channel_id BIGINT NOT NULL,
    is_visible_for_customer SMALLINT NOT NULL,
    search_index_needs_rebuild SMALLINT NOT NULL,
    insert_fingerprint VARCHAR(64),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO article (id, ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, search_index_needs_rebuild, insert_fingerprint, create_time, create_by, change_time, change_by) VALUES (12, 9, 1, 1, 1, 0, NULL, '2014-03-06 10:30:00', 3, '2014-03-06 10:30:00', 3);
INSERT INTO article (id, ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, search_index_needs_rebuild, insert_fingerprint, create_time, create_by, change_time, change_by) VALUES (7, 3, 3, 1, 1, 0, NULL, '2014-03-06 00:10:00', 1, '2014-03-06 00:10:00', 1);

CREATE TABLE article_data_mime (
    id bigserial,
    article_id BIGINT NOT NULL,
    a_from TEXT,
    a_reply_to TEXT,
    a_to TEXT,
    a_cc TEXT,
    a_bcc TEXT,
    a_subject TEXT,
    a_message_id TEXT,
    a_message_id_md5 VARCHAR(32),
    a_in_reply_to TEXT,
    a_references TEXT,
    a_content_type VARCHAR(250),
    a_body TEXT,
    incoming_time INTEGER NOT NULL,
    content_path VARCHAR(250),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO article_data_mime (id, article_id, a_from, a_reply_to, a_to, a_cc, a_bcc, a_subject, a_message_id, a_message_id_md5, a_in_reply_to, a_references, a_content_type, a_body, incoming_time, content_path, create_time, create_by, change_time, change_by) VALUES (20, 7, '"Jane Customer" <jane@acme.example>', NULL, 'support@example.com', NULL, NULL, 'Printer on fire', '<20140305.abc@acme.example>', 'a4b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8', NULL, NULL, 'text/plain; charset=utf-8', 'Hello,

the printer''s on fire \ again.
It says "PC LOAD LETTER" — ünïcödé ✓	and a  sub.', 1394064600, '2014/03/05', '2014-03-06 00:10:00', 1, '2014-03-06 00:10:00', 1);
INSERT INTO article_data_mime (id, article_id, a_from, a_reply_to, a_to, a_cc, a_bcc, a_subject, a_message_id, a_message_id_md5, a_in_reply_to, a_references, a_content_type, a_body, incoming_time, content_path, create_time, create_by, change_time, change_by) VALUES (21, 12, '"Alice Smith" <support@example.com>', NULL, '"Bob O''Brien" <bob@acme.example>', NULL, NULL, 'RE: Invoice question', '<20140306.def@example.com>', '0123456789abcdef0123456789abcdef', NULL, NULL, 'text/plain; charset=utf-8', 'Dear Bob,

please find the invoice attached.

-- 
Alice Smith
Example Support', 1394101800, '2014/03/06', '2014-03-06 10:30:00', 3, '2014-03-06 10:30:00', 3);

CREATE TABLE article_data_mime_attachment (
    id bigserial,
    article_id BIGINT NOT NULL,
    filename VARCHAR(250),
    content_size VARCHAR(30),
    content_type TEXT,
    content_id VARCHAR(250),
    content_alternative VARCHAR(50),
    disposition VARCHAR(15),
    content TEXT,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO article_data_mime_attachment (id, article_id, filename, content_size, content_type, content_id, content_alternative, disposition, content, create_time, create_by, change_time, change_by) VALUES (31, 12, 'invoice.pdf', '20', 'application/pdf', '', '', 'attachment', 'JVBERi0xLjQKACdc/w0aImVuZAo=
', '2014-03-06 10:30:00', 3, '2014-03-06 10:30:00', 3);
INSERT INTO article_data_mime_attachment (id, article_id, filename, content_size, content_type, content_id, content_alternative, disposition, content, create_time, create_by, change_time, change_by) VALUES (32, 12, 'logo.png', '21', 'image/png', '<logo.png@01D03F>', '', 'inline', 'iVBORw0KGgoAACcnXFz//gBJRU5E
', '2014-03-06 10:30:00', 3, '2014-03-06 10:30:00', 3);
INSERT INTO article_data_mime_attachment (id, article_id, filename, content_size, content_type, content_id, content_alternative, disposition, content, create_time, create_by, change_time, change_by) VALUES (33, 12, 'notes.txt', '33', 'text/plain; charset=utf-8', '', NULL, 'attachment', 'bm90ZXM6IGl0J3MgYSBcIGJhY2tzbGFzaArDpMO2w7wK
', '2014-03-06 10:30:00', 3, '2014-03-06 10:30:00', 3);

CREATE TABLE article_data_mime_plain (
    id bigserial,
    article_id BIGINT NOT NULL,
    body TEXT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO article_data_mime_plain (id, article_id, body, create_time, create_by, change_time, change_by) VALUES (40, 7, 'RnJvbTogIkphbmUgQ3VzdG9tZXIiIDxqYW5lQGFjbWUuZXhhbXBsZT4NClRvOiBzdXBwb3J0QGV4
YW1wbGUuY29tDQpTdWJqZWN0OiBQcmludGVyIG9uIGZpcmUNCk1lc3NhZ2UtSUQ6IDwyMDE0MDMw
NS5hYmNAYWNtZS5leGFtcGxlPg0KQ29udGVudC1UeXBlOiB0ZXh0L3BsYWluOyBjaGFyc2V0PXV0
Zi04DQoNCkhlbGxvLA0KDQp0aGUgcHJpbnRlcidzIG9uIGZpcmUgXCBhZ2Fpbi4NCgD/DQo=
', '2014-03-06 00:10:00', 1, '2014-03-06 00:10:00', 1);

CREATE TABLE article_flag (
    article_id BIGINT NOT NULL,
    article_key VARCHAR(50) NOT NULL,
    article_value VARCHAR(50),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL
);
INSERT INTO article_flag (article_id, article_key, article_value, create_time, create_by) VALUES (12, 'Seen', '1', '2014-03-06 10:30:00', 3);

CREATE TABLE article_sender_type (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO article_sender_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'agent', 'Sender type agent.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO article_sender_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'system', 'Sender type system.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO article_sender_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'customer', 'Sender type customer.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE auto_response (
    id serial,
    name VARCHAR(200) NOT NULL,
    text0 TEXT,
    text1 TEXT,
    type_id SMALLINT NOT NULL,
    system_address_id SMALLINT NOT NULL,
    content_type VARCHAR(250),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO auto_response (id, name, text0, text1, type_id, system_address_id, content_type, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Support auto reply', 'Thanks for your mail, we are on it.', 'RE: <OTRS_TICKET_Title>', 1, 2, 'text/plain', '', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE auto_response_type (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO auto_response_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'auto reply', 'Automatic reply which will be sent out after a new ticket has been created.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO auto_response_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'auto reject', 'Automatic reject which will be sent out after a follow-up has been rejected.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO auto_response_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'auto follow up', 'Automatic confirmation which is sent out after a follow-up has been received.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO auto_response_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (4, 'auto reply/new ticket', 'Automatic response which will be sent out after a follow-up has been rejected and a new ticket has been created.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO auto_response_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (5, 'auto remove', 'Auto remove will be sent out after a customer removed the request.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE communication_channel (
    id bigserial,
    name VARCHAR(200) NOT NULL,
    module VARCHAR(200) NOT NULL,
    package_name VARCHAR(200) NOT NULL,
    channel_data TEXT NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO communication_channel (id, name, module, package_name, channel_data, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Email', 'Kernel::System::CommunicationChannel::Email', 'Framework', '---
ArticleDataArticleIDField: article_id
ArticleDataTables:
- article_data_mime
- article_data_mime_plain
- article_data_mime_attachment
', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO communication_channel (id, name, module, package_name, channel_data, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'Phone', 'Kernel::System::CommunicationChannel::Phone', 'Framework', '---
ArticleDataArticleIDField: article_id
ArticleDataTables:
- article_data_mime
- article_data_mime_plain
- article_data_mime_attachment
', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO communication_channel (id, name, module, package_name, channel_data, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'Internal', 'Kernel::System::CommunicationChannel::Internal', 'Framework', '---
ArticleDataArticleIDField: article_id
ArticleDataTables:
- article_data_mime
- article_data_mime_plain
- article_data_mime_attachment
', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO communication_channel (id, name, module, package_name, channel_data, valid_id, create_time, create_by, change_time, change_by) VALUES (4, 'Chat', 'Kernel::System::CommunicationChannel::Chat', 'Framework', '---
ArticleDataArticleIDField: article_id
', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE customer_company (
    customer_id VARCHAR(150) NOT NULL,
    name VARCHAR(200) NOT NULL,
    street VARCHAR(200),
    zip VARCHAR(200),
    city VARCHAR(200),
    country VARCHAR(200),
    url VARCHAR(200),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(customer_id)
);
INSERT INTO customer_company (customer_id, name, street, zip, city, country, url, comments, valid_id, create_time, create_by, change_time, change_by) VALUES ('ACME', 'ACME Corp.', 'Main St 1', '12345', 'Springfield', 'US', 'https://acme.example', 'Key account', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE customer_preferences (
    user_id VARCHAR(250) NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value VARCHAR(250)
);
INSERT INTO customer_preferences (user_id, preferences_key, preferences_value) VALUES ('bob', 'UserShowTickets', '25');
INSERT INTO customer_preferences (user_id, preferences_key, preferences_value) VALUES ('jane', 'UserLanguage', 'fr');

CREATE TABLE customer_user (
    id serial,
    login VARCHAR(200) NOT NULL,
    email VARCHAR(150) NOT NULL,
    customer_id VARCHAR(150) NOT NULL,
    pw VARCHAR(128),
    title VARCHAR(50),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    phone VARCHAR(150),
    fax VARCHAR(150),
    mobile VARCHAR(150),
    street VARCHAR(150),
    zip VARCHAR(200),
    city VARCHAR(200),
    country VARCHAR(200),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO customer_user (id, login, email, customer_id, pw, title, first_name, last_name, phone, fax, mobile, street, zip, city, country, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (5, 'jane', 'jane@acme.example', 'ACME', NULL, 'Ms', 'Jane', 'Customer', '+1 555 0100', NULL, NULL, 'Main St 1', '12345', 'Springfield', 'US', '', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO customer_user (id, login, email, customer_id, pw, title, first_name, last_name, phone, fax, mobile, street, zip, city, country, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (8, 'bob', 'bob@acme.example', 'ACME', NULL, NULL, 'Bob', 'O''Brien', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE customer_user_customer (
    user_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(150) NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL
);
INSERT INTO customer_user_customer (user_id, customer_id, create_time, create_by, change_time, change_by) VALUES ('bob', 'ACME', '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE dynamic_field (
    id serial,
    internal_field SMALLINT NOT NULL,
    name VARCHAR(200) NOT NULL,
    label VARCHAR(200) NOT NULL,
    field_order INTEGER NOT NULL,
    field_type VARCHAR(200) NOT NULL,
    object_type VARCHAR(100) NOT NULL,
    config TEXT,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO dynamic_field (id, internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 0, 'CustomerReference', 'Customer reference', 1, 'Text', 'Ticket', '---
DefaultValue: ''''
Link: ''''
', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO dynamic_field (id, internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 0, 'NoteKind', 'Note kind', 2, 'Dropdown', 'Article', '---
PossibleValues:
  internal: Internal
  public: Public
', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE dynamic_field_value (
    id serial,
    field_id INTEGER NOT NULL,
    object_id BIGINT NOT NULL,
    value_text TEXT,
    value_date timestamp(0),
    value_int BIGINT,
    PRIMARY KEY(id)
);
INSERT INTO dynamic_field_value (id, field_id, object_id, value_text, value_date, value_int) VALUES (1, 1, 9, 'PO-4711', NULL, NULL);
INSERT INTO dynamic_field_value (id, field_id, object_id, value_text, value_date, value_int) VALUES (2, 2, 12, 'internal', NULL, NULL);
INSERT INTO dynamic_field_value (id, field_id, object_id, value_text, value_date, value_int) VALUES (3, 1, 3, 'REF-0815', NULL, NULL);

CREATE TABLE follow_up_possible (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO follow_up_possible (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'possible', 'Follow-ups for closed tickets are possible. Ticket will be reopened.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO follow_up_possible (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'reject', 'Follow-ups for closed tickets are not possible. No new ticket will be created.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO follow_up_possible (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'new ticket', 'Follow-ups for closed tickets are not possible. A new ticket will be created.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE generic_agent_jobs (
    job_name VARCHAR(200) NOT NULL,
    job_key VARCHAR(200) NOT NULL,
    job_value VARCHAR(200)
);
INSERT INTO generic_agent_jobs (job_name, job_key, job_value) VALUES ('close stale pending', 'NewStateID', '2');
INSERT INTO generic_agent_jobs (job_name, job_key, job_value) VALUES ('close stale pending', 'ScheduleDays', '1');
INSERT INTO generic_agent_jobs (job_name, job_key, job_value) VALUES ('close stale pending', 'Valid', '1');

CREATE TABLE groups (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'users', 'Group for default access.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'admin', 'Group of all administrators.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'stats', 'Group for statistics access.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (4, 'support', 'Support agents', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (5, 'billing', 'Billing agents', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE group_customer_user (
    user_id VARCHAR(100) NOT NULL,
    group_id INTEGER NOT NULL,
    permission_key VARCHAR(20) NOT NULL,
    permission_value SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL
);
INSERT INTO group_customer_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by) VALUES ('jane', 4, 'ro', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO group_customer_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by) VALUES ('jane', 4, 'rw', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE group_role (
    role_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    permission_key VARCHAR(20) NOT NULL,
    permission_value SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL
);
INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by) VALUES (1, 4, 'ro', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by) VALUES (2, 4, 'rw', 0, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by) VALUES (2, 5, 'rw', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE group_user (
    user_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    permission_key VARCHAR(20) NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL
);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (1, 1, 'rw', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (1, 2, 'rw', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (1, 3, 'rw', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (1, 4, 'rw', '2014-02-03 10:00:00', 1, '2014-02-03 10:00:00', 1);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (2, 1, 'rw', '2014-02-03 10:00:00', 1, '2014-02-03 10:00:00', 1);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (3, 4, 'rw', '2014-02-03 10:00:00', 1, '2014-02-03 10:00:00', 1);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (4, 4, 'note', '2014-02-03 10:00:00', 1, '2014-02-03 10:00:00', 1);
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (4, 4, 'rw', '2014-02-03 10:00:00', 1, '2014-02-03 10:00:00', 1);

CREATE TABLE link_object (
    id serial,
    name VARCHAR(100) NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO link_object (id, name) VALUES (1, 'Ticket');

CREATE TABLE link_relation (
    source_object_id SMALLINT NOT NULL,
    source_key VARCHAR(50) NOT NULL,
    target_object_id SMALLINT NOT NULL,
    target_key VARCHAR(50) NOT NULL,
    type_id SMALLINT NOT NULL,
    state_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL
);
INSERT INTO link_relation (source_object_id, source_key, target_object_id, target_key, type_id, state_id, create_time, create_by) VALUES (1, '3', 1, '9', 2, 1, '2014-03-06 10:30:00', 3);

CREATE TABLE link_state (
    id serial,
    name VARCHAR(50) NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO link_state (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Valid', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO link_state (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'Temporary', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE link_type (
    id serial,
    name VARCHAR(50) NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO link_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Normal', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO link_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'ParentChild', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE notification_event (
    id serial,
    name VARCHAR(200) NOT NULL,
    valid_id SMALLINT NOT NULL,
    comments VARCHAR(250),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO notification_event (id, name, valid_id, comments, create_time, create_by, change_time, change_by) VALUES (1, 'Ticket create notification', 1, '', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE notification_event_item (
    notification_id INTEGER NOT NULL,
    event_key VARCHAR(200) NOT NULL,
    event_value VARCHAR(200) NOT NULL
);
INSERT INTO notification_event_item (notification_id, event_key, event_value) VALUES (1, 'Events', 'NotificationNewTicket');
INSERT INTO notification_event_item (notification_id, event_key, event_value) VALUES (1, 'Recipients', 'AgentMyQueues');

CREATE TABLE notification_event_message (
    id serial,
    notification_id INTEGER NOT NULL,
    subject VARCHAR(200) NOT NULL,
    text TEXT NOT NULL,
    content_type VARCHAR(250) NOT NULL,
    language VARCHAR(60) NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO notification_event_message (id, notification_id, subject, text, content_type, language) VALUES (1, 1, 'Ticket Created: <OTRS_TICKET_Title>', 'A new ticket was created.', 'text/plain', 'en');
INSERT INTO notification_event_message (id, notification_id, subject, text, content_type, language) VALUES (2, 1, 'Ticket erstellt: <OTRS_TICKET_Title>', 'Ein neues Ticket wurde erstellt.', 'text/plain', 'de');

CREATE TABLE personal_queues (
    user_id INTEGER NOT NULL,
    queue_id INTEGER NOT NULL
);
INSERT INTO personal_queues (user_id, queue_id) VALUES (4, 5);

CREATE TABLE queue (
    id serial,
    name VARCHAR(200) NOT NULL,
    group_id INTEGER NOT NULL,
    unlock_timeout INTEGER,
    first_response_time INTEGER,
    first_response_notify SMALLINT,
    update_time INTEGER,
    update_notify SMALLINT,
    solution_time INTEGER,
    solution_notify SMALLINT,
    system_address_id SMALLINT NOT NULL,
    calendar_name VARCHAR(100),
    default_sign_key VARCHAR(100),
    salutation_id SMALLINT NOT NULL,
    signature_id SMALLINT NOT NULL,
    follow_up_id SMALLINT NOT NULL,
    follow_up_lock SMALLINT NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, first_response_notify, update_time, update_notify, solution_time, solution_notify, system_address_id, calendar_name, default_sign_key, salutation_id, signature_id, follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Postmaster', 1, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 0, 'Postmaster queue.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, first_response_notify, update_time, update_notify, solution_time, solution_notify, system_address_id, calendar_name, default_sign_key, salutation_id, signature_id, follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'Raw', 1, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 0, 'All default incoming tickets.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, first_response_notify, update_time, update_notify, solution_time, solution_notify, system_address_id, calendar_name, default_sign_key, salutation_id, signature_id, follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'Junk', 1, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 0, 'All junk tickets.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, first_response_notify, update_time, update_notify, solution_time, solution_notify, system_address_id, calendar_name, default_sign_key, salutation_id, signature_id, follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (4, 'Misc', 1, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 0, 'All misc tickets.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, first_response_notify, update_time, update_notify, solution_time, solution_notify, system_address_id, calendar_name, default_sign_key, salutation_id, signature_id, follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (5, 'Support', 4, 0, 60, 0, 0, 0, 480, 0, 2, '', NULL, 1, 1, 3, 1, 'Customer support.', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, first_response_notify, update_time, update_notify, solution_time, solution_notify, system_address_id, calendar_name, default_sign_key, salutation_id, signature_id, follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (6, 'Billing', 5, 0, 0, 0, 0, 0, 0, 0, 2, '', NULL, 1, 1, 1, 0, 'Invoices and refunds.', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE queue_auto_response (
    id serial,
    queue_id INTEGER NOT NULL,
    auto_response_id INTEGER NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO queue_auto_response (id, queue_id, auto_response_id, create_time, create_by, change_time, change_by) VALUES (1, 5, 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE queue_standard_template (
    queue_id INTEGER NOT NULL,
    standard_template_id INTEGER NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL
);
INSERT INTO queue_standard_template (queue_id, standard_template_id, create_time, create_by, change_time, change_by) VALUES (5, 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO queue_standard_template (queue_id, standard_template_id, create_time, create_by, change_time, change_by) VALUES (5, 2, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO queue_standard_template (queue_id, standard_template_id, create_time, create_by, change_time, change_by) VALUES (6, 2, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE roles (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO roles (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Support Team', 'First-line support', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO roles (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'Billing Team', 'Invoices and refunds', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE role_user (
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL
);
INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by) VALUES (3, 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by) VALUES (5, 2, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE salutation (
    id serial,
    name VARCHAR(200) NOT NULL,
    text TEXT NOT NULL,
    content_type VARCHAR(250),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO salutation (id, name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'system standard salutation (en)', 'Dear <OTRS_CUSTOMER_REALNAME>,

Thank you for your request.
', 'text/plain; charset=utf-8', 'Standard Salutation.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE service (
    id serial,
    name VARCHAR(200) NOT NULL,
    valid_id SMALLINT NOT NULL,
    comments VARCHAR(250),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO service (id, name, valid_id, comments, create_time, create_by, change_time, change_by) VALUES (1, 'Printing', 1, '', '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE service_customer_user (
    customer_user_login VARCHAR(200) NOT NULL,
    service_id INTEGER NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL
);
INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by) VALUES ('jane', 1, '2014-02-03 10:00:00', 2);

CREATE TABLE service_sla (
    service_id INTEGER NOT NULL,
    sla_id INTEGER NOT NULL
);
INSERT INTO service_sla (service_id, sla_id) VALUES (1, 1);

CREATE TABLE sessions (
    id bigserial,
    session_id VARCHAR(100) NOT NULL,
    data_key VARCHAR(100) NOT NULL,
    data_value TEXT,
    serialized SMALLINT NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO sessions (id, session_id, data_key, data_value, serialized) VALUES (1, 'a1b2c3', 'UserLogin', 'jdoe', 0);
INSERT INTO sessions (id, session_id, data_key, data_value, serialized) VALUES (2, 'a1b2c3', 'UserID', '2', 0);

CREATE TABLE signature (
    id serial,
    name VARCHAR(200) NOT NULL,
    text TEXT NOT NULL,
    content_type VARCHAR(250),
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO signature (id, name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'system standard signature (en)', '
Your Ticket-Team

 <OTRS_Agent_UserFirstname> <OTRS_Agent_UserLastname>
', 'text/plain; charset=utf-8', 'Standard Signature.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE sla (
    id serial,
    name VARCHAR(200) NOT NULL,
    calendar_name VARCHAR(100),
    first_response_time INTEGER NOT NULL,
    first_response_notify SMALLINT,
    update_time INTEGER NOT NULL,
    update_notify SMALLINT,
    solution_time INTEGER NOT NULL,
    solution_notify SMALLINT,
    valid_id SMALLINT NOT NULL,
    comments VARCHAR(250),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO sla (id, name, calendar_name, first_response_time, first_response_notify, update_time, update_notify, solution_time, solution_notify, valid_id, comments, create_time, create_by, change_time, change_by) VALUES (1, 'Gold', '1', 60, 80, 0, 0, 480, 80, 1, '', '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE standard_attachment (
    id serial,
    name VARCHAR(200) NOT NULL,
    content_type VARCHAR(250) NOT NULL,
    content TEXT NOT NULL,
    filename VARCHAR(250) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO standard_attachment (id, name, content_type, content, filename, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Terms', 'application/pdf', 'JVBERi0xLjMKAP8KdGVybXMK
', 'terms.pdf', '', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE standard_template (
    id serial,
    name VARCHAR(200) NOT NULL,
    text TEXT,
    content_type VARCHAR(250),
    template_type VARCHAR(100) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO standard_template (id, name, text, content_type, template_type, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'empty answer', '', 'text/plain; charset=utf-8', 'Answer', '', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO standard_template (id, name, text, content_type, template_type, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'Invoice copy', 'Dear customer,

please find a copy of your invoice attached.
', 'text/plain; charset=utf-8', 'Answer', '', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE standard_template_attachment (
    id serial,
    standard_attachment_id INTEGER NOT NULL,
    standard_template_id INTEGER NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO standard_template_attachment (id, standard_attachment_id, standard_template_id, create_time, create_by, change_time, change_by) VALUES (1, 1, 2, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE sysconfig_default (
    id serial,
    name VARCHAR(250) NOT NULL,
    description TEXT NOT NULL,
    navigation VARCHAR(200) NOT NULL,
    is_invisible SMALLINT NOT NULL,
    is_readonly SMALLINT NOT NULL,
    is_required SMALLINT NOT NULL,
    is_valid SMALLINT NOT NULL,
    has_configlevel SMALLINT NOT NULL,
    user_modification_possible SMALLINT NOT NULL,
    user_modification_active SMALLINT NOT NULL,
    user_preferences_group VARCHAR(250),
    xml_content_raw TEXT NOT NULL,
    xml_content_parsed TEXT NOT NULL,
    xml_filename VARCHAR(250) NOT NULL,
    effective_value TEXT NOT NULL,
    is_dirty SMALLINT NOT NULL,
    exclusive_lock_guid VARCHAR(32) NOT NULL,
    exclusive_lock_user_id INTEGER,
    exclusive_lock_expiry_time timestamp(0),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO sysconfig_default (id, name, description, navigation, is_invisible, is_readonly, is_required, is_valid, has_configlevel, user_modification_possible, user_modification_active, user_preferences_group, xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty, exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time, create_time, create_by, change_time, change_by) VALUES (601, 'TimeWorkingHours', 'Defines the hours and week days to count the working time.', 'Core::Time', 0, 0, 1, 1, 100, 0, 0, NULL, '<Setting Name="TimeWorkingHours" Required="1" Valid="1"></Setting>', '---
Name: TimeWorkingHours
', 'Calendar.xml', '---
Mon:
- ''8''
- ''9''
', 0, '0', NULL, NULL, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO sysconfig_default (id, name, description, navigation, is_invisible, is_readonly, is_required, is_valid, has_configlevel, user_modification_possible, user_modification_active, user_preferences_group, xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty, exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time, create_time, create_by, change_time, change_by) VALUES (602, 'TimeWorkingHours::Calendar1', 'Defines the hours and week days of the indicated calendar, to count the working time.', 'Core::Time::Calendar1', 0, 0, 1, 1, 100, 0, 0, NULL, '<Setting Name="TimeWorkingHours::Calendar1" Required="1" Valid="1"></Setting>', '---
Name: TimeWorkingHours::Calendar1
', 'Calendar.xml', '---
Mon:
- ''8''
- ''9''
', 0, '0', NULL, NULL, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO sysconfig_default (id, name, description, navigation, is_invisible, is_readonly, is_required, is_valid, has_configlevel, user_modification_possible, user_modification_active, user_preferences_group, xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty, exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time, create_time, create_by, change_time, change_by) VALUES (603, 'Ticket::Hook', 'The identifier for a ticket, e.g. Ticket#, Call#, MyTicket#.', 'Core::Ticket', 0, 0, 1, 1, 100, 0, 0, NULL, '<Setting Name="Ticket::Hook" Required="1" Valid="1"></Setting>', '---
Name: Ticket::Hook
', 'Ticket.xml', '--- Ticket#
', 0, '0', NULL, NULL, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE sysconfig_modified (
    id serial,
    sysconfig_default_id INTEGER NOT NULL,
    name VARCHAR(250) NOT NULL,
    user_id INTEGER,
    is_valid SMALLINT NOT NULL,
    user_modification_active SMALLINT NOT NULL,
    effective_value TEXT NOT NULL,
    is_dirty SMALLINT NOT NULL,
    reset_to_default SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO sysconfig_modified (id, sysconfig_default_id, name, user_id, is_valid, user_modification_active, effective_value, is_dirty, reset_to_default, create_time, create_by, change_time, change_by) VALUES (11, 601, 'TimeWorkingHours', NULL, 1, 0, '---
Mon:
- ''9''
- ''10''
- ''11''
Tue: []
', 0, 0, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO sysconfig_modified (id, sysconfig_default_id, name, user_id, is_valid, user_modification_active, effective_value, is_dirty, reset_to_default, create_time, create_by, change_time, change_by) VALUES (12, 602, 'TimeWorkingHours::Calendar1', NULL, 1, 0, '---
Sat:
- ''10''
- ''11''
', 0, 0, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE system_address (
    id serial,
    value0 VARCHAR(200) NOT NULL,
    value1 VARCHAR(200) NOT NULL,
    value2 VARCHAR(200),
    value3 VARCHAR(200),
    queue_id INTEGER NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO system_address (id, value0, value1, value2, value3, queue_id, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'otrs@localhost', 'OTRS System', NULL, NULL, 1, 'Standard Address.', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO system_address (id, value0, value1, value2, value3, queue_id, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'support@example.com', 'Example Support', NULL, NULL, 5, '', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE ticket (
    id bigserial,
    tn VARCHAR(50) NOT NULL,
    title VARCHAR(255),
    queue_id INTEGER NOT NULL,
    ticket_lock_id SMALLINT NOT NULL,
    type_id SMALLINT,
    service_id INTEGER,
    sla_id INTEGER,
    user_id INTEGER NOT NULL,
    responsible_user_id INTEGER NOT NULL,
    ticket_priority_id SMALLINT NOT NULL,
    ticket_state_id SMALLINT NOT NULL,
    customer_id VARCHAR(150),
    customer_user_id VARCHAR(250),
    timeout INTEGER NOT NULL,
    until_time INTEGER NOT NULL,
    escalation_time INTEGER NOT NULL,
    escalation_update_time INTEGER NOT NULL,
    escalation_response_time INTEGER NOT NULL,
    escalation_solution_time INTEGER NOT NULL,
    archive_flag SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, service_id, sla_id, user_id, responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time, escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by) VALUES (15, '2014030710000024', 'Refund request', 6, 1, 1, 1, 1, 5, 5, 3, 4, 'ACME', 'jane', 0, 0, 0, 0, 0, 0, 0, '2014-03-07 09:00:00', 5, '2014-03-07 09:00:00', 5);
INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, service_id, sla_id, user_id, responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time, escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by) VALUES (3, '2014030510000013', 'Printer on fire', 1, 1, 1, NULL, NULL, 2, 2, 3, 4, 'ACME', 'jane', 0, 0, 0, 0, 0, 0, 0, '2014-03-06 00:10:00', 1, '2014-03-06 00:10:00', 1);
INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, service_id, sla_id, user_id, responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time, escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by) VALUES (9, '2014030610000091', 'Invoice question', 5, 1, 2, NULL, NULL, 3, 3, 4, 2, 'ACME', 'bob', 0, 0, 0, 0, 0, 0, 0, '2014-03-06 10:00:00', 2, '2014-03-06 11:00:00', 3);

CREATE TABLE ticket_flag (
    ticket_id BIGINT NOT NULL,
    ticket_key VARCHAR(50) NOT NULL,
    ticket_value VARCHAR(50),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL
);
INSERT INTO ticket_flag (ticket_id, ticket_key, ticket_value, create_time, create_by) VALUES (3, 'Seen', '1', '2014-03-06 10:30:00', 2);
INSERT INTO ticket_flag (ticket_id, ticket_key, ticket_value, create_time, create_by) VALUES (9, 'Seen', '1', '2014-03-06 10:30:00', 3);

CREATE TABLE ticket_history (
    id bigserial,
    name VARCHAR(200) NOT NULL,
    history_type_id SMALLINT NOT NULL,
    ticket_id BIGINT NOT NULL,
    article_id BIGINT,
    type_id SMALLINT NOT NULL,
    queue_id INTEGER NOT NULL,
    owner_id INTEGER NOT NULL,
    priority_id SMALLINT NOT NULL,
    state_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket_history (id, name, history_type_id, ticket_id, article_id, type_id, queue_id, owner_id, priority_id, state_id, create_time, create_by, change_time, change_by) VALUES (100, '%%2014030510000013%%Postmaster%%3 normal%%open%%3', 1, 3, 7, 1, 1, 2, 3, 4, '2014-03-06 00:10:00', 1, '2014-03-06 00:10:00', 1);
INSERT INTO ticket_history (id, name, history_type_id, ticket_id, article_id, type_id, queue_id, owner_id, priority_id, state_id, create_time, create_by, change_time, change_by) VALUES (101, '%%bob@acme.example, ', 8, 9, 12, 2, 5, 3, 4, 2, '2014-03-06 10:30:00', 3, '2014-03-06 10:30:00', 3);
INSERT INTO ticket_history (id, name, history_type_id, ticket_id, article_id, type_id, queue_id, owner_id, priority_id, state_id, create_time, create_by, change_time, change_by) VALUES (102, '%%open%%closed successful%%', 27, 9, NULL, 2, 5, 3, 4, 2, '2014-03-06 11:00:00', 3, '2014-03-06 11:00:00', 3);

CREATE TABLE ticket_history_type (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket_history_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'NewTicket', '', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_history_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (12, 'EmailCustomer', '', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_history_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (16, 'Move', '', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_history_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (27, 'StateUpdate', '', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_history_type (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (8, 'SendAnswer', '', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE ticket_lock_type (
    id serial,
    name VARCHAR(200) NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket_lock_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'unlock', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_lock_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'lock', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_lock_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'tmp_lock', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE ticket_priority (
    id serial,
    name VARCHAR(200) NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket_priority (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (1, '1 very low', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_priority (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (2, '2 low', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_priority (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (3, '3 normal', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_priority (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (4, '4 high', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_priority (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (5, '5 very high', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE ticket_state (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    type_id SMALLINT NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'new', 'New ticket created by customer.', 1, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'closed successful', 'Ticket is closed successful.', 3, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'closed unsuccessful', 'Ticket is closed unsuccessful.', 3, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (4, 'open', 'Open tickets.', 2, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (5, 'removed', 'Customer removed ticket.', 6, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (6, 'pending reminder', 'Ticket is pending for agent reminder.', 4, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (7, 'pending auto close+', 'Ticket is pending for automatic close.', 5, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (8, 'pending auto close-', 'Ticket is pending for automatic close.', 5, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES (9, 'merged', 'State for merged tickets.', 7, 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE ticket_state_type (
    id serial,
    name VARCHAR(200) NOT NULL,
    comments VARCHAR(250),
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES (1, 'new', 'All new state types (default: viewable).', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES (2, 'open', 'All open state types (default: viewable).', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES (3, 'closed', 'All closed state types (default: not viewable).', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES (4, 'pending reminder', 'All ''pending reminder'' state types (default: viewable).', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES (5, 'pending auto', 'All ''pending auto *'' state types (default: viewable).', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES (6, 'removed', 'All ''removed'' state types (default: not viewable).', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES (7, 'merged', 'State type for merged tickets (default: not viewable).', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);

CREATE TABLE ticket_type (
    id serial,
    name VARCHAR(200) NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO ticket_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'Unclassified', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO ticket_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'Problem', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE ticket_watcher (
    ticket_id BIGINT NOT NULL,
    user_id INTEGER NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL
);
INSERT INTO ticket_watcher (ticket_id, user_id, create_time, create_by, change_time, change_by) VALUES (9, 2, '2014-03-06 10:30:00', 2, '2014-03-06 10:30:00', 2);

CREATE TABLE time_accounting (
    id bigserial,
    ticket_id BIGINT NOT NULL,
    article_id BIGINT,
    time_unit DECIMAL(10,2) NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO time_accounting (id, ticket_id, article_id, time_unit, create_time, create_by, change_time, change_by) VALUES (1, 9, 12, 15.50, '2014-03-06 10:30:00', 3, '2014-03-06 10:30:00', 3);
INSERT INTO time_accounting (id, ticket_id, article_id, time_unit, create_time, create_by, change_time, change_by) VALUES (2, 3, NULL, 0.25, '2014-03-06 10:30:00', 2, '2014-03-06 10:30:00', 2);

CREATE TABLE users (
    id serial,
    login VARCHAR(200) NOT NULL,
    pw VARCHAR(128) NOT NULL,
    title VARCHAR(50),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    valid_id SMALLINT NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES (1, 'root@localhost', '$2a$12$K9rNSSLSkFjYCa7PeUGJFurWvW7o.0QZoYHqpm2b7r1n7Vf2xQ1l2', '', 'Admin', 'OTRS', 1, '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES (2, 'jdoe', '$2a$12$3m0Q4Vn0RYnqK0q9mJ9oP.4QJ8pX8m5n5Q0QXUQ6qX2bB0KZbY5uO', NULL, 'John', 'Doe', 1, '2014-02-01 09:00:00', 1, '2014-03-01 12:00:00', 3);
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES (3, 'asmith', '$2a$12$Wl1v8H9m1K8a5w0cBv8xOe2rq8Q8yqXJm0W8o6s2Q5R0s1a2b3c4d', NULL, 'Alice', 'Smith', 1, '2014-02-02 09:00:00', 2, '2014-02-02 09:00:00', 2);
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES (4, 'kgroup', '0f7a28def39f55f118b12fa07c73885e9ed4bbb09598bb51a33f682857948cdb', NULL, 'Karl', 'Group', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES (5, 'lrole', 'BCRYPT:5:Q7rTx2LmP9vKa4Zc:GaMb8oyK3GyY0ZLg4MKgt8VZP5.ZARi', NULL, 'Lena', 'Role', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);
INSERT INTO users (id, login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES (6, 'mnone', 'bf1da987e3ecd80090eb6821b65df98d2820d9454fa5cd10137421d5f6fcf886', NULL, 'Max', 'None', 1, '2014-02-03 10:00:00', 2, '2014-02-03 10:00:00', 2);

CREATE TABLE user_preferences (
    user_id INTEGER NOT NULL,
    preferences_key VARCHAR(150) NOT NULL,
    preferences_value TEXT
);
INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (1, 'UserLanguage', 'en');
INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (4, 'UserLanguage', 'de');
INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (4, 'UserSignature', 'Grüße,
Karl');

CREATE TABLE valid (
    id serial,
    name VARCHAR(200) NOT NULL,
    create_time timestamp(0) NOT NULL,
    create_by INTEGER NOT NULL,
    change_time timestamp(0) NOT NULL,
    change_by INTEGER NOT NULL,
    PRIMARY KEY(id)
);
INSERT INTO valid (id, name, create_time, create_by, change_time, change_by) VALUES (1, 'valid', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO valid (id, name, create_time, create_by, change_time, change_by) VALUES (2, 'invalid', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
INSERT INTO valid (id, name, create_time, create_by, change_time, change_by) VALUES (3, 'invalid-temporarily', '2014-01-01 00:00:00', 1, '2014-01-01 00:00:00', 1);
