-- OTRS Initial Data
-- Essential data for OTRS-compatible system

-- ============================================
-- Ticket State Types
-- ============================================
INSERT INTO ticket_state_type (id, name, comments, create_by, change_by) VALUES
(1, 'new', 'New ticket states', 1, 1),
(2, 'open', 'Open ticket states', 1, 1),
(3, 'closed', 'Closed ticket states', 1, 1),
(4, 'pending reminder', 'Pending reminder states', 1, 1),
(5, 'pending auto', 'Pending auto states', 1, 1),
(6, 'removed', 'Removed states', 1, 1),
(7, 'merged', 'Merged states', 1, 1);

-- ============================================
-- Ticket States
-- ============================================
INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_by, change_by) VALUES
(1, 'new', 'New ticket', 1, 1, 1, 1),
(2, 'open', 'Open tickets', 2, 1, 1, 1),
(3, 'closed successful', 'Ticket closed successfully', 3, 1, 1, 1),
(4, 'closed unsuccessful', 'Ticket closed unsuccessfully', 3, 1, 1, 1),
(5, 'pending reminder', 'Ticket pending reminder', 4, 1, 1, 1),
(6, 'pending auto close+', 'Ticket pending auto close', 5, 1, 1, 1),
(7, 'pending auto close-', 'Ticket pending auto close', 5, 1, 1, 1),
(8, 'removed', 'Ticket removed', 6, 1, 1, 1),
(9, 'merged', 'Ticket merged', 7, 1, 1, 1);

-- ============================================
-- Ticket Priorities
-- ============================================
INSERT INTO ticket_priority (id, name, color, valid_id, create_by, change_by) VALUES
(1, '1 very low', '#03c4f0', 1, 1, 1),
(2, '2 low', '#83bfc8', 1, 1, 1),
(3, '3 normal', '#cdcdcd', 1, 1, 1),
(4, '4 high', '#ffaaaa', 1, 1, 1),
(5, '5 very high', '#ff5555', 1, 1, 1);

-- ============================================
-- Ticket Types
-- ============================================
INSERT INTO ticket_type (id, name, comments, valid_id, create_by, change_by) VALUES
(1, 'Unclassified', 'Default type for unclassified tickets', 1, 1, 1),
(2, 'Incident', 'Service incidents', 1, 1, 1),
(3, 'Service Request', 'Service requests', 1, 1, 1),
(4, 'Problem', 'Problem tickets', 1, 1, 1),
(5, 'Change Request', 'Change requests', 1, 1, 1);

-- ============================================
-- Ticket Lock Types
-- ============================================
INSERT INTO ticket_lock_type (id, name, comments, valid_id, create_by, change_by) VALUES
(1, 'unlock', 'Ticket unlocked', 1, 1, 1),
(2, 'lock', 'Ticket locked', 1, 1, 1),
(3, 'tmp_lock', 'Ticket temporarily locked', 1, 1, 1);

-- ============================================
-- Article Sender Types
-- ============================================
INSERT INTO article_sender_type (id, name, comments, valid_id, create_by, change_by) VALUES
(1, 'agent', 'Agent message', 1, 1, 1),
(2, 'system', 'System message', 1, 1, 1),
(3, 'customer', 'Customer message', 1, 1, 1);

