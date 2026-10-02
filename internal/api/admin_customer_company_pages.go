package api

// Customer company sub-pages: the company's customer users, its tickets and
// its customer users' service assignments (/admin/customer/companies/:id/...).

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// companyTicketsPerPage is the page size of the company tickets page.
const companyTicketsPerPage = 50

type companyCustomerUser struct {
	ID          int    `json:"id"`
	Login       string `json:"login"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone"`
	Mobile      string `json:"mobile"`
	ValidID     int    `json:"valid_id"`
	ValidName   string `json:"valid_name"`
	TicketCount int    `json:"ticket_count"`
}

type companyTicket struct {
	ID           int64     `json:"id"`
	TN           string    `json:"tn"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	StateType    string    `json:"state_type"`
	Priority     string    `json:"priority"`
	Queue        string    `json:"queue"`
	CustomerUser string    `json:"customer_user"`
	CreateTime   time.Time `json:"create_time"`
	AgeValue     int       `json:"age_value"`
	AgeUnit      string    `json:"age_unit"` // minutes, hours or days
}

type companyServiceCell struct {
	UserID   int    `json:"customer_user_id"`
	Login    string `json:"login"`
	FullName string `json:"full_name"`
	Assigned bool   `json:"assigned"`
}

type companyServiceRow struct {
	ID            int                  `json:"id"`
	Name          string               `json:"name"`
	Comments      string               `json:"comments"`
	AssignedCount int                  `json:"assigned_count"`
	Users         []companyServiceCell `json:"users"`
}

// loadCompanyForPage loads the company named by the :id route parameter. When
// the company can't be shown it writes the error response and returns nil.
func loadCompanyForPage(c *gin.Context, db *sql.DB) *CustomerCompanyInfo {
	company, err := loadCustomerCompany(db, c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		sendErrorResponse(c, http.StatusNotFound, "Customer company not found")
		return nil
	}
	if err != nil {
		log.Printf("load customer company %q: %v", c.Param("id"), err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load customer company")
		return nil
	}
	return company
}

func companyPageContext(c *gin.Context, company *CustomerCompanyInfo, title string) pongo2.Context {
	return pongo2.Context{
		"Title":           title,
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-companies",
		"User":            getUserMapForTemplate(c),
		"Company":         company,
		"CompanyPath":     url.PathEscape(company.CustomerID),
	}
}

// loadCompanyCustomerUsers returns the customer users whose customer_id is the company.
func loadCompanyCustomerUsers(db *sql.DB, customerID string) ([]companyCustomerUser, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT cu.id, cu.login, cu.email, cu.first_name, cu.last_name,
		       COALESCE(cu.phone, ''), COALESCE(cu.mobile, ''), cu.valid_id, COALESCE(v.name, ''),
		       (SELECT COUNT(*) FROM ticket t WHERE t.customer_user_id = cu.login)
		FROM customer_user cu
		LEFT JOIN valid v ON v.id = cu.valid_id
		WHERE cu.customer_id = ?
		ORDER BY cu.last_name, cu.first_name, cu.login`), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []companyCustomerUser{}
	for rows.Next() {
		var u companyCustomerUser
		if err := rows.Scan(&u.ID, &u.Login, &u.Email, &u.FirstName, &u.LastName,
			&u.Phone, &u.Mobile, &u.ValidID, &u.ValidName, &u.TicketCount); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// handleAdminCustomerCompanyUsers lists the customer users of a company.
func handleAdminCustomerCompanyUsers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		company := loadCompanyForPage(c, db)
		if company == nil {
			return
		}
		users, err := loadCompanyCustomerUsers(db, company.CustomerID)
		if err != nil {
			log.Printf("company %q users: %v", company.CustomerID, err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to load customer users")
			return
		}

		if wantsJSONResponse(c) {
			c.JSON(http.StatusOK, gin.H{"success": true, "company_name": company.Name, "users": users})
			return
		}
		ctx := companyPageContext(c, company, "Customer Company Users")
		ctx["Users"] = users
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_company_users.pongo2", ctx)
	}
}

// ticketAge splits the time since created into a whole number of the largest
// fitting unit (minutes, hours or days) for display.
func ticketAge(created, now time.Time) (int, string) {
	d := now.Sub(created)
	switch {
	case d < time.Hour:
		return int(d / time.Minute), "minutes"
	case d < 24*time.Hour:
		return int(d / time.Hour), "hours"
	default:
		return int(d / (24 * time.Hour)), "days"
	}
}

