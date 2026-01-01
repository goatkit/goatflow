// Package filters provides email filter annotations and processing rules.
package filters

const (
	AnnotationQueueIDOverride      = "postmaster.queue_id_override"
	AnnotationQueueNameOverride    = "postmaster.queue_name_override"
	AnnotationPriorityIDOverride   = "postmaster.priority_id_override"
	AnnotationTitleOverride        = "postmaster.title_override"
	AnnotationCustomerIDOverride   = "postmaster.customer_id_override"
	AnnotationCustomerUserOverride = "postmaster.customer_user_override"
	AnnotationIgnoreMessage        = "postmaster.ignore_message"
	AnnotationFollowUpTicketNumber = "postmaster.follow_up_ticket_number"
	AnnotationTrustedHeaderPrefix  = "postmaster.trusted_header."
)
