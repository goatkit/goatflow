-- OTRS/Znuny default lookup rows that a fresh GoatFlow install lacks.
--
-- Databases imported from OTRS/Znuny already carry these rows (with OTRS ids);
-- fresh installs seeded by 000002 do not. Rows are matched by name only:
-- nothing is inserted when a row of that name exists, and existing ids are
-- never changed. Application code resolves every lookup by name, so the ids
-- these rows receive do not matter. Identical SQL on MySQL and PostgreSQL.

-- Ticket state types (OTRS: new, open, closed, pending reminder, pending auto, removed, merged).
INSERT INTO ticket_state_type (name, comments, create_time, create_by, change_time, change_by)
SELECT v.name, v.comments, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (
    SELECT 1 AS ord, 'removed' AS name, 'All ''removed'' state types (default: viewable).' AS comments
    UNION ALL SELECT 2 AS ord, 'merged' AS name, 'State type for merged tickets (default: viewable).' AS comments
) v
WHERE NOT EXISTS (SELECT 1 FROM ticket_state_type x WHERE x.name = v.name)
ORDER BY v.ord;

-- Ticket states (OTRS defaults missing from 000002). type_id resolved by state type name.
INSERT INTO ticket_state (name, comments, type_id, valid_id, create_time, create_by, change_time, change_by)
SELECT v.name, v.comments, tst.id, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (
    SELECT 1 AS ord, 'pending auto close+' AS name, 'Ticket is pending for automatic close.' AS comments, 'pending auto' AS type_name
    UNION ALL SELECT 2 AS ord, 'pending auto close-' AS name, 'Ticket is pending for automatic close.' AS comments, 'pending auto' AS type_name
    UNION ALL SELECT 3 AS ord, 'removed' AS name, 'Customer removed ticket request.' AS comments, 'removed' AS type_name
    UNION ALL SELECT 4 AS ord, 'merged' AS name, 'State for merged tickets.' AS comments, 'merged' AS type_name
) v
JOIN ticket_state_type tst ON tst.name = v.type_name
WHERE NOT EXISTS (SELECT 1 FROM ticket_state x WHERE x.name = v.name)
ORDER BY v.ord;

-- Ticket history types (full OTRS/Znuny list).
INSERT INTO ticket_history_type (name, valid_id, create_time, create_by, change_time, change_by)
SELECT v.name, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (
    SELECT 1 AS ord, 'NewTicket' AS name
    UNION ALL SELECT 2 AS ord, 'FollowUp' AS name
    UNION ALL SELECT 3 AS ord, 'SendAutoReject' AS name
    UNION ALL SELECT 4 AS ord, 'SendAutoReply' AS name
    UNION ALL SELECT 5 AS ord, 'SendAutoFollowUp' AS name
    UNION ALL SELECT 6 AS ord, 'Forward' AS name
    UNION ALL SELECT 7 AS ord, 'Bounce' AS name
    UNION ALL SELECT 8 AS ord, 'SendAnswer' AS name
    UNION ALL SELECT 9 AS ord, 'SendAgentNotification' AS name
    UNION ALL SELECT 10 AS ord, 'SendCustomerNotification' AS name
    UNION ALL SELECT 11 AS ord, 'EmailAgent' AS name
    UNION ALL SELECT 12 AS ord, 'EmailCustomer' AS name
    UNION ALL SELECT 13 AS ord, 'PhoneCallAgent' AS name
    UNION ALL SELECT 14 AS ord, 'PhoneCallCustomer' AS name
    UNION ALL SELECT 15 AS ord, 'AddNote' AS name
    UNION ALL SELECT 16 AS ord, 'Move' AS name
    UNION ALL SELECT 17 AS ord, 'Lock' AS name
    UNION ALL SELECT 18 AS ord, 'Unlock' AS name
    UNION ALL SELECT 19 AS ord, 'Remove' AS name
    UNION ALL SELECT 20 AS ord, 'TimeAccounting' AS name
    UNION ALL SELECT 21 AS ord, 'CustomerUpdate' AS name
    UNION ALL SELECT 22 AS ord, 'PriorityUpdate' AS name
    UNION ALL SELECT 23 AS ord, 'OwnerUpdate' AS name
    UNION ALL SELECT 24 AS ord, 'LoopProtection' AS name
    UNION ALL SELECT 25 AS ord, 'Misc' AS name
    UNION ALL SELECT 26 AS ord, 'SetPendingTime' AS name
    UNION ALL SELECT 27 AS ord, 'StateUpdate' AS name
    UNION ALL SELECT 28 AS ord, 'TicketDynamicFieldUpdate' AS name
    UNION ALL SELECT 29 AS ord, 'WebRequestCustomer' AS name
    UNION ALL SELECT 30 AS ord, 'TicketLinkAdd' AS name
    UNION ALL SELECT 31 AS ord, 'TicketLinkDelete' AS name
    UNION ALL SELECT 32 AS ord, 'SystemRequest' AS name
    UNION ALL SELECT 33 AS ord, 'Merged' AS name
    UNION ALL SELECT 34 AS ord, 'ResponsibleUpdate' AS name
    UNION ALL SELECT 35 AS ord, 'Subscribe' AS name
    UNION ALL SELECT 36 AS ord, 'Unsubscribe' AS name
    UNION ALL SELECT 37 AS ord, 'TypeUpdate' AS name
    UNION ALL SELECT 38 AS ord, 'ServiceUpdate' AS name
    UNION ALL SELECT 39 AS ord, 'SLAUpdate' AS name
    UNION ALL SELECT 40 AS ord, 'ArchiveFlagUpdate' AS name
    UNION ALL SELECT 41 AS ord, 'EscalationSolutionTimeStop' AS name
    UNION ALL SELECT 42 AS ord, 'EscalationResponseTimeStart' AS name
    UNION ALL SELECT 43 AS ord, 'EscalationUpdateTimeStart' AS name
    UNION ALL SELECT 44 AS ord, 'EscalationSolutionTimeStart' AS name
    UNION ALL SELECT 45 AS ord, 'EscalationResponseTimeNotifyBefore' AS name
    UNION ALL SELECT 46 AS ord, 'EscalationUpdateTimeNotifyBefore' AS name
    UNION ALL SELECT 47 AS ord, 'EscalationSolutionTimeNotifyBefore' AS name
    UNION ALL SELECT 48 AS ord, 'EscalationResponseTimeStop' AS name
    UNION ALL SELECT 49 AS ord, 'EscalationUpdateTimeStop' AS name
    UNION ALL SELECT 50 AS ord, 'TitleUpdate' AS name
    UNION ALL SELECT 51 AS ord, 'EmailResend' AS name
) v
WHERE NOT EXISTS (SELECT 1 FROM ticket_history_type x WHERE x.name = v.name)
ORDER BY v.ord;