-- ============================================
-- Ticket History Types
-- ============================================
INSERT INTO ticket_history_type (id, name, comments, valid_id, create_by, change_by) VALUES
(1, 'NewTicket', 'New ticket created', 1, 1, 1),
(2, 'TicketStateUpdate', 'Ticket state updated', 1, 1, 1),
(3, 'TicketPriorityUpdate', 'Ticket priority updated', 1, 1, 1),
(4, 'TicketQueueUpdate', 'Ticket queue updated', 1, 1, 1),
(5, 'TicketTitleUpdate', 'Ticket title updated', 1, 1, 1),
(6, 'TicketOwnerUpdate', 'Ticket owner updated', 1, 1, 1),
(7, 'TicketResponsibleUpdate', 'Ticket responsible updated', 1, 1, 1),
(8, 'TicketCustomerUpdate', 'Ticket customer updated', 1, 1, 1),
(9, 'TicketFreeTextUpdate', 'Ticket free text updated', 1, 1, 1),
(10, 'TicketFreeTimeUpdate', 'Ticket free time updated', 1, 1, 1),
(11, 'TicketPendingTimeUpdate', 'Ticket pending time updated', 1, 1, 1),
(12, 'Lock', 'Ticket locked', 1, 1, 1),
(13, 'Unlock', 'Ticket unlocked', 1, 1, 1),
(14, 'Move', 'Ticket moved', 1, 1, 1),
(15, 'AddNote', 'Note added', 1, 1, 1),
(16, 'Close', 'Ticket closed', 1, 1, 1),
(17, 'CustomerUpdate', 'Customer updated', 1, 1, 1),
(18, 'PriorityUpdate', 'Priority updated', 1, 1, 1),
(19, 'OwnerUpdate', 'Owner updated', 1, 1, 1),
(20, 'ResponsibleUpdate', 'Responsible updated', 1, 1, 1),
(21, 'Subscribe', 'Subscribed', 1, 1, 1),
(22, 'Unsubscribe', 'Unsubscribed', 1, 1, 1),
(23, 'SystemRequest', 'System request', 1, 1, 1),
(24, 'FollowUp', 'Follow up', 1, 1, 1),
(25, 'SendAutoReject', 'Auto reject sent', 1, 1, 1),
(26, 'SendAutoReply', 'Auto reply sent', 1, 1, 1),
(27, 'SendAutoFollowUp', 'Auto follow up sent', 1, 1, 1),
(28, 'Forward', 'Forwarded', 1, 1, 1),
(29, 'Bounce', 'Bounced', 1, 1, 1),
(30, 'SendAnswer', 'Answer sent', 1, 1, 1),
(31, 'SendAgentNotification', 'Agent notification sent', 1, 1, 1),
(32, 'SendCustomerNotification', 'Customer notification sent', 1, 1, 1),
(33, 'EmailAgent', 'Email to agent', 1, 1, 1),
(34, 'EmailCustomer', 'Email to customer', 1, 1, 1),
(35, 'PhoneCallAgent', 'Phone call from agent', 1, 1, 1),
(36, 'PhoneCallCustomer', 'Phone call from customer', 1, 1, 1),
(37, 'WebRequestCustomer', 'Web request from customer', 1, 1, 1),
(38, 'Misc', 'Miscellaneous', 1, 1, 1),
(39, 'ArchiveFlagUpdate', 'Archive flag updated', 1, 1, 1),
(40, 'EscalationSolutionTimeStop', 'Escalation solution time stopped', 1, 1, 1),
(41, 'EscalationResponseTimeStart', 'Escalation response time started', 1, 1, 1),
(42, 'EscalationUpdateTimeStart', 'Escalation update time started', 1, 1, 1),
(43, 'EscalationSolutionTimeStart', 'Escalation solution time started', 1, 1, 1),
(44, 'EscalationResponseTimeNotifyBefore', 'Escalation response time notify before', 1, 1, 1),
(45, 'EscalationUpdateTimeNotifyBefore', 'Escalation update time notify before', 1, 1, 1),
(46, 'EscalationSolutionTimeNotifyBefore', 'Escalation solution time notify before', 1, 1, 1),
(47, 'EscalationResponseTimeStop', 'Escalation response time stopped', 1, 1, 1),
(48, 'EscalationUpdateTimeStop', 'Escalation update time stopped', 1, 1, 1);

-- ============================================
-- Default Queue
-- ============================================
INSERT INTO queue (id, name, group_id, unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id, create_by, change_by) VALUES
(1, 'Postmaster', 1, 0, 1, 0, 'Default queue for all incoming emails', 1, 1, 1),
(2, 'Raw', 1, 0, 1, 0, 'Queue for unprocessed emails', 1, 1, 1),
(3, 'Junk', 1, 0, 1, 0, 'Queue for junk/spam', 1, 1, 1),
(4, 'Misc', 2, 0, 1, 0, 'Miscellaneous tickets', 1, 1, 1);

-- Reset sequences
SELECT setval('ticket_state_type_id_seq', 7, true);
SELECT setval('ticket_state_id_seq', 9, true);
SELECT setval('ticket_priority_id_seq', 5, true);
SELECT setval('ticket_type_id_seq', 5, true);
SELECT setval('ticket_lock_type_id_seq', 3, true);
SELECT setval('article_sender_type_id_seq', 3, true);
SELECT setval('ticket_history_type_id_seq', 48, true);
SELECT setval('queue_id_seq', 4, true);