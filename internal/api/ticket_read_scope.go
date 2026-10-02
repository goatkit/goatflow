package api

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// ticketReadScope says which tickets a caller may list or count: a customer
// only their own (ticket.customer_user_id = login), an agent only those in the
// queues they can read (admins: all).
type ticketReadScope struct {
	customer      bool
	customerLogin string
	queues        statsScope
}

// filter returns a SQL condition (no leading AND) restricting the ticket table
// aliased as alias to the scope. An agent with no readable queue matches nothing.
func (s ticketReadScope) filter(alias string) (string, []interface{}) {
	if s.customer {
		return alias + ".customer_user_id = ?", []interface{}{s.customerLogin}
	}
	return s.queues.filter(alias + ".queue_id")
}

// canReadQueue reports whether an agent scope includes queueID (customers: false).
func (s ticketReadScope) canReadQueue(queueID int) bool {
	if s.customer {
		return false
	}
	if s.queues.all {
		return true
	}
	for _, id := range s.queues.queueIDs {
		if id == queueID {
			return true
		}
	}
	return false
}

// listTickets runs repo.List restricted to an agent scope. The repository
// treats an empty AccessibleQueueIDs as "no restriction", so an agent who can
// read no queue gets an empty list without querying.
func (s ticketReadScope) listTickets(repo *repository.TicketRepository, req *models.TicketListRequest) ([]models.Ticket, error) {
	if !s.queues.all {
		if len(s.queues.queueIDs) == 0 {
			return []models.Ticket{}, nil
		}
		req.AccessibleQueueIDs = make([]uint, len(s.queues.queueIDs))
		for i, id := range s.queues.queueIDs {
			req.AccessibleQueueIDs[i] = uint(id)
		}
	}
	resp, err := repo.List(req)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Tickets == nil {
		return []models.Ticket{}, nil
	}
	return resp.Tickets, nil
}

// resolveTicketReadScope resolves the caller's ticket read scope. When
// allowCustomers is false, customers get 403. On failure the error response is
// written and ok is false.
func resolveTicketReadScope(c *gin.Context, db *sql.DB, allowCustomers bool) (ticketReadScope, bool) {
	if isCustomerRequest(c) {
		if !allowCustomers {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Not available to customers"})
			return ticketReadScope{}, false
		}
		login := customerLoginFromCtx(c, db)
		if login == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Customer identity could not be resolved"})
			return ticketReadScope{}, false
		}
		return ticketReadScope{customer: true, customerLogin: login}, true
	}

	userID := extractUserIDForRBAC(c)
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return ticketReadScope{}, false
	}
	queues, err := resolveStatsScope(c, db, userID)
	if err != nil {
		log.Printf("resolving queue access for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check permissions"})
		return ticketReadScope{}, false
	}
	return ticketReadScope{queues: queues}, true
}

// isCustomerRequest reports whether the authenticated principal is a customer
// (customer JWT, customer session or customer API token).
func isCustomerRequest(c *gin.Context) bool {
	if v, _ := c.Get("is_customer"); v == true {
		return true
	}
	return c.GetString("user_role") == "Customer"
}

// customerLoginFromCtx returns the customer login (ticket.customer_user_id) of
// a customer principal, or "" when it cannot be determined. The JWT email is
// never used: it need not equal the login.
func customerLoginFromCtx(c *gin.Context, db *sql.DB) string {
	for _, key := range []string{"customer_login", "username"} {
		if login := strings.TrimSpace(c.GetString(key)); login != "" {
			return login
		}
	}
	if v, ok := c.Get("claims"); ok {
		if claims, ok := v.(*auth.Claims); ok && claims.Role == "Customer" && strings.TrimSpace(claims.Login) != "" {
			return strings.TrimSpace(claims.Login)
		}
	}
	// Customer API token: customer_user_id is the customer_user.id (int).
	if v, ok := c.Get("customer_user_id"); ok {
		if id, ok := v.(int); ok && id > 0 && db != nil {
			var login string
			if err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(
				`SELECT login FROM customer_user WHERE id = ?`), id).Scan(&login); err == nil {
				return strings.TrimSpace(login)
			}
		}
	}
	return ""
}
