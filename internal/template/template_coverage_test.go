package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 100% TEMPLATE COVERAGE TESTS
// Every page template must be tested to ensure it renders without errors.
// =============================================================================

// AllPageTemplates lists every template that must have render coverage.
// When adding a new page template, add it here AND to the corresponding test.
var AllPageTemplates = map[string]bool{
	// Admin templates
	"pages/admin/attachment.pongo2":                true,
	"pages/admin/customer_companies.pongo2":        true,
	"pages/admin/customer_company_form.pongo2":     true,
	"pages/admin/customer_company_services.pongo2": true,
	"pages/admin/customer_company_tickets.pongo2":  true,
	"pages/admin/customer_company_users.pongo2":    true,
	"pages/admin/customer_portal_settings.pongo2":  true,
	"pages/admin/customer_user_services.pongo2":    true,
	"pages/admin/customer_users.pongo2":            true,
	"pages/admin/dashboard.pongo2":                 true,
	"pages/admin/dynamic_field_form.pongo2":        true,
	"pages/admin/dynamic_field_screens.pongo2":     true,
	"pages/admin/dynamic_fields.pongo2":            true,
	"pages/admin/dynamic_module.pongo2":            true,
	"pages/admin/dynamic_test.pongo2":              true,
	"pages/admin/email_identities.pongo2":          true,
	"pages/admin/email_queue.pongo2":               true,
	"pages/admin/group_form.pongo2":                true,
	"pages/admin/group_members.pongo2":             true,
	"pages/admin/group_permissions.pongo2":         true,
	"pages/admin/group_view.pongo2":                true,
	"pages/admin/groups.pongo2":                    true,
	"pages/admin/lookups.pongo2":                   true,
	"pages/admin/permissions.pongo2":               true,
	"pages/admin/permissions_debug.pongo2":         true,
	"pages/admin/permissions_simple.pongo2":        true,
	"pages/admin/permissions_standalone.pongo2":    true,
	"pages/admin/permissions_working.pongo2":       true,
	"pages/admin/priorities.pongo2":                true,
	"pages/admin/priority.pongo2":                  true,
	"pages/admin/queues.pongo2":                    true,
	"pages/admin/roadmap.pongo2":                   true,
	"pages/admin/role_permissions.pongo2":          true,
	"pages/admin/role_users.pongo2":                true,
	"pages/admin/roles.pongo2":                     true,
	"pages/admin/schema_discovery.pongo2":          true,
	"pages/admin/schema_monitoring.pongo2":         true,
	"pages/admin/services.pongo2":                  true,
	"pages/admin/sla.pongo2":                       true,
	"pages/admin/state.pongo2":                     true,
	"pages/admin/states.pongo2":                    true,
	"pages/admin/tickets.pongo2":                   true,
	"pages/admin/types.pongo2":                     true,
	"pages/admin/users.pongo2":                     true,

	// Agent templates
	"pages/agent/dashboard.pongo2":           true,
	"pages/agent/queues.pongo2":              true,
	"pages/agent/ticket_view.backup.pongo2":  true,
	"pages/agent/ticket_view.pongo2":         true,
	"pages/agent/tickets.pongo2":             true,

	// Customer templates
	"pages/customer/company_info.pongo2":   true,
	"pages/customer/company_users.pongo2":  true,
	"pages/customer/dashboard.pongo2":      true,
	"pages/customer/kb_article.pongo2":     true,
	"pages/customer/kb_search.pongo2":      true,
	"pages/customer/knowledge_base.pongo2": true,
	"pages/customer/login.pongo2":          true,
	"pages/customer/new_ticket.pongo2":     true,
	"pages/customer/password_form.pongo2":  true,
	"pages/customer/profile.pongo2":        true,
	"pages/customer/ticket_view.pongo2":    true,
	"pages/customer/tickets.pongo2":        true,

	// Dashboard templates
	"pages/dashboard.pongo2":          true,
	"pages/dashboard-simple.pongo2":   true,
	"pages/dashboard/realtime.pongo2": true,

	// Dev templates
	"pages/dev/claude_tickets.pongo2":    true,
	"pages/dev/dashboard.pongo2":         true,
	"pages/dev/dashboard_dynamic.pongo2": true,
	"pages/dev/database.pongo2":          true,
	"pages/dev/logs.pongo2":              true,

	// Queue templates
	"pages/queue_detail.pongo2":  true,
	"pages/queues.pongo2":        true,
	"pages/queues/detail.pongo2": true,
	"pages/queues/list.pongo2":   true,

	// Ticket templates
	"pages/ticket_detail.pongo2":  true,
	"pages/tickets.pongo2":        true,
	"pages/tickets/detail.pongo2": true,
	"pages/tickets/list.pongo2":   true,
	"pages/tickets/new.pongo2":    true,

	// Auth/Misc templates
	"pages/claude_chat_demo.pongo2":   true,
	"pages/error.pongo2":              true,
	"pages/login.pongo2":              true,
	"pages/profile.pongo2":            true,
	"pages/register.pongo2":           true,
	"pages/settings.pongo2":           true,
	"pages/under_construction.pongo2": true,
}

