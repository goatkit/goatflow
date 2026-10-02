package api

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/customfields"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
)

// Custom field values belong to an entity, and reading or changing them needs
// the access the entity itself needs (the routes only require an agent):
//   - ticket, article: ro (read) / rw (write) on the queue of the ticket;
//   - queue: ro on the queue to read, admin to write (queues are admin data);
//   - agent, group, customer_group, organisation: any agent reads, admin writes;
//   - contact (customer user): any agent reads and writes, like the customer
//     user record.
//
// Entities the caller may not read are reported as not found.

// customFieldAccess is the permission context of one request.
type customFieldAccess struct {
	c       *gin.Context
	db      *sql.DB
	qa      middleware.QueueAccessChecker
	userID  uint
	isAdmin bool
}

var errCustomFieldNoAgent = errors.New("no authenticated agent")

func newCustomFieldAccess(c *gin.Context) (*customFieldAccess, error) {
	if middleware.IsCustomerPrincipal(c) {
		return nil, errCustomFieldNoAgent
	}
	a := &customFieldAccess{c: c}
	switch v := c.Value("user_id").(type) {
	case uint:
		a.userID = v
	case int:
		if v > 0 {
			a.userID = uint(v)
		}
	case int64:
		if v > 0 {
			a.userID = uint(v)
		}
	}
	if a.userID == 0 {
		return nil, errCustomFieldNoAgent
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	a.db = db
	a.qa = middleware.NewQueueAccessChecker(db)
	if a.qa == nil {
		return nil, errors.New("queue access checker unavailable")
	}
	if a.isAdmin, err = a.qa.IsAdmin(c.Request.Context(), a.userID); err != nil {
		return nil, err
	}
	return a, nil
}

// customFieldAccessOrAbort writes 401/500 and returns nil when the request has
// no agent or the permission context cannot be built.
func customFieldAccessOrAbort(c *gin.Context) *customFieldAccess {
	a, err := newCustomFieldAccess(c)
	if errors.Is(err, errCustomFieldNoAgent) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Agent authentication required"})
		return nil
	}
	if err != nil {
		log.Printf("custom field permission context: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return nil
	}
	return a
}

func (a *customFieldAccess) queuePerm(queueID uint, perm string) (bool, error) {
	if a.isAdmin {
		return true, nil
	}
	return a.qa.HasQueueAccess(a.c.Request.Context(), a.userID, queueID, perm)
}

// objectQueue returns the queue that decides access to a ticket or article,
// or ok=false when the object does not exist.
func (a *customFieldAccess) objectQueue(entityType string, objectID int64) (queueID uint, ok bool, err error) {
	q := `SELECT queue_id FROM ticket WHERE id = ?`
	if entityType == customfields.EntityArticle {
		q = `SELECT t.queue_id FROM article a JOIN ticket t ON t.id = a.ticket_id WHERE a.id = ?`
	}
	err = a.db.QueryRowContext(a.c.Request.Context(), database.ConvertPlaceholders(q), objectID).Scan(&queueID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return queueID, err == nil, err
}

// check returns 0 when the caller may read (write=false) or change the custom
// field values of the object, else the HTTP status to answer with.
func (a *customFieldAccess) check(entityType string, objectID int64, write bool) (int, error) {
	switch entityType {
	case customfields.EntityTicket, customfields.EntityArticle:
		queueID, ok, err := a.objectQueue(entityType, objectID)
		if err != nil || !ok {
			return http.StatusNotFound, err
		}
		canRead, err := a.queuePerm(queueID, "ro")
		if err != nil || !canRead {
			return http.StatusNotFound, err
		}
		if !write {
			return 0, nil
		}
		canWrite, err := a.queuePerm(queueID, "rw")
		if err != nil || !canWrite {
			return http.StatusForbidden, err
		}
		return 0, nil
	case customfields.EntityQueue:
		canRead, err := a.queuePerm(uint(objectID), "ro")
		if err != nil || !canRead {
			return http.StatusNotFound, err
		}
		if write && !a.isAdmin {
			return http.StatusForbidden, nil
		}
		return 0, nil
	case customfields.EntityContact:
		return 0, nil
	default: // agent, group, customer_group, organisation
		if write && !a.isAdmin {
			return http.StatusForbidden, nil
		}
		return 0, nil
	}
}

// checkOrAbort is check for handlers: on refusal it writes the response and
// returns false.
func (a *customFieldAccess) checkOrAbort(entityType string, objectID int64, write bool) bool {
	status, err := a.check(entityType, objectID, write)
	switch {
	case err != nil:
		log.Printf("custom field permission check %s/%d: %v", entityType, objectID, err)
		a.c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
	case status == http.StatusNotFound:
		a.c.JSON(http.StatusNotFound, gin.H{"error": "Entity not found"})
	case status == http.StatusForbidden:
		a.c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to change this entity"})
	default:
		return true
	}
	return false
}

// readable keeps the ids of objects the caller may read.
func (a *customFieldAccess) readable(entityType string, ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		status, err := a.check(entityType, id, false)
		if err != nil {
			return nil, err
		}
		if status == 0 {
			out = append(out, id)
		}
	}
	return out, nil
}
