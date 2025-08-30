package api

import (
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

// RegisterWithRouting registers all API handlers with the routing system
func RegisterWithRouting(registry *routing.HandlerRegistry) error {
	// Profile and preference handlers
	registry.Register("profile", HandleProfile)
	registry.Register("handleProfile", HandleProfile) // Both names for compatibility
	registry.Register("get_session_timeout", HandleGetSessionTimeout)
	registry.Register("set_session_timeout", HandleSetSessionTimeout)
	
	// Redirect handlers
	registry.Register("redirect_profile", HandleRedirectProfile)
	registry.Register("redirect_tickets", HandleRedirectTickets)
	registry.Register("redirect_tickets_new", HandleRedirectTicketsNew)
	registry.Register("redirect_queues", HandleRedirectQueues)
	registry.Register("redirect_settings", HandleRedirectSettings)
	
	// Core handlers
	registry.Register("redirect", HandleRedirect)
	registry.Register("template", HandleTemplate)
	registry.Register("login", HandleLoginPage)
	registry.Register("logout", HandleLogout)
	registry.Register("dashboard", HandleDashboard)
	
	// Auth API handlers
	registry.Register("auth_login", HandleAuthLogin)
	registry.Register("auth_logout", HandleAuthLogout)
	registry.Register("auth_check", HandleAuthCheck)
	
	// Admin handlers
	registry.Register("admin_dashboard", HandleAdminDashboard)
	registry.Register("admin_tickets", HandleAdminTickets)
	registry.Register("admin_users", HandleAdminUsers)
	registry.Register("admin_user_get", HandleAdminUserGet)
	registry.Register("admin_users_create", HandleAdminUserCreate)
	registry.Register("admin_users_update", HandleAdminUserUpdateDebug) // TEMPORARY: Use debug version
	registry.Register("admin_users_delete", HandleAdminUserDelete)
	registry.Register("admin_users_status", HandleAdminUsersStatus)
	registry.Register("admin_users_reset_password", HandleAdminUserResetPassword)
	registry.Register("admin_user_groups", HandleAdminUserGroups)
	registry.Register("admin_password_policy", HandlePasswordPolicy)
	registry.Register("admin_groups", HandleAdminGroups)
	registry.Register("admin_group_get", HandleGetGroup)
	registry.Register("admin_groups_create", HandleCreateGroup)
	registry.Register("admin_groups_update", HandleUpdateGroup)
	registry.Register("admin_groups_delete", HandleDeleteGroup)
	registry.Register("admin_groups_users", HandleGroupMembers)
	registry.Register("admin_groups_add_user", HandleAddUserToGroup)
	registry.Register("admin_groups_remove_user", HandleRemoveUserFromGroup)
	registry.Register("admin_queues", HandleAdminQueues)
	registry.Register("admin_priorities", HandleAdminPriorities)
	registry.Register("admin_permissions", HandleAdminPermissions)
	registry.Register("admin_states", HandleAdminStates)
	registry.Register("admin_types", HandleAdminTypes)
	registry.Register("admin_services", HandleAdminServices)
	registry.Register("admin_sla", HandleAdminSLA)
	registry.Register("admin_lookups", HandleAdminLookups)
	
	// Admin Customer User handlers
	registry.Register("admin_customer_users_list", HandleAdminCustomerUsersList)
	registry.Register("admin_customer_users_get", HandleAdminCustomerUsersGet)
	registry.Register("admin_customer_users_create", HandleAdminCustomerUsersCreate)
	registry.Register("admin_customer_users_update", HandleAdminCustomerUsersUpdate)
	registry.Register("admin_customer_users_delete", HandleAdminCustomerUsersDelete)
	registry.Register("admin_customer_users_tickets", HandleAdminCustomerUsersTickets)
	registry.Register("admin_customer_users_import_form", HandleAdminCustomerUsersImportForm)
	registry.Register("admin_customer_users_import", HandleAdminCustomerUsersImport)
	registry.Register("admin_customer_users_export", HandleAdminCustomerUsersExport)
	registry.Register("admin_customer_users_bulk_action", HandleAdminCustomerUsersBulkAction)
	
	// Customer handlers
	registry.Register("customer_dashboard", HandleCustomerDashboard)
	registry.Register("customer_tickets", HandleCustomerTickets)
	registry.Register("customer_new_ticket", HandleCustomerNewTicket)
	registry.Register("customer_create_ticket", HandleCustomerCreateTicket)
	registry.Register("customer_ticket_view", HandleCustomerTicketView)
	registry.Register("customer_ticket_reply", HandleCustomerTicketReply)
	registry.Register("customer_close_ticket", HandleCustomerCloseTicket)
	registry.Register("customer_profile", HandleCustomerProfile)
	registry.Register("customer_update_profile", HandleCustomerUpdateProfile)
	registry.Register("customer_password_form", HandleCustomerPasswordForm)
	registry.Register("customer_change_password", HandleCustomerChangePassword)
	registry.Register("customer_knowledge_base", HandleCustomerKnowledgeBase)
	registry.Register("customer_kb_search", HandleCustomerKBSearch)
	registry.Register("customer_kb_article", HandleCustomerKBArticle)
	registry.Register("customer_company_info", HandleCustomerCompanyInfo)
	registry.Register("customer_company_users", HandleCustomerCompanyUsers)
	
	// Agent handlers
	registry.Register("agent_dashboard", HandleAgentDashboard)
	registry.Register("agent_tickets", HandleAgentTickets)
	registry.Register("agent_ticket_view", HandleAgentTicketView)
	registry.Register("agent_ticket_reply", HandleAgentTicketReply)
	registry.Register("agent_ticket_note", HandleAgentTicketNote)
	registry.Register("agent_ticket_phone", HandleAgentTicketPhone)
	registry.Register("agent_ticket_status", HandleAgentTicketStatus)
	registry.Register("agent_ticket_assign", HandleAgentTicketAssign)
	registry.Register("agent_ticket_priority", HandleAgentTicketPriority)
	// NEWLY ADDED: Missing handlers that were causing 404 errors
	registry.Register("agent_ticket_queue", HandleAgentTicketQueue)
	registry.Register("agent_ticket_merge", HandleAgentTicketMerge)
	registry.Register("ticket_customer_users", HandleTicketCustomerUsers)
	registry.Register("article_attachment_download", HandleArticleAttachmentDownload)
	registry.Register("agent_queues", HandleAgentQueues)
	registry.Register("agent_queue_view", HandleAgentQueueView)
	registry.Register("agent_queue_lock", HandleAgentQueueLock)
	registry.Register("agent_queue_unlock", HandleAgentQueueUnlock)
	registry.Register("agent_customers", HandleAgentCustomers)
	registry.Register("agent_customer_view", HandleAgentCustomerView)
	registry.Register("agent_customer_tickets", HandleAgentCustomerTickets)
	registry.Register("agent_search", HandleAgentSearch)
	registry.Register("agent_search_results", HandleAgentSearchResults)
	
	// Dev handlers
	registry.Register("dev_dashboard", handleDevDashboard)
	registry.Register("claude_tickets", handleClaudeTickets)
	registry.Register("dev_database", handleDevDatabase)
	registry.Register("dev_logs", handleDevLogs)
	registry.Register("ticket_events", handleTicketEvents)
	registry.Register("dev_action", handleDevAction)
	
	// Redirect handlers for common routes
	registry.Register("redirect_tickets", handleRedirectTickets)
	registry.Register("redirect_tickets_new", handleRedirectTicketsNew)
	registry.Register("redirect_queues", handleRedirectQueues)
	registry.Register("redirect_profile", handleRedirectProfile)
	registry.Register("redirect_settings", handleRedirectSettings)
	
	// WebSocket handlers
	registry.Register("websocket_chat", HandleWebSocketChat)
	
	// API v1 handlers
	registry.Register("api_v1_tickets_list", HandleAPIv1TicketsList)
	registry.Register("api_v1_ticket_get", HandleAPIv1TicketGet)
	registry.Register("api_v1_ticket_create", HandleAPIv1TicketCreate)
	registry.Register("api_v1_ticket_update", HandleAPIv1TicketUpdate)
	registry.Register("api_v1_ticket_delete", HandleAPIv1TicketDelete)
	
	registry.Register("api_v1_user_me", HandleAPIv1UserMe)
	registry.Register("api_v1_users_list", HandleAPIv1UsersList)
	registry.Register("api_v1_user_get", HandleAPIv1UserGet)
	registry.Register("api_v1_user_create", HandleAPIv1UserCreate)
	registry.Register("api_v1_user_update", HandleAPIv1UserUpdate)
	registry.Register("api_v1_user_delete", HandleAPIv1UserDelete)
	
	registry.Register("api_v1_queues_list", HandleAPIv1QueuesList)
	registry.Register("api_v1_queue_get", HandleAPIv1QueueGet)
	registry.Register("api_v1_queue_create", HandleAPIv1QueueCreate)
	registry.Register("api_v1_queue_update", HandleAPIv1QueueUpdate)
	registry.Register("api_v1_queue_delete", HandleAPIv1QueueDelete)
	
	registry.Register("api_v1_priorities_list", HandleAPIv1PrioritiesList)
	registry.Register("api_v1_priority_get", HandleAPIv1PriorityGet)
	
	registry.Register("api_v1_search", HandleAPIv1Search)
	
	// Claude feedback handlers
	registry.Register("claude_feedback", HandleClaudeFeedback)
	registry.Register("claude_ticket_status", HandleClaudeTicketStatus)
	
	return nil
}