package api

import (
	"context"
	"database/sql"
	"log"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/services/acl"
)

// ACLHelper provides ACL filtering for ticket-related data.
type ACLHelper struct {
	service *acl.Service
}

// NewACLHelper creates a new ACL helper with the given database connection.
func NewACLHelper(db *sql.DB) *ACLHelper {
	if db == nil {
		return nil
	}
	return &ACLHelper{
		service: acl.NewService(db),
	}
}

// BuildACLContext creates an ACL context from ticket and user information.
func (h *ACLHelper) BuildACLContext(ticket *models.Ticket, userID int, action string) *models.ACLContext {
	if ticket == nil {
		return &models.ACLContext{
			UserID: userID,
			Action: action,
		}
	}

	ctx := &models.ACLContext{
		UserID:     userID,
		TicketID:   ticket.ID,
		Ticket:     ticket,
		QueueID:    ticket.QueueID,
		StateID:    ticket.TicketStateID,
		PriorityID: ticket.TicketPriorityID,
		Action:     action,
	}

	if ticket.TypeID != nil {
		ctx.TypeID = *ticket.TypeID
	}
	if ticket.ServiceID != nil {
		ctx.ServiceID = *ticket.ServiceID
	}
	if ticket.SLAID != nil {
		ctx.SLAID = *ticket.SLAID
	}
	if ticket.UserID != nil {
		ctx.OwnerID = *ticket.UserID
	}
	ctx.LockID = ticket.TicketLockID
	if ticket.CustomerID != nil {
		ctx.CustomerID = *ticket.CustomerID
	}
	if ticket.CustomerUserID != nil {
		ctx.CustomerUserID = *ticket.CustomerUserID
	}

	return ctx
}

// FilterStates filters available ticket states based on ACLs.
// The states parameter is a slice of maps with "id" and "name" keys.
func (h *ACLHelper) FilterStates(ctx context.Context, aclCtx *models.ACLContext, states []map[string]interface{}) []map[string]interface{} {
	if h == nil || h.service == nil || len(states) == 0 {
		return states
	}

	// Build options map for ACL service
	options := make(map[int]string)
	for _, state := range states {
		if id, ok := state["id"].(int); ok {
			if name, ok := state["name"].(string); ok {
				options[id] = name
			}
		}
	}

	filtered, err := h.service.FilterOptions(ctx, aclCtx, "Ticket", "State", options)
	if err != nil {
		log.Printf("ACL: error filtering states: %v", err)
		return states
	}

	// Filter the original states slice to maintain full state data
	var result []map[string]interface{}
	for _, state := range states {
		if id, ok := state["id"].(int); ok {
			if _, allowed := filtered[id]; allowed {
				result = append(result, state)
			}
		}
	}

	return result
}

// FilterQueues filters available queues based on ACLs.
func (h *ACLHelper) FilterQueues(ctx context.Context, aclCtx *models.ACLContext, queues []map[string]interface{}) []map[string]interface{} {
	if h == nil || h.service == nil || len(queues) == 0 {
		return queues
	}

	options := make(map[int]string)
	for _, queue := range queues {
		if id, ok := queue["id"].(int); ok {
			if name, ok := queue["name"].(string); ok {
				options[id] = name
			}
		}
	}

	filtered, err := h.service.FilterOptions(ctx, aclCtx, "Ticket", "Queue", options)
	if err != nil {
		log.Printf("ACL: error filtering queues: %v", err)
		return queues
	}

	var result []map[string]interface{}
	for _, queue := range queues {
		if id, ok := queue["id"].(int); ok {
			if _, allowed := filtered[id]; allowed {
				result = append(result, queue)
			}
		}
	}

	return result
}