// =============================================================================
// CONTEXT BUILDERS
// =============================================================================

func adminContext() pongo2.Context {
	ctx := baseContext()
	ctx["User"] = map[string]interface{}{
		"ID":       1,
		"Username": "admin",
		"IsAdmin":  true,
	}
	return ctx
}

func agentContext() pongo2.Context {
	ctx := baseContext()
	ctx["User"] = map[string]interface{}{
		"ID":       2,
		"Username": "agent",
		"IsAdmin":  false,
	}
	return ctx
}

func emptySlice() []map[string]interface{} {
	return []map[string]interface{}{}
}

func sampleTicket() map[string]interface{} {
	return map[string]interface{}{
		"id":           123,
		"ID":           123,
		"tn":           "2025010112345678",
		"title":        "Test Ticket",
		"Title":        "Test Ticket",
		"queue":        "Support",
		"Queue":        "Support",
		"state":        "open",
		"State":        "open",
		"priority":     "normal",
		"Priority":     "normal",
		"CustomerUser": "customer@example.com",
		"Owner":        "agent",
		"Created":      time.Now(),
		"Changed":      time.Now(),
	}
}

func sampleGroup() map[string]interface{} {
	return map[string]interface{}{
		"ID":          1,
		"Name":        "Test Group",
		"Comment":     "Test group comment",
		"ValidID":     1,
		"MemberCount": 5,
	}
}

func sampleUser() map[string]interface{} {
	return map[string]interface{}{
		"ID":         1,
		"Login":      "testuser",
		"FirstName":  "Test",
		"LastName":   "User",
		"Email":      "test@example.com",
		"ValidID":    1,
		"IsAdmin":    false,
		"Created":    time.Now(),
		"Changed":    time.Now(),
	}
}

func sampleQueue() map[string]interface{} {
	return map[string]interface{}{
		"ID":          1,
		"Name":        "Support",
		"GroupID":     1,
		"ValidID":     1,
		"Comment":     "Main support queue",
		"TicketCount": 10,
	}
}

func sampleCompany() map[string]interface{} {
	return map[string]interface{}{
		"customer_id": "CUST001",
		"name":        "Test Company",
		"street":      "123 Test St",
		"city":        "Test City",
		"zip":         "12345",
		"country":     "USA",
		"url":         "https://example.com",
		"comments":    "Test company",
		"valid_id":    1,
	}
}

func sampleDynamicField() map[string]interface{} {
	return map[string]interface{}{
		"ID":        1,
		"Name":      "TestField",
		"Label":     "Test Field",
		"FieldType": "Text",
		"ObjectType": "Ticket",
		"Config":    map[string]interface{}{"DefaultValue": "", "MaxLength": 100},
		"ValidID":   1,
	}
}

func sampleArticle() map[string]interface{} {
	return map[string]interface{}{
		"ID":               1,
		"TicketID":         123,
		"ArticleType":      "note-internal",
		"SenderType":       "agent",
		"From":             "agent@example.com",
		"To":               "customer@example.com",
		"Subject":          "Test Article",
		"Body":             "Test body content",
		"ContentType":      "text/plain",
		"Created":          time.Now(),
		"IncomingTime":     time.Now(),
	}
}

func sampleService() map[string]interface{} {
	return map[string]interface{}{
		"ID":       1,
		"Name":     "Test Service",
		"Comment":  "Test service",
		"ValidID":  1,
	}
}

func sampleSLA() map[string]interface{} {
	return map[string]interface{}{
		"ID":               1,
		"Name":             "Test SLA",
		"Comment":          "Test SLA",
		"FirstResponseTime": 3600,
		"SolutionTime":     86400,
		"ValidID":          1,
	}
}

