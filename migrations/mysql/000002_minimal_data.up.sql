-- Minimal upstream-compatible seed data for MySQL deployments.
START TRANSACTION;

INSERT IGNORE INTO valid (id, name, create_time, create_by, change_time, change_by) VALUES
(1, 'valid', NOW(), 1, NOW(), 1),
(2, 'invalid', NOW(), 1, NOW(), 1),
(3, 'invalid-temporarily', NOW(), 1, NOW(), 1);

INSERT IGNORE INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'new', 'New ticket', 1, 1, NOW(), 1, NOW(), 1),
(2, 'open', 'Open tickets', 2, 1, NOW(), 1, NOW(), 1),
(3, 'pending reminder', 'Pending reminder', 3, 1, NOW(), 1, NOW(), 1),
(4, 'closed successful', 'Closed successful', 5, 1, NOW(), 1, NOW(), 1),
(5, 'closed unsuccessful', 'Closed unsuccessful', 5, 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by) VALUES
(1, 'new', 'All new state types', NOW(), 1, NOW(), 1),
(2, 'open', 'All open state types', NOW(), 1, NOW(), 1),
(3, 'pending reminder', 'All pending reminder state types', NOW(), 1, NOW(), 1),
(4, 'pending auto', 'All pending auto state types', NOW(), 1, NOW(), 1),
(5, 'closed', 'All closed state types', NOW(), 1, NOW(), 1);

INSERT IGNORE INTO ticket_priority (id, name, valid_id, color, create_time, create_by, change_time, change_by) VALUES
(1, '1 very low', 1, '#03c4f0', NOW(), 1, NOW(), 1),
(2, '2 low', 1, '#83bfc8', NOW(), 1, NOW(), 1),
(3, '3 normal', 1, '#cdcdcd', NOW(), 1, NOW(), 1),
(4, '4 high', 1, '#ffaaaa', NOW(), 1, NOW(), 1),
(5, '5 very high', 1, '#ff505e', NOW(), 1, NOW(), 1);

INSERT IGNORE INTO ticket_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'Unclassified', 1, NOW(), 1, NOW(), 1),
(2, 'Incident', 1, NOW(), 1, NOW(), 1),
(3, 'Service Request', 1, NOW(), 1, NOW(), 1),
(4, 'Problem', 1, NOW(), 1, NOW(), 1),
(5, 'Change Request', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO ticket_lock_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'unlock', 1, NOW(), 1, NOW(), 1),
(2, 'lock', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO article_sender_type (id, name, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'agent', 1, NOW(), 1, NOW(), 1),
(2, 'system', 1, NOW(), 1, NOW(), 1),
(3, 'customer', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO communication_channel (id, name, module, package_name, channel_data, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'Email', 'Kernel::System::CommunicationChannel::Email', 'Framework', '{}', 1, NOW(), 1, NOW(), 1),
(2, 'Phone', 'Kernel::System::CommunicationChannel::Phone', 'Framework', '{}', 1, NOW(), 1, NOW(), 1),
(3, 'Internal', 'Kernel::System::CommunicationChannel::Internal', 'Framework', '{}', 1, NOW(), 1, NOW(), 1),
(4, 'Chat', 'Kernel::System::CommunicationChannel::Chat', 'Framework', '{}', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO follow_up_possible (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'possible', 'Follow-ups for closed tickets are possible.', 1, NOW(), 1, NOW(), 1),
(2, 'reject', 'Follow-ups for closed tickets are rejected.', 1, NOW(), 1, NOW(), 1),
(3, 'new ticket', 'Follow-ups for closed tickets create new tickets.', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO groups (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'users', 'Standard users group', 1, NOW(), 1, NOW(), 1),
(2, 'admin', 'Admin group', 1, NOW(), 1, NOW(), 1),
(3, 'stats', 'Stats access group', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO system_address (id, value0, value1, comments, valid_id, queue_id, create_time, create_by, change_time, change_by) VALUES
(1, 'postmaster@gotrs.local', 'GOTRS Postmaster', 'System address for the Postmaster queue', 1, 1, NOW(), 1, NOW(), 1),
(2, 'intake@gotrs.local', 'GOTRS Intake', 'Raw/unprocessed queue system address', 1, 2, NOW(), 1, NOW(), 1),
(3, 'junk@gotrs.local', 'GOTRS Junk Monitor', 'Spam quarantine queue address', 1, 3, NOW(), 1, NOW(), 1),
(4, 'misc@gotrs.local', 'GOTRS Misc', 'Miscellaneous work queue address', 1, 4, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO salutation (id, name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'Default', 'Dear Customer,', 'text/plain', 'Default salutation', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO signature (id, name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'Default', 'Your Support Team', 'text/plain', 'Default signature', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO queue (
    id,
    name,
    group_id,
    system_address_id,
    salutation_id,
    signature_id,
    unlock_timeout,
    follow_up_id,
    follow_up_lock,
    comments,
    valid_id,
    create_time,
    create_by,
    change_time,
    change_by
) VALUES
(1, 'Postmaster', 1, 1, 1, 1, 0, 1, 0, 'Default queue for incoming emails', 1, NOW(), 1, NOW(), 1),
(2, 'Raw', 1, 2, 1, 1, 0, 1, 0, 'Queue for unprocessed emails', 1, NOW(), 1, NOW(), 1),
(3, 'Junk', 1, 3, 1, 1, 0, 2, 0, 'Queue for junk/spam', 1, NOW(), 1, NOW(), 1),
(4, 'Misc', 1, 4, 1, 1, 0, 1, 0, 'Miscellaneous queue', 1, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO users (id, login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES
(1, 'root@localhost', SHA2(UUID(), 256), 'System', 'Administrator', 2, NOW(), 1, NOW(), 1);

INSERT IGNORE INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES
(1, 1, 'rw', NOW(), 1, NOW(), 1),
(1, 2, 'rw', NOW(), 1, NOW(), 1),
(1, 3, 'rw', NOW(), 1, NOW(), 1);

COMMIT;