// FilterPriorities filters available priorities based on ACLs.
func (h *ACLHelper) FilterPriorities(ctx context.Context, aclCtx *models.ACLContext, priorities []map[string]interface{}) []map[string]interface{} {
	if h == nil || h.service == nil || len(priorities) == 0 {
		return priorities
	}

	options := make(map[int]string)
	for _, priority := range priorities {
		if id, ok := priority["id"].(int); ok {
			if name, ok := priority["name"].(string); ok {
				options[id] = name
			}
		}
	}

	filtered, err := h.service.FilterOptions(ctx, aclCtx, "Ticket", "Priority", options)
	if err != nil {
		log.Printf("ACL: error filtering priorities: %v", err)
		return priorities
	}

	var result []map[string]interface{}
	for _, priority := range priorities {
		if id, ok := priority["id"].(int); ok {
			if _, allowed := filtered[id]; allowed {
				result = append(result, priority)
			}
		}
	}

	return result
}

// FilterTypes filters available ticket types based on ACLs.
func (h *ACLHelper) FilterTypes(ctx context.Context, aclCtx *models.ACLContext, types []map[string]interface{}) []map[string]interface{} {
	if h == nil || h.service == nil || len(types) == 0 {
		return types
	}

	options := make(map[int]string)
	for _, t := range types {
		if id, ok := t["id"].(int); ok {
			if name, ok := t["name"].(string); ok {
				options[id] = name
			}
		}
	}

	filtered, err := h.service.FilterOptions(ctx, aclCtx, "Ticket", "Type", options)
	if err != nil {
		log.Printf("ACL: error filtering types: %v", err)
		return types
	}

	var result []map[string]interface{}
	for _, t := range types {
		if id, ok := t["id"].(int); ok {
			if _, allowed := filtered[id]; allowed {
				result = append(result, t)
			}
		}
	}

	return result
}

// FilterServices filters available services based on ACLs.
func (h *ACLHelper) FilterServices(ctx context.Context, aclCtx *models.ACLContext, services []map[string]interface{}) []map[string]interface{} {
	if h == nil || h.service == nil || len(services) == 0 {
		return services
	}

	options := make(map[int]string)
	for _, svc := range services {
		if id, ok := svc["id"].(int); ok {
			if name, ok := svc["name"].(string); ok {
				options[id] = name
			}
		}
	}

	filtered, err := h.service.FilterOptions(ctx, aclCtx, "Ticket", "Service", options)
	if err != nil {
		log.Printf("ACL: error filtering services: %v", err)
		return services
	}

	var result []map[string]interface{}
	for _, svc := range services {
		if id, ok := svc["id"].(int); ok {
			if _, allowed := filtered[id]; allowed {
				result = append(result, svc)
			}
		}
	}

	return result
}

// FilterSLAs filters available SLAs based on ACLs.
func (h *ACLHelper) FilterSLAs(ctx context.Context, aclCtx *models.ACLContext, slas []map[string]interface{}) []map[string]interface{} {
	if h == nil || h.service == nil || len(slas) == 0 {
		return slas
	}

	options := make(map[int]string)
	for _, sla := range slas {
		if id, ok := sla["id"].(int); ok {
			if name, ok := sla["name"].(string); ok {
				options[id] = name
			}
		}
	}

	filtered, err := h.service.FilterOptions(ctx, aclCtx, "Ticket", "SLA", options)
	if err != nil {
		log.Printf("ACL: error filtering SLAs: %v", err)
		return slas
	}

	var result []map[string]interface{}
	for _, sla := range slas {
		if id, ok := sla["id"].(int); ok {
			if _, allowed := filtered[id]; allowed {
				result = append(result, sla)
			}
		}
	}

	return result
}

// FilterActions filters available actions (buttons) based on ACLs.
func (h *ACLHelper) FilterActions(ctx context.Context, aclCtx *models.ACLContext, actions []string) []string {
	if h == nil || h.service == nil || len(actions) == 0 {
		return actions
	}

	filtered, err := h.service.FilterActions(ctx, aclCtx, actions)
	if err != nil {
		log.Printf("ACL: error filtering actions: %v", err)
		return actions
	}

	return filtered
}

// RefreshCache refreshes the cached ACLs.
func (h *ACLHelper) RefreshCache(ctx context.Context) error {
	if h == nil || h.service == nil {
		return nil
	}
	return h.service.RefreshCache(ctx)
}
