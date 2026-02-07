package mcp

// ToolRegistry contains all available MCP tools for GoatFlow.
var ToolRegistry = []Tool{
	{
		Name:        "list_tickets",
		Description: "List tickets with optional filters. Returns ticket ID, number, title, state, queue, priority, and owner.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"queue_id": {
					Type:        "integer",
					Description: "Filter by queue ID",
				},
				"state_id": {
					Type:        "integer",
					Description: "Filter by state ID (1=new, 2=open, 3=pending, 4=closed)",
				},
				"owner_id": {
					Type:        "integer",
					Description: "Filter by owner user ID",
				},
				"customer_id": {
					Type:        "string",
					Description: "Filter by customer ID",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of tickets to return (default 20, max 100)",
					Default:     20,
				},
				"offset": {
					Type:        "integer",
					Description: "Offset for pagination",
					Default:     0,
				},
			},
		},
	},
	{
		Name:        "get_ticket",
		Description: "Get detailed information about a specific ticket including articles/messages.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"ticket_id": {
					Type:        "integer",
					Description: "The ticket ID to retrieve",
				},
				"ticket_number": {
					Type:        "string",
					Description: "The ticket number (TN) to retrieve",
				},
				"include_articles": {
					Type:        "boolean",
					Description: "Include ticket articles/messages (default true)",
					Default:     true,
				},
			},
		},
	},
	{
		Name:        "create_ticket",
		Description: "Create a new ticket in the system.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"title": {
					Type:        "string",
					Description: "Ticket title/subject",
				},
				"queue_id": {
					Type:        "integer",
					Description: "Queue ID to create the ticket in",
				},
				"priority_id": {
					Type:        "integer",
					Description: "Priority ID (1=low, 2=normal, 3=high, 4=urgent)",
				},
				"state_id": {
					Type:        "integer",
					Description: "Initial state ID (default: 1=new)",
				},
				"customer_user": {
					Type:        "string",
					Description: "Customer user login or email",
				},
				"body": {
					Type:        "string",
					Description: "Initial article/message body",
				},
			},
			Required: []string{"title", "queue_id", "body"},
		},
	},
	{
		Name:        "update_ticket",
		Description: "Update ticket attributes (state, priority, queue, owner, etc.).",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"ticket_id": {
					Type:        "integer",
					Description: "The ticket ID to update",
				},
				"title": {
					Type:        "string",
					Description: "New ticket title",
				},
				"state_id": {
					Type:        "integer",
					Description: "New state ID",
				},
				"priority_id": {
					Type:        "integer",
					Description: "New priority ID",
				},
				"queue_id": {
					Type:        "integer",
					Description: "Move to new queue ID",
				},
				"owner_id": {
					Type:        "integer",
					Description: "Assign to new owner user ID",
				},
			},
			Required: []string{"ticket_id"},
		},
	},
	{
		Name:        "add_article",
		Description: "Add a note/article to an existing ticket.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"ticket_id": {
					Type:        "integer",
					Description: "The ticket ID to add the article to",
				},
				"subject": {
					Type:        "string",
					Description: "Article subject (optional, defaults to ticket title)",
				},
				"body": {
					Type:        "string",
					Description: "Article body content",
				},
				"article_type": {
					Type:        "string",
					Description: "Article type",
					Enum:        []string{"note-internal", "note-external", "email-external"},
					Default:     "note-internal",
				},
			},
			Required: []string{"ticket_id", "body"},
		},
	},
	{
		Name:        "list_queues",
		Description: "List all queues the authenticated user has access to.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "list_users",
		Description: "List agent users in the system.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"valid": {
					Type:        "boolean",
					Description: "Filter by valid/active users only (default true)",
					Default:     true,
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of users to return",
					Default:     50,
				},
			},
		},
	},
	{
		Name:        "search_tickets",
		Description: "Full-text search across tickets.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "Search query string",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum results (default 20)",
					Default:     20,
				},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "get_statistics",
		Description: "Get dashboard statistics (ticket counts by state, queue, etc.).",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "execute_sql",
		Description: "Execute a read-only SQL query (SELECT only). Requires admin group membership. For debugging and development.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "SQL SELECT query to execute",
				},
				"args": {
					Type:        "array",
					Description: "Query arguments for ? placeholders",
				},
			},
			Required: []string{"query"},
		},
	},
}