// visibleQueueFilter returns the SQL condition and arguments that restrict
// tickets to the queues the current user may read. Members of the admin group
// see every queue, like the agent ticket list. ok is false when the user can
// read no queue at all.
func visibleQueueFilter(c *gin.Context, db *sql.DB) (cond string, args []any, ok bool, err error) {
	access := service.NewQueueAccessService(db)
	userID := GetUserIDFromCtxUint(c, 0)
	isAdmin, err := access.IsAdmin(c.Request.Context(), userID)
	if err != nil {
		return "", nil, false, err
	}
	if isAdmin {
		return "", nil, true, nil
	}
	queueIDs, err := access.GetAccessibleQueueIDs(c.Request.Context(), userID, "ro")
	if err != nil || len(queueIDs) == 0 {
		return "", nil, false, err
	}
	placeholders := make([]string, len(queueIDs))
	for i, id := range queueIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	return " AND t.queue_id IN (" + strings.Join(placeholders, ", ") + ")", args, true, nil
}

// handleAdminCustomerCompanyTickets lists a company's tickets (ticket.customer_id),
// newest first, restricted to queues the admin may read.
func handleAdminCustomerCompanyTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		company := loadCompanyForPage(c, db)
		if company == nil {
			return
		}
		page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err != nil || page < 1 {
			page = 1
		}

		cond, queueArgs, anyQueue, err := visibleQueueFilter(c, db)
		if err != nil {
			log.Printf("company %q tickets: queue access: %v", company.CustomerID, err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to resolve queue permissions")
			return
		}

		tickets := []companyTicket{}
		total := 0
		if anyQueue {
			args := append([]any{company.CustomerID}, queueArgs...)
			where := " FROM ticket t WHERE t.customer_id = ?" + cond
			if err := db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*)"+where), args...).Scan(&total); err != nil {
				log.Printf("company %q tickets: count: %v", company.CustomerID, err)
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to load tickets")
				return
			}
			tickets, err = loadCompanyTickets(db, cond, args, page)
			if err != nil {
				log.Printf("company %q tickets: %v", company.CustomerID, err)
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to load tickets")
				return
			}
		}

		totalPages := (total + companyTicketsPerPage - 1) / companyTicketsPerPage
		if wantsJSONResponse(c) {
			c.JSON(http.StatusOK, gin.H{
				"success": true, "company_name": company.Name, "tickets": tickets,
				"pagination": gin.H{"page": page, "per_page": companyTicketsPerPage, "total": total, "total_pages": totalPages},
			})
			return
		}
		ctx := companyPageContext(c, company, "Customer Company Tickets")
		ctx["Tickets"] = tickets
		ctx["Total"] = total
		ctx["Page"] = page
		ctx["TotalPages"] = totalPages
		ctx["PrevPage"] = page - 1
		ctx["NextPage"] = page + 1
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_company_tickets.pongo2", ctx)
	}
}

