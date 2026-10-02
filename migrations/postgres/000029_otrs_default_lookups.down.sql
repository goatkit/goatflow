-- Remove the default lookup rows added by 000029, but only rows nothing
-- references (an OTRS-imported database may have had them before 000029).

DELETE FROM ticket_state
WHERE name IN ('pending auto close+', 'pending auto close-', 'removed', 'merged')
  AND NOT EXISTS (SELECT 1 FROM ticket r WHERE r.ticket_state_id = ticket_state.id)
  AND NOT EXISTS (SELECT 1 FROM ticket_history r WHERE r.state_id = ticket_state.id);

DELETE FROM ticket_state_type
WHERE name IN ('removed', 'merged')
  AND NOT EXISTS (SELECT 1 FROM ticket_state r WHERE r.type_id = ticket_state_type.id);

DELETE FROM ticket_history_type
WHERE name IN ('NewTicket', 'FollowUp', 'SendAutoReject', 'SendAutoReply', 'SendAutoFollowUp', 'Forward', 'Bounce', 'SendAnswer', 'SendAgentNotification', 'SendCustomerNotification', 'EmailAgent', 'EmailCustomer', 'PhoneCallAgent', 'PhoneCallCustomer', 'AddNote', 'Move', 'Lock', 'Unlock', 'Remove', 'TimeAccounting', 'CustomerUpdate', 'PriorityUpdate', 'OwnerUpdate', 'LoopProtection', 'Misc', 'SetPendingTime', 'StateUpdate', 'TicketDynamicFieldUpdate', 'WebRequestCustomer', 'TicketLinkAdd', 'TicketLinkDelete', 'SystemRequest', 'Merged', 'ResponsibleUpdate', 'Subscribe', 'Unsubscribe', 'TypeUpdate', 'ServiceUpdate', 'SLAUpdate', 'ArchiveFlagUpdate', 'EscalationSolutionTimeStop', 'EscalationResponseTimeStart', 'EscalationUpdateTimeStart', 'EscalationSolutionTimeStart', 'EscalationResponseTimeNotifyBefore', 'EscalationUpdateTimeNotifyBefore', 'EscalationSolutionTimeNotifyBefore', 'EscalationResponseTimeStop', 'EscalationUpdateTimeStop', 'TitleUpdate', 'EmailResend')
  AND NOT EXISTS (SELECT 1 FROM ticket_history r WHERE r.history_type_id = ticket_history_type.id);

DELETE FROM ticket_lock_type
WHERE name IN ('tmp_lock')
  AND NOT EXISTS (SELECT 1 FROM ticket r WHERE r.ticket_lock_id = ticket_lock_type.id);

DELETE FROM link_type
WHERE name IN ('Normal', 'ParentChild')
  AND NOT EXISTS (SELECT 1 FROM link_relation r WHERE r.type_id = link_type.id);

DELETE FROM link_state
WHERE name IN ('Valid', 'Temporary')
  AND NOT EXISTS (SELECT 1 FROM link_relation r WHERE r.state_id = link_state.id);

DELETE FROM link_object
WHERE name = 'Ticket'
  AND NOT EXISTS (SELECT 1 FROM link_relation r WHERE r.source_object_id = link_object.id OR r.target_object_id = link_object.id);

DELETE FROM auto_response_type
WHERE name IN ('auto reply', 'auto reject', 'auto follow up', 'auto reply/new ticket', 'auto remove')
  AND NOT EXISTS (SELECT 1 FROM auto_response r WHERE r.type_id = auto_response_type.id);
