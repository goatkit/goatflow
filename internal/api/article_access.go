package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// articleRequester identifies who is calling an article endpoint.
type articleRequester struct {
	customer bool
	login    string // customer login (customers only)
	userID   int    // agent users.id (agents only)
}

func articleRequesterFrom(c *gin.Context) articleRequester {
	isCustomer := false
	if v, _ := c.Get("is_customer"); v == true {
		isCustomer = true
	} else if role, _ := c.Get("user_role"); role == "Customer" {
		isCustomer = true
	}
	if !isCustomer {
		return articleRequester{userID: GetUserIDFromCtx(c, 0)}
	}
	login := c.GetString("customer_login")
	if login == "" {
		login = c.GetString("username")
	}
	if login == "" {
		login = c.GetString("customer_email")
	}
	return articleRequester{customer: true, login: login}
}

// authorizeTicketArticles checks that the requester may use the articles of
// ticketID with permission perm ("ro" or "rw"). Agents need that permission on
// the ticket's queue (admins always pass). Customers may only read, and only
// on their own tickets; the caller must then restrict to customer-visible
// articles. On refusal the response is written and ok is false. A ticket the
// requester cannot see is reported as not found.
func authorizeTicketArticles(c *gin.Context, db *sql.DB, ticketID int, perm string) (req articleRequester, ok bool) {
	req = articleRequesterFrom(c)

	var queueID uint
	var customerUserID sql.NullString
	err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(
		`SELECT queue_id, customer_user_id FROM ticket WHERE id = ?`), ticketID).Scan(&queueID, &customerUserID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return req, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ticket"})
		return req, false
	}

	if req.customer {
		if req.login == "" || !customerUserID.Valid || customerUserID.String != req.login {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return req, false
		}
		if perm != "ro" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Customers cannot modify articles"})
			return req, false
		}
		return req, true
	}

	if req.userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return req, false
	}
	qa := service.NewQueueAccessService(db)
	ctx := c.Request.Context()
	isAdmin, err := qa.IsAdmin(ctx, uint(req.userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return req, false
	}
	if !isAdmin {
		has, err := qa.HasQueueAccess(ctx, uint(req.userID), queueID, perm)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			return req, false
		}
		if !has {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return req, false
		}
	}
	return req, true
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