func loadCompanyTickets(db *sql.DB, cond string, args []any, page int) ([]companyTicket, error) {
	query := `
		SELECT t.id, t.tn, t.title, COALESCE(ts.name, ''), COALESCE(tst.name, ''),
		       COALESCE(tp.name, ''), COALESCE(q.name, ''), COALESCE(t.customer_user_id, ''), t.create_time
		FROM ticket t
		LEFT JOIN ticket_state ts ON ts.id = t.ticket_state_id
		LEFT JOIN ticket_state_type tst ON tst.id = ts.type_id
		LEFT JOIN ticket_priority tp ON tp.id = t.ticket_priority_id
		LEFT JOIN queue q ON q.id = t.queue_id
		WHERE t.customer_id = ?` + cond + `
		ORDER BY t.create_time DESC, t.id DESC
		LIMIT ? OFFSET ?`
	args = append(args, companyTicketsPerPage, (page-1)*companyTicketsPerPage)
	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	tickets := []companyTicket{}
	for rows.Next() {
		var t companyTicket
		var title sql.NullString
		if err := rows.Scan(&t.ID, &t.TN, &title, &t.State, &t.StateType,
			&t.Priority, &t.Queue, &t.CustomerUser, &t.CreateTime); err != nil {
			return nil, err
		}
		t.Title = title.String
		t.AgeValue, t.AgeUnit = ticketAge(t.CreateTime, now)
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

// loadCompanyServiceMatrix returns every valid service with, for each customer
// user of the company, whether service_customer_user assigns it to them.
func loadCompanyServiceMatrix(db *sql.DB, customerID string, users []companyCustomerUser) ([]companyServiceRow, error) {
	assigned := map[[2]int]bool{}
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT scu.service_id, cu.id
		FROM service_customer_user scu
		JOIN customer_user cu ON cu.login = scu.customer_user_login
		WHERE cu.customer_id = ?`), customerID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var serviceID, userID int
		if err := rows.Scan(&serviceID, &userID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		assigned[[2]int{serviceID, userID}] = true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(database.ConvertPlaceholders(`
		SELECT id, name, COALESCE(comments, '') FROM service WHERE valid_id = 1 ORDER BY name`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := []companyServiceRow{}
	for rows.Next() {
		var s companyServiceRow
		if err := rows.Scan(&s.ID, &s.Name, &s.Comments); err != nil {
			return nil, err
		}
		s.Users = make([]companyServiceCell, 0, len(users))
		for _, u := range users {
			cell := companyServiceCell{
				UserID:   u.ID,
				Login:    u.Login,
				FullName: strings.TrimSpace(u.FirstName + " " + u.LastName),
				Assigned: assigned[[2]int{s.ID, u.ID}],
			}
			if cell.Assigned {
				s.AssignedCount++
			}
			s.Users = append(s.Users, cell)
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func loadValidServiceIDs(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`SELECT id FROM service WHERE valid_id = 1`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// handleAdminCustomerCompanyServices shows which valid services are assigned to
// which of the company's customer users. GoatFlow, like OTRS, stores service
// assignments per customer user (service_customer_user); there is no
// company-level assignment table.
func handleAdminCustomerCompanyServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		company := loadCompanyForPage(c, db)
		if company == nil {
			return
		}
		users, err := loadCompanyCustomerUsers(db, company.CustomerID)
		if err != nil {
			log.Printf("company %q services: users: %v", company.CustomerID, err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to load customer users")
			return
		}
		services, err := loadCompanyServiceMatrix(db, company.CustomerID, users)
		if err != nil {
			log.Printf("company %q services: %v", company.CustomerID, err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to load services")
			return
		}

		if wantsJSONResponse(c) {
			c.JSON(http.StatusOK, gin.H{"success": true, "company_name": company.Name, "services": services})
			return
		}
		ctx := companyPageContext(c, company, "Customer Company Services")
		ctx["Services"] = services
		ctx["UserCount"] = len(users)
		ctx["Saved"] = c.Query("saved") == "1"
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_company_services.pongo2", ctx)
	}
}

// handleAdminUpdateCustomerCompanyServices stores the service matrix posted by
// the company services page. Each "assign" value is "<service id>:<customer
// user id>"; every pair of a company user and a valid service that is not
// posted is unassigned. Assignments of invalid services are left untouched.
func handleAdminUpdateCustomerCompanyServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		company := loadCompanyForPage(c, db)
		if company == nil {
			return
		}
		users, err := loadCompanyCustomerUsers(db, company.CustomerID)
		if err != nil {
			log.Printf("company %q services update: users: %v", company.CustomerID, err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to load customer users")
			return
		}
		loginByID := make(map[int]string, len(users))
		for _, u := range users {
			loginByID[u.ID] = u.Login
		}
		validServices, err := loadValidServiceIDs(db)
		if err != nil {
			log.Printf("company %q services update: services: %v", company.CustomerID, err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to load services")
			return
		}

		type pair struct {
			serviceID int
			login     string
		}
		var pairs []pair
		seen := map[string]bool{}
		for _, raw := range c.PostFormArray("assign") {
			sid, uid, found := strings.Cut(raw, ":")
			serviceID, errS := strconv.Atoi(sid)
			userID, errU := strconv.Atoi(uid)
			login, isCompanyUser := loginByID[userID]
			if !found || errS != nil || errU != nil || !validServices[serviceID] || !isCompanyUser {
				sendErrorResponse(c, http.StatusBadRequest,
					fmt.Sprintf("Invalid assignment %q: expected <valid service id>:<customer user id of this company>", raw))
				return
			}
			if !seen[raw] {
				seen[raw] = true
				pairs = append(pairs, pair{serviceID, login})
			}
		}

		tx, err := db.Begin()
		if err != nil {
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to save services")
			return
		}
		defer func() { _ = tx.Rollback() }()

		clearQuery := database.ConvertPlaceholders(`
			DELETE FROM service_customer_user
			WHERE customer_user_login = ?
			  AND service_id IN (SELECT id FROM service WHERE valid_id = 1)`)
		for _, u := range users {
			if _, err := tx.Exec(clearQuery, u.Login); err != nil {
				log.Printf("company %q services update: clear %q: %v", company.CustomerID, u.Login, err)
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to save services")
				return
			}
		}
		insert := database.ConvertPlaceholders(`
			INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
			VALUES (?, ?, CURRENT_TIMESTAMP, ?)`)
		actorID := GetUserIDFromCtxUint(c, 1)
		for _, p := range pairs {
			if _, err := tx.Exec(insert, p.login, p.serviceID, actorID); err != nil {
				log.Printf("company %q services update: assign %d to %q: %v", company.CustomerID, p.serviceID, p.login, err)
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to save services")
				return
			}
		}
		if err := tx.Commit(); err != nil {
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to save services")
			return
		}

		if wantsJSONResponse(c) {
			c.JSON(http.StatusOK, gin.H{"success": true, "assigned": len(pairs)})
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/customer/companies/"+url.PathEscape(company.CustomerID)+"/services?saved=1")
	}
}