func sampleRole() map[string]interface{} {
	return map[string]interface{}{
		"ID":      1,
		"Name":    "Test Role",
		"Comment": "Test role",
		"ValidID": 1,
	}
}

func samplePriority() map[string]interface{} {
	return map[string]interface{}{
		"ID":      1,
		"Name":    "normal",
		"Color":   "#0066CC",
		"ValidID": 1,
	}
}

func sampleState() map[string]interface{} {
	return map[string]interface{}{
		"ID":        1,
		"Name":      "open",
		"TypeID":    1,
		"TypeName":  "open",
		"ValidID":   1,
	}
}

func sampleType() map[string]interface{} {
	return map[string]interface{}{
		"ID":      1,
		"Name":    "Unclassified",
		"ValidID": 1,
	}
}

func sampleKBArticle() map[string]interface{} {
	return map[string]interface{}{
		"ID":          1,
		"CategoryID":  1,
		"Title":       "Test KB Article",
		"Slug":        "test-kb-article",
		"Content":     "<p>Test content</p>",
		"ViewCount":   100,
		"Published":   true,
		"Created":     time.Now(),
	}
}

func sampleKBCategory() map[string]interface{} {
	return map[string]interface{}{
		"ID":           1,
		"Name":         "General",
		"Description":  "General articles",
		"ArticleCount": 5,
	}
}

func samplePermission() map[string]interface{} {
	return map[string]interface{}{
		"ID":     1,
		"Name":   "ticket_read",
		"Module": "ticket",
		"Action": "read",
	}
}

func sampleEmail() map[string]interface{} {
	return map[string]interface{}{
		"ID":              1,
		"Subject":         "Test Email",
		"Status":          "pending",
		"Recipient":       "test@example.com",
		"LastSMTPMessage": "",
		"CreateTime":      time.Now(),
	}
}

func sampleEmailIdentity() map[string]interface{} {
	return map[string]interface{}{
		"ID":          1,
		"Email":       "support@example.com",
		"DisplayName": "Support Team",
		"ValidID":     1,
	}
}

func sampleAttachment() map[string]interface{} {
	return map[string]interface{}{
		"ID":          1,
		"ArticleID":   1,
		"Filename":    "test.pdf",
		"ContentType": "application/pdf",
		"ContentSize": 1024,
	}
}

// =============================================================================
// ADMIN TEMPLATE TESTS
// =============================================================================

func TestAllAdminTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "admin/attachment",
			template: "pages/admin/attachment.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Attachment"] = sampleAttachment()
				return ctx
			}(),
		},
		{
			name:     "admin/customer_companies",
			template: "pages/admin/customer_companies.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Companies"] = []map[string]interface{}{sampleCompany()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/customer_company_form",
			template: "pages/admin/customer_company_form.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["IsNew"] = true
				ctx["Company"] = sampleCompany()
				return ctx
			}(),
		},
		{
			name:     "admin/customer_company_services",
			template: "pages/admin/customer_company_services.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Company"] = sampleCompany()
				ctx["Services"] = []map[string]interface{}{sampleService()}
				ctx["AllServices"] = []map[string]interface{}{sampleService()}
				return ctx
			}(),
		},
		{
			name:     "admin/customer_company_tickets",
			template: "pages/admin/customer_company_tickets.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Company"] = sampleCompany()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "admin/customer_company_users",
			template: "pages/admin/customer_company_users.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Company"] = sampleCompany()
				ctx["Users"] = []map[string]interface{}{sampleUser()}
				return ctx
			}(),
		},
		{
			name:     "admin/customer_portal_settings",
			template: "pages/admin/customer_portal_settings.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Settings"] = map[string]interface{}{
					"allow_registration": true,
					"require_approval":   false,
				}
				return ctx
			}(),
		},
		{
			name:     "admin/customer_user_services",
			template: "pages/admin/customer_user_services.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["CustomerUser"] = sampleUser()
				ctx["Services"] = []map[string]interface{}{sampleService()}
				ctx["AllServices"] = []map[string]interface{}{sampleService()}
				return ctx
			}(),
		},
		{
			name:     "admin/customer_users",
			template: "pages/admin/customer_users.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Users"] = []map[string]interface{}{sampleUser()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/dashboard",
			template: "pages/admin/dashboard.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Stats"] = map[string]interface{}{
					"TotalUsers":    10,
					"TotalTickets":  100,
					"TotalQueues":   5,
					"ActiveTickets": 50,
				}
				return ctx
			}(),
		},
		{
			name:     "admin/dynamic_field_form",
			template: "pages/admin/dynamic_field_form.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["IsNew"] = true
				ctx["Field"] = sampleDynamicField()
				ctx["FieldTypes"] = []string{"Text", "Textarea", "Dropdown", "Checkbox"}
				ctx["ValidOptions"] = []map[string]interface{}{{"ID": 1, "Name": "valid"}, {"ID": 2, "Name": "invalid"}}
				return ctx
			}(),
		},
		{
			name:     "admin/dynamic_field_screens",
			template: "pages/admin/dynamic_field_screens.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Field"] = sampleDynamicField()
				ctx["Screens"] = []map[string]interface{}{
					{"Name": "AgentTicketPhone", "Active": true},
					{"Name": "AgentTicketEmail", "Active": false},
				}
				return ctx
			}(),
		},
		{
			name:     "admin/dynamic_fields",
			template: "pages/admin/dynamic_fields.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Fields"] = []map[string]interface{}{sampleDynamicField()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/dynamic_module",
			template: "pages/admin/dynamic_module.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Module"] = map[string]interface{}{
					"Name":   "TestModule",
					"Active": true,
				}
				return ctx
			}(),
		},
		{
			name:     "admin/dynamic_test",
			template: "pages/admin/dynamic_test.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "admin/email_identities",
			template: "pages/admin/email_identities.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Identities"] = []map[string]interface{}{sampleEmailIdentity()}
				return ctx
			}(),
		},
		{
			name:     "admin/email_queue",
			template: "pages/admin/email_queue.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Emails"] = []map[string]interface{}{sampleEmail()}
				ctx["Stats"] = map[string]interface{}{"pending": 1, "failed": 0}
				return ctx
			}(),
		},
		{
			name:     "admin/group_form",
			template: "pages/admin/group_form.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["IsNew"] = true
				ctx["Group"] = sampleGroup()
				ctx["ValidOptions"] = []map[string]interface{}{{"ID": 1, "Name": "valid"}}
				return ctx
			}(),
		},
		{
			name:     "admin/group_members",
			template: "pages/admin/group_members.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Group"] = sampleGroup()
				ctx["Members"] = []map[string]interface{}{sampleUser()}
				ctx["AllUsers"] = []map[string]interface{}{sampleUser()}
				ctx["PermissionTypes"] = []string{"ro", "rw", "move_into", "create", "note", "owner", "priority"}
				return ctx
			}(),
		},
		{
			name:     "admin/group_permissions",
			template: "pages/admin/group_permissions.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Group"] = sampleGroup()
				ctx["Permissions"] = []map[string]interface{}{samplePermission()}
				return ctx
			}(),
		},
		{
			name:     "admin/group_view",
			template: "pages/admin/group_view.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Group"] = sampleGroup()
				ctx["Members"] = []map[string]interface{}{sampleUser()}
				return ctx
			}(),
		},
		{
			name:     "admin/groups",
			template: "pages/admin/groups.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Groups"] = []map[string]interface{}{sampleGroup()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/lookups",
			template: "pages/admin/lookups.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["LookupType"] = "priorities"
				ctx["Items"] = []map[string]interface{}{samplePriority()}
				return ctx
			}(),
		},
		{
			name:     "admin/permissions",
			template: "pages/admin/permissions.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Permissions"] = []map[string]interface{}{samplePermission()}
				return ctx
			}(),
		},
		{
			name:     "admin/permissions_debug",
			template: "pages/admin/permissions_debug.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Debug"] = map[string]interface{}{}
				return ctx
			}(),
		},
		{
			name:     "admin/permissions_simple",
			template: "pages/admin/permissions_simple.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Permissions"] = []map[string]interface{}{samplePermission()}
				return ctx
			}(),
		},
		{
			name:     "admin/permissions_standalone",
			template: "pages/admin/permissions_standalone.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Permissions"] = []map[string]interface{}{samplePermission()}
				return ctx
			}(),
		},
		{
			name:     "admin/permissions_working",
			template: "pages/admin/permissions_working.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Permissions"] = []map[string]interface{}{samplePermission()}
				return ctx
			}(),
		},
		{
			name:     "admin/priorities",
			template: "pages/admin/priorities.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Priorities"] = []map[string]interface{}{samplePriority()}
				return ctx
			}(),
		},
		{
			name:     "admin/priority",
			template: "pages/admin/priority.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["IsNew"] = true
				ctx["Priority"] = samplePriority()
				ctx["ValidOptions"] = []map[string]interface{}{{"ID": 1, "Name": "valid"}}
				return ctx
			}(),
		},
		{
			name:     "admin/queues",
			template: "pages/admin/queues.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/roadmap",
			template: "pages/admin/roadmap.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "admin/role_permissions",
			template: "pages/admin/role_permissions.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Role"] = sampleRole()
				ctx["Permissions"] = []map[string]interface{}{samplePermission()}
				ctx["AllPermissions"] = []map[string]interface{}{samplePermission()}
				return ctx
			}(),
		},
		{
			name:     "admin/role_users",
			template: "pages/admin/role_users.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Role"] = sampleRole()
				ctx["Users"] = []map[string]interface{}{sampleUser()}
				ctx["AllUsers"] = []map[string]interface{}{sampleUser()}
				return ctx
			}(),
		},
		{
			name:     "admin/roles",
			template: "pages/admin/roles.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Roles"] = []map[string]interface{}{sampleRole()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/schema_discovery",
			template: "pages/admin/schema_discovery.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "admin/schema_monitoring",
			template: "pages/admin/schema_monitoring.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "admin/services",
			template: "pages/admin/services.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Services"] = []map[string]interface{}{sampleService()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/sla",
			template: "pages/admin/sla.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["SLAs"] = []map[string]interface{}{sampleSLA()}
				ctx["Search"] = ""
				ctx["Status"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/state",
			template: "pages/admin/state.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["IsNew"] = true
				ctx["State"] = sampleState()
				ctx["StateTypes"] = []map[string]interface{}{{"ID": 1, "Name": "open"}}
				ctx["ValidOptions"] = []map[string]interface{}{{"ID": 1, "Name": "valid"}}
				return ctx
			}(),
		},
		{
			name:     "admin/states",
			template: "pages/admin/states.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["States"] = []map[string]interface{}{sampleState()}
				return ctx
			}(),
		},
		{
			name:     "admin/tickets",
			template: "pages/admin/tickets.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
		{
			name:     "admin/types",
			template: "pages/admin/types.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Types"] = []map[string]interface{}{sampleType()}
				return ctx
			}(),
		},
		{
			name:     "admin/users",
			template: "pages/admin/users.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Users"] = []map[string]interface{}{sampleUser()}
				ctx["Search"] = ""
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// AGENT TEMPLATE TESTS
// =============================================================================

func TestAllAgentTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "agent/dashboard",
			template: "pages/agent/dashboard.pongo2",
			ctx: func() pongo2.Context {
				ctx := agentContext()
				ctx["Stats"] = map[string]interface{}{
					"OpenTickets":    10,
					"MyTickets":      5,
					"PendingTickets": 3,
				}
				ctx["RecentTickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "agent/queues",
			template: "pages/agent/queues.pongo2",
			ctx: func() pongo2.Context {
				ctx := agentContext()
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				return ctx
			}(),
		},
		{
			name:     "agent/ticket_view.backup",
			template: "pages/agent/ticket_view.backup.pongo2",
			ctx: func() pongo2.Context {
				ctx := agentContext()
				ctx["Ticket"] = sampleTicket()
				ctx["Articles"] = []map[string]interface{}{sampleArticle()}
				ctx["DynamicFields"] = emptySlice()
				ctx["ArticleTypes"] = []map[string]interface{}{{"ID": 1, "Name": "note-internal"}}
				ctx["CanEdit"] = true
				return ctx
			}(),
		},
		{
			name:     "agent/ticket_view",
			template: "pages/agent/ticket_view.pongo2",
			ctx: func() pongo2.Context {
				ctx := agentContext()
				ctx["Ticket"] = sampleTicket()
				ctx["TicketID"] = 123
				ctx["Articles"] = []map[string]interface{}{sampleArticle()}
				ctx["DynamicFields"] = emptySlice()
				ctx["ArticleTypes"] = []map[string]interface{}{{"ID": 1, "Name": "note-internal"}}
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				ctx["States"] = []map[string]interface{}{sampleState()}
				ctx["Priorities"] = []map[string]interface{}{samplePriority()}
				ctx["CanEdit"] = true
				return ctx
			}(),
		},
		{
			name:     "agent/tickets",
			template: "pages/agent/tickets.pongo2",
			ctx: func() pongo2.Context {
				ctx := agentContext()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				ctx["States"] = []map[string]interface{}{sampleState()}
				ctx["Priorities"] = []map[string]interface{}{samplePriority()}
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// CUSTOMER TEMPLATE TESTS
// =============================================================================

func TestAllCustomerTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "customer/company_info",
			template: "pages/customer/company_info.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Company"] = sampleCompany()
				return ctx
			}(),
		},
		{
			name:     "customer/company_users",
			template: "pages/customer/company_users.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Company"] = sampleCompany()
				ctx["Users"] = []map[string]interface{}{sampleUser()}
				return ctx
			}(),
		},
		{
			name:     "customer/dashboard",
			template: "pages/customer/dashboard.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Stats"] = map[string]interface{}{
					"OpenTickets":   2,
					"ClosedTickets": 5,
				}
				ctx["RecentTickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "customer/kb_article",
			template: "pages/customer/kb_article.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Article"] = sampleKBArticle()
				ctx["RelatedArticles"] = emptySlice()
				return ctx
			}(),
		},
		{
			name:     "customer/kb_search",
			template: "pages/customer/kb_search.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Query"] = ""
				ctx["Results"] = emptySlice()
				return ctx
			}(),
		},
		{
			name:     "customer/knowledge_base",
			template: "pages/customer/knowledge_base.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Categories"] = []map[string]interface{}{sampleKBCategory()}
				ctx["FeaturedArticles"] = []map[string]interface{}{sampleKBArticle()}
				return ctx
			}(),
		},
		{
			name:     "customer/login",
			template: "pages/customer/login.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Error"] = ""
				return ctx
			}(),
		},
		{
			name:     "customer/new_ticket",
			template: "pages/customer/new_ticket.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				ctx["Priorities"] = []map[string]interface{}{samplePriority()}
				ctx["Types"] = []map[string]interface{}{sampleType()}
				ctx["DynamicFields"] = emptySlice()
				return ctx
			}(),
		},
		{
			name:     "customer/password_form",
			template: "pages/customer/password_form.pongo2",
			ctx:      customerContext(),
		},
		{
			name:     "customer/profile",
			template: "pages/customer/profile.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Profile"] = sampleUser()
				return ctx
			}(),
		},
		{
			name:     "customer/ticket_view",
			template: "pages/customer/ticket_view.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Ticket"] = sampleTicket()
				ctx["Articles"] = []map[string]interface{}{sampleArticle()}
				return ctx
			}(),
		},
		{
			name:     "customer/tickets",
			template: "pages/customer/tickets.pongo2",
			ctx: func() pongo2.Context {
				ctx := customerContext()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// DASHBOARD TEMPLATE TESTS
// =============================================================================

func TestAllDashboardTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "dashboard",
			template: "pages/dashboard.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Stats"] = map[string]interface{}{
					"OpenTickets":    10,
					"PendingTickets": 5,
					"ClosedToday":    3,
				}
				ctx["RecentTickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "dashboard-simple",
			template: "pages/dashboard-simple.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Stats"] = map[string]interface{}{}
				return ctx
			}(),
		},
		{
			name:     "dashboard/realtime",
			template: "pages/dashboard/realtime.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Stats"] = map[string]interface{}{}
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// DEV TEMPLATE TESTS
// =============================================================================

func TestAllDevTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "dev/claude_tickets",
			template: "pages/dev/claude_tickets.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "dev/dashboard",
			template: "pages/dev/dashboard.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "dev/dashboard_dynamic",
			template: "pages/dev/dashboard_dynamic.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "dev/database",
			template: "pages/dev/database.pongo2",
			ctx:      adminContext(),
		},
		{
			name:     "dev/logs",
			template: "pages/dev/logs.pongo2",
			ctx: func() pongo2.Context {
				ctx := adminContext()
				ctx["Logs"] = emptySlice()
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// QUEUE TEMPLATE TESTS
// =============================================================================

func TestAllQueueTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "queue_detail",
			template: "pages/queue_detail.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Queue"] = sampleQueue()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "queues",
			template: "pages/queues.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				return ctx
			}(),
		},
		{
			name:     "queues/detail",
			template: "pages/queues/detail.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Queue"] = sampleQueue()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "queues/list",
			template: "pages/queues/list.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// TICKET TEMPLATE TESTS
// =============================================================================

func TestAllTicketTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "ticket_detail",
			template: "pages/ticket_detail.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Ticket"] = sampleTicket()
				ctx["Articles"] = []map[string]interface{}{sampleArticle()}
				ctx["Attachments"] = emptySlice()
				return ctx
			}(),
		},
		{
			name:     "tickets",
			template: "pages/tickets.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "tickets/detail",
			template: "pages/tickets/detail.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Ticket"] = sampleTicket()
				ctx["TicketID"] = 123
				ctx["Articles"] = []map[string]interface{}{sampleArticle()}
				ctx["DynamicFields"] = emptySlice()
				ctx["ArticleTypes"] = []map[string]interface{}{{"ID": 1, "Name": "note-internal"}}
				ctx["Queues"] = []map[string]interface{}{sampleQueue()}
				ctx["States"] = []map[string]interface{}{sampleState()}
				ctx["Priorities"] = []map[string]interface{}{samplePriority()}
				ctx["CanEdit"] = true
				return ctx
			}(),
		},
		{
			name:     "tickets/list",
			template: "pages/tickets/list.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Tickets"] = []map[string]interface{}{sampleTicket()}
				return ctx
			}(),
		},
		{
			name:     "tickets/new",
			template: "pages/tickets/new.pongo2",
			ctx: func() pongo2.Context {
				ctx := ticketContext()
				ctx["Types"] = []map[string]interface{}{sampleType()}
				ctx["Services"] = emptySlice()
				ctx["DynamicFields"] = emptySlice()
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// MISC TEMPLATE TESTS
// =============================================================================

func TestAllMiscTemplatesRender(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	tests := []struct {
		name     string
		template string
		ctx      pongo2.Context
	}{
		{
			name:     "claude_chat_demo",
			template: "pages/claude_chat_demo.pongo2",
			ctx:      baseContext(),
		},
		{
			name:     "error",
			template: "pages/error.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Error"] = "Something went wrong"
				ctx["StatusCode"] = 500
				return ctx
			}(),
		},
		{
			name:     "login",
			template: "pages/login.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Error"] = ""
				return ctx
			}(),
		},
		{
			name:     "profile",
			template: "pages/profile.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Profile"] = sampleUser()
				return ctx
			}(),
		},
		{
			name:     "register",
			template: "pages/register.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Error"] = ""
				return ctx
			}(),
		},
		{
			name:     "settings",
			template: "pages/settings.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Settings"] = map[string]interface{}{}
				return ctx
			}(),
		},
		{
			name:     "under_construction",
			template: "pages/under_construction.pongo2",
			ctx:      baseContext(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tt.template, tt.ctx)
			require.NoError(t, err, "Template %s should render without error", tt.template)
			require.NotEmpty(t, html, "Template %s should produce output", tt.template)
		})
	}
}