-- Ticket lock types (OTRS: unlock, lock, tmp_lock).
INSERT INTO ticket_lock_type (name, valid_id, create_time, create_by, change_time, change_by)
SELECT v.name, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (
    SELECT 1 AS ord, 'tmp_lock' AS name
) v
WHERE NOT EXISTS (SELECT 1 FROM ticket_lock_type x WHERE x.name = v.name)
ORDER BY v.ord;

-- Link types.
INSERT INTO link_type (name, valid_id, create_time, create_by, change_time, change_by)
SELECT v.name, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (
    SELECT 1 AS ord, 'Normal' AS name
    UNION ALL SELECT 2 AS ord, 'ParentChild' AS name
) v
WHERE NOT EXISTS (SELECT 1 FROM link_type x WHERE x.name = v.name)
ORDER BY v.ord;

-- Link states.
INSERT INTO link_state (name, valid_id, create_time, create_by, change_time, change_by)
SELECT v.name, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (
    SELECT 1 AS ord, 'Valid' AS name
    UNION ALL SELECT 2 AS ord, 'Temporary' AS name
) v
WHERE NOT EXISTS (SELECT 1 FROM link_state x WHERE x.name = v.name)
ORDER BY v.ord;

-- Link object classes (OTRS creates 'Ticket' on first use).
INSERT INTO link_object (name)
SELECT v.name
FROM (
    SELECT 1 AS ord, 'Ticket' AS name
) v
WHERE NOT EXISTS (SELECT 1 FROM link_object x WHERE x.name = v.name)
ORDER BY v.ord;

-- Auto response types.
INSERT INTO auto_response_type (name, comments, valid_id, create_time, create_by, change_time, change_by)
SELECT v.name, v.comments, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (
    SELECT 1 AS ord, 'auto reply' AS name, 'Automatic reply which will be sent out after a new ticket has been created.' AS comments
    UNION ALL SELECT 2 AS ord, 'auto reject' AS name, 'Automatic reject which will be sent out after a follow-up has been rejected (in case queue follow-up option is "reject").' AS comments
    UNION ALL SELECT 3 AS ord, 'auto follow up' AS name, 'Automatic confirmation which is sent out after a follow-up has been received for a ticket (in case queue follow-up option is "possible").' AS comments
    UNION ALL SELECT 4 AS ord, 'auto reply/new ticket' AS name, 'Automatic response which will be sent out after a follow-up has been rejected and a new ticket has been created (in case queue follow-up option is "new ticket").' AS comments
    UNION ALL SELECT 5 AS ord, 'auto remove' AS name, 'Auto remove will be sent out after a customer removed the request.' AS comments
) v
WHERE NOT EXISTS (SELECT 1 FROM auto_response_type x WHERE x.name = v.name)
ORDER BY v.ord;