// =============================================================================
// 100% COVERAGE ENFORCEMENT TEST
// =============================================================================

// TestAllPageTemplatesHaveCoverage ensures every page template is tested
func TestAllPageTemplatesHaveCoverage(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	// Walk templates directory and collect all page templates
	var allTemplates []string
	err := filepath.Walk(helper.TemplateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".pongo2") {
			return nil
		}

		relPath, _ := filepath.Rel(helper.TemplateDir, path)

		// Only check page templates
		if strings.HasPrefix(relPath, "pages/") {
			allTemplates = append(allTemplates, relPath)
		}

		return nil
	})
	require.NoError(t, err)

	// Check each template is in our coverage map
	var missing []string
	for _, tmpl := range allTemplates {
		if !AllPageTemplates[tmpl] {
			missing = append(missing, tmpl)
		}
	}

	if len(missing) > 0 {
		t.Errorf("The following page templates are missing from AllPageTemplates map and need render tests:\n%s",
			strings.Join(missing, "\n"))
	}

	// Also check for stale entries in the map (templates that no longer exist)
	var stale []string
	for tmpl := range AllPageTemplates {
		fullPath := filepath.Join(helper.TemplateDir, tmpl)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			stale = append(stale, tmpl)
		}
	}

	if len(stale) > 0 {
		t.Errorf("The following entries in AllPageTemplates map reference non-existent templates:\n%s",
			strings.Join(stale, "\n"))
	}
}
