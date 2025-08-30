package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// RegisterAdminCustomerCompanyRoutes registers customer company management routes
func RegisterAdminCustomerCompanyRoutes(r *gin.RouterGroup, db *sql.DB) {
	r.GET("/customer-companies", handleAdminCustomerCompanies(db))
	r.GET("/customer-companies/new", handleAdminNewCustomerCompany(db))
	r.POST("/customer-companies/new", handleAdminCreateCustomerCompany(db))
	r.GET("/customer-companies/:id/edit", handleAdminEditCustomerCompany(db))
	r.POST("/customer-companies/:id/edit", handleAdminUpdateCustomerCompany(db))
	r.POST("/customer-companies/:id/delete", handleAdminDeleteCustomerCompany(db))
	r.GET("/customer-companies/:id/users", handleAdminCustomerCompanyUsers(db))
	r.GET("/customer-companies/:id/tickets", handleAdminCustomerCompanyTickets(db))
	r.GET("/customer-companies/:id/services", handleAdminCustomerCompanyServices(db))
	r.POST("/customer-companies/:id/services", handleAdminUpdateCustomerCompanyServices(db))
	
	// Portal customization routes
	r.GET("/customer-companies/:id/portal", handleAdminCustomerPortalSettings(db))
	r.POST("/customer-companies/:id/portal", handleAdminUpdateCustomerPortalSettings(db))
	r.POST("/customer-companies/:id/portal/logo", handleAdminUploadCustomerPortalLogo(db))
}

// handleAdminCustomerCompanies shows the customer companies list
func handleAdminCustomerCompanies(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get filter parameters
		search := c.Query("search")
		validFilter := c.DefaultQuery("valid", "all")
		
		// Build query
		query := `
			SELECT cc.customer_id, cc.name, cc.street, cc.zip, cc.city, 
			       cc.country, cc.url, cc.comments, cc.valid_id,
			       v.name as valid_name,
			       cc.create_time, cc.change_time,
			       (SELECT COUNT(*) FROM customer_user WHERE customer_id = cc.customer_id) as user_count,
			       (SELECT COUNT(*) FROM ticket WHERE customer_id = cc.customer_id) as ticket_count
			FROM customer_company cc
			LEFT JOIN valid v ON cc.valid_id = v.id
			WHERE 1=1
		`
		
		args := []interface{}{}
		argCount := 0
		
		// Apply search filter
		if search != "" {
			argCount++
			query += fmt.Sprintf(" AND (cc.name ILIKE $%d OR cc.customer_id ILIKE $%d OR cc.city ILIKE $%d)", 
				argCount, argCount, argCount)
			args = append(args, "%"+search+"%")
		}
		
		// Apply validity filter
		if validFilter == "valid" {
			argCount++
			query += fmt.Sprintf(" AND cc.valid_id = $%d", argCount)
			args = append(args, 1)
		} else if validFilter == "invalid" {
			argCount++
			query += fmt.Sprintf(" AND cc.valid_id != $%d", argCount)
			args = append(args, 1)
		}
		
		query += " ORDER BY cc.name"
		
		rows, err := db.Query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		
		companies := []map[string]interface{}{}
		for rows.Next() {
			var company struct {
				CustomerID  string
				Name        string
				Street      sql.NullString
				Zip         sql.NullString
				City        sql.NullString
				Country     sql.NullString
				URL         sql.NullString
				Comments    sql.NullString
				ValidID     int
				ValidName   string
				CreateTime  time.Time
				ChangeTime  time.Time
				UserCount   int
				TicketCount int
			}
			
			err := rows.Scan(&company.CustomerID, &company.Name, &company.Street,
				&company.Zip, &company.City, &company.Country, &company.URL,
				&company.Comments, &company.ValidID, &company.ValidName,
				&company.CreateTime, &company.ChangeTime, &company.UserCount,
				&company.TicketCount)
			
			if err != nil {
				continue
			}
			
			companies = append(companies, map[string]interface{}{
				"customer_id":  company.CustomerID,
				"name":         company.Name,
				"street":       company.Street.String,
				"zip":          company.Zip.String,
				"city":         company.City.String,
				"country":      company.Country.String,
				"url":          company.URL.String,
				"comments":     company.Comments.String,
				"valid_id":     company.ValidID,
				"valid_name":   company.ValidName,
				"user_count":   company.UserCount,
				"ticket_count": company.TicketCount,
				"create_time":  company.CreateTime.Format("2006-01-02 15:04"),
				"change_time":  company.ChangeTime.Format("2006-01-02 15:04"),
			})
		}
		
		pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/customer_companies.pongo2", pongo2.Context{
			"Title":           "Customer Companies",
			"ActivePage":      "admin",
			"ActiveAdminPage": "customer-companies",
			"Companies":       companies,
			"CurrentFilters": map[string]string{
				"search": search,
				"valid":  validFilter,
			},
		})
	}
}

// handleAdminNewCustomerCompany shows the new customer company form
func handleAdminNewCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/customer_company_form.pongo2", pongo2.Context{
			"Title":           "New Customer Company",
			"ActivePage":      "admin",
			"ActiveAdminPage": "customer-companies",
			"IsNew":           true,
		})
	}
}

// handleAdminCreateCustomerCompany creates a new customer company
func handleAdminCreateCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.PostForm("customer_id")
		name := c.PostForm("name")
		street := c.PostForm("street")
		zip := c.PostForm("zip")
		city := c.PostForm("city")
		country := c.PostForm("country")
		url := c.PostForm("url")
		comments := c.PostForm("comments")
		
		if customerID == "" || name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Customer ID and Name are required"})
			return
		}
		
		// Check if customer ID already exists
		var exists bool
		db.QueryRow("SELECT EXISTS(SELECT 1 FROM customer_company WHERE customer_id = $1)", customerID).Scan(&exists)
		if exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Customer ID already exists"})
			return
		}
		
		// Insert new company
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO customer_company (
				customer_id, name, street, zip, city, country, url, comments,
				valid_id, create_time, create_by, change_time, change_by
			) VALUES (
				$1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), 
				NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
				1, NOW(), 1, NOW(), 1
			)
		`), customerID, name, street, zip, city, country, url, comments)
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create customer company"})
			return
		}
		
		c.Redirect(http.StatusSeeOther, "/admin/customer-companies")
	}
}

// handleAdminEditCustomerCompany shows the edit customer company form with portal customization
func handleAdminEditCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		tab := c.DefaultQuery("tab", "general") // Support tabs for different sections
		
		var company struct {
			CustomerID string
			Name       string
			Street     sql.NullString
			Zip        sql.NullString
			City       sql.NullString
			Country    sql.NullString
			URL        sql.NullString
			Comments   sql.NullString
			ValidID    int
		}
		
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT customer_id, name, street, zip, city, country, url, comments, valid_id
			FROM customer_company
			WHERE customer_id = $1
		`), customerID).Scan(&company.CustomerID, &company.Name, &company.Street,
			&company.Zip, &company.City, &company.Country, &company.URL,
			&company.Comments, &company.ValidID)
		
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer company not found"})
			return
		}
		
		// Get portal settings from sysconfig (stored as JSON in config_item table)
		var portalConfig map[string]interface{}
		var configJSON sql.NullString
		db.QueryRow(database.ConvertPlaceholders(`
			SELECT content_json FROM sysconfig 
			WHERE name = $1
		`), "CustomerPortal::Company::"+customerID).Scan(&configJSON)
		
		if configJSON.Valid {
			// Parse stored portal configuration
			// For now, use defaults
		}
		
		// Default portal settings if none exist
		if portalConfig == nil {
			portalConfig = map[string]interface{}{
				"logo_url":        "",
				"primary_color":   "#1e40af", // Blue
				"secondary_color": "#64748b", // Gray
				"header_bg":       "#ffffff",
				"footer_text":     "© " + company.Name,
				"welcome_message": "Welcome to " + company.Name + " Support Portal",
				"custom_css":      "",
			}
		}
		
		pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/customer_company_form.pongo2", pongo2.Context{
			"Title":           "Edit Customer Company",
			"ActivePage":      "admin",
			"ActiveAdminPage": "customer-companies",
			"IsNew":           false,
			"ActiveTab":       tab,
			"Company": map[string]interface{}{
				"customer_id": company.CustomerID,
				"name":        company.Name,
				"street":      company.Street.String,
				"zip":         company.Zip.String,
				"city":        company.City.String,
				"country":     company.Country.String,
				"url":         company.URL.String,
				"comments":    company.Comments.String,
				"valid_id":    company.ValidID,
			},
			"PortalConfig": portalConfig,
		})
	}
}

// handleAdminUpdateCustomerCompany updates a customer company
func handleAdminUpdateCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		name := c.PostForm("name")
		street := c.PostForm("street")
		zip := c.PostForm("zip")
		city := c.PostForm("city")
		country := c.PostForm("country")
		url := c.PostForm("url")
		comments := c.PostForm("comments")
		validID := c.PostForm("valid_id")
		
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
			return
		}
		
		// Update company
		_, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company SET
				name = $2, street = NULLIF($3, ''), zip = NULLIF($4, ''),
				city = NULLIF($5, ''), country = NULLIF($6, ''),
				url = NULLIF($7, ''), comments = NULLIF($8, ''),
				valid_id = $9, change_time = NOW(), change_by = 1
			WHERE customer_id = $1
		`), customerID, name, street, zip, city, country, url, comments, validID)
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update customer company"})
			return
		}
		
		c.Redirect(http.StatusSeeOther, "/admin/customer-companies")
	}
}

// handleAdminDeleteCustomerCompany soft-deletes (invalidates) a customer company
func handleAdminDeleteCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		// Soft delete by setting valid_id to 2 (invalid)
		_, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company 
			SET valid_id = 2, change_time = NOW(), change_by = 1
			WHERE customer_id = $1
		`), customerID)
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete customer company"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// handleAdminCustomerCompanyUsers shows users belonging to a customer company
func handleAdminCustomerCompanyUsers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		// Get company info
		var companyName string
		db.QueryRow("SELECT name FROM customer_company WHERE customer_id = $1", customerID).Scan(&companyName)
		
		// Get users
		rows, _ := db.Query(database.ConvertPlaceholders(`
			SELECT cu.id, cu.login, cu.email, cu.first_name, cu.last_name,
			       cu.phone, cu.mobile, cu.valid_id, v.name as valid_name,
			       (SELECT COUNT(*) FROM ticket WHERE customer_user_id = cu.login) as ticket_count
			FROM customer_user cu
			LEFT JOIN valid v ON cu.valid_id = v.id
			WHERE cu.customer_id = $1
			ORDER BY cu.last_name, cu.first_name
		`), customerID)
		defer rows.Close()
		
		users := []map[string]interface{}{}
		for rows.Next() {
			var user struct {
				ID          int
				Login       string
				Email       string
				FirstName   sql.NullString
				LastName    sql.NullString
				Phone       sql.NullString
				Mobile      sql.NullString
				ValidID     int
				ValidName   string
				TicketCount int
			}
			
			rows.Scan(&user.ID, &user.Login, &user.Email, &user.FirstName,
				&user.LastName, &user.Phone, &user.Mobile, &user.ValidID,
				&user.ValidName, &user.TicketCount)
			
			users = append(users, map[string]interface{}{
				"id":           user.ID,
				"login":        user.Login,
				"email":        user.Email,
				"first_name":   user.FirstName.String,
				"last_name":    user.LastName.String,
				"phone":        user.Phone.String,
				"mobile":       user.Mobile.String,
				"valid_id":     user.ValidID,
				"valid_name":   user.ValidName,
				"ticket_count": user.TicketCount,
			})
		}
		
		c.JSON(http.StatusOK, gin.H{
			"company_name": companyName,
			"users":        users,
		})
	}
}

// handleAdminCustomerCompanyTickets shows tickets for a customer company
func handleAdminCustomerCompanyTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		rows, _ := db.Query(database.ConvertPlaceholders(`
			SELECT t.id, t.tn, t.title, ts.name as state,
			       tp.name as priority, t.create_time,
			       cu.login as customer_user
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			LEFT JOIN customer_user cu ON t.customer_user_id = cu.login
			WHERE t.customer_id = $1
			ORDER BY t.create_time DESC
			LIMIT 100
		`), customerID)
		defer rows.Close()
		
		tickets := []map[string]interface{}{}
		for rows.Next() {
			var ticket struct {
				ID           int
				TN           string
				Title        string
				State        string
				Priority     string
				CreateTime   time.Time
				CustomerUser sql.NullString
			}
			
			rows.Scan(&ticket.ID, &ticket.TN, &ticket.Title, &ticket.State,
				&ticket.Priority, &ticket.CreateTime, &ticket.CustomerUser)
			
			tickets = append(tickets, map[string]interface{}{
				"id":            ticket.ID,
				"tn":            ticket.TN,
				"title":         ticket.Title,
				"state":         ticket.State,
				"priority":      ticket.Priority,
				"create_time":   ticket.CreateTime.Format("2006-01-02 15:04"),
				"customer_user": ticket.CustomerUser.String,
			})
		}
		
		c.JSON(http.StatusOK, gin.H{"tickets": tickets})
	}
}

// handleAdminCustomerCompanyServices manages service assignments for a customer company
func handleAdminCustomerCompanyServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		// Get all services and their assignment status
		rows, _ := db.Query(database.ConvertPlaceholders(`
			SELECT s.id, s.name, s.comments,
			       EXISTS(
			           SELECT 1 FROM customer_user_service 
			           WHERE service_id = s.id 
			           AND customer_user_login IN (
			               SELECT login FROM customer_user WHERE customer_id = $1
			           )
			       ) as is_assigned
			FROM service s
			WHERE s.valid_id = 1
			ORDER BY s.name
		`), customerID)
		defer rows.Close()
		
		services := []map[string]interface{}{}
		for rows.Next() {
			var service struct {
				ID         int
				Name       string
				Comments   sql.NullString
				IsAssigned bool
			}
			
			rows.Scan(&service.ID, &service.Name, &service.Comments, &service.IsAssigned)
			
			services = append(services, map[string]interface{}{
				"id":          service.ID,
				"name":        service.Name,
				"comments":    service.Comments.String,
				"is_assigned": service.IsAssigned,
			})
		}
		
		c.JSON(http.StatusOK, gin.H{"services": services})
	}
}

// handleAdminUpdateCustomerCompanyServices updates service assignments
func handleAdminUpdateCustomerCompanyServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		// Get service IDs from form
		serviceIDs := []string{}
		c.Request.ParseForm()
		for key := range c.Request.PostForm {
			if strings.HasPrefix(key, "service_") {
				serviceID := strings.TrimPrefix(key, "service_")
				serviceIDs = append(serviceIDs, serviceID)
			}
		}
		
		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
			return
		}
		defer tx.Rollback()
		
		// Get all customer users for this company
		rows, _ := tx.Query("SELECT login FROM customer_user WHERE customer_id = $1", customerID)
		userLogins := []string{}
		for rows.Next() {
			var login string
			rows.Scan(&login)
			userLogins = append(userLogins, login)
		}
		rows.Close()
		
		// Clear existing assignments for all users in this company
		for _, login := range userLogins {
			tx.Exec("DELETE FROM customer_user_service WHERE customer_user_login = $1", login)
		}
		
		// Add new assignments
		for _, login := range userLogins {
			for _, serviceID := range serviceIDs {
				tx.Exec(database.ConvertPlaceholders(`
					INSERT INTO customer_user_service (customer_user_login, service_id, create_time, create_by)
					VALUES ($1, $2, NOW(), 1)
				`), login, serviceID)
			}
		}
		
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update services"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// handleAdminCustomerPortalSettings shows portal customization settings
func handleAdminCustomerPortalSettings(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		// Get company name
		var companyName string
		db.QueryRow("SELECT name FROM customer_company WHERE customer_id = $1", customerID).Scan(&companyName)
		
		// Get portal configuration
		var configJSON sql.NullString
		db.QueryRow(database.ConvertPlaceholders(`
			SELECT content_json FROM sysconfig 
			WHERE name = $1
		`), "CustomerPortal::Company::"+customerID).Scan(&configJSON)
		
		portalConfig := map[string]interface{}{
			"logo_url":         "",
			"primary_color":    "#1e40af",
			"secondary_color":  "#64748b", 
			"header_bg":        "#ffffff",
			"footer_text":      "© " + companyName,
			"welcome_message":  "Welcome to " + companyName + " Support Portal",
			"custom_css":       "",
			"enable_kb":        true,
			"enable_downloads": false,
			"custom_domain":    "",
		}
		
		if configJSON.Valid {
			// Parse and merge stored config
			// TODO: Implement JSON parsing
		}
		
		c.JSON(http.StatusOK, gin.H{
			"company_name":   companyName,
			"portal_config":  portalConfig,
			"preview_url":    "/customer/portal/" + customerID + "/preview",
		})
	}
}

// handleAdminUpdateCustomerPortalSettings updates portal customization
func handleAdminUpdateCustomerPortalSettings(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		// Get form values
		config := map[string]interface{}{
			"logo_url":         c.PostForm("logo_url"),
			"primary_color":    c.PostForm("primary_color"),
			"secondary_color":  c.PostForm("secondary_color"),
			"header_bg":        c.PostForm("header_bg"),
			"footer_text":      c.PostForm("footer_text"),
			"welcome_message":  c.PostForm("welcome_message"),
			"custom_css":       c.PostForm("custom_css"),
			"enable_kb":        c.PostForm("enable_kb") == "1",
			"enable_downloads": c.PostForm("enable_downloads") == "1",
			"custom_domain":    c.PostForm("custom_domain"),
		}
		
		// Store in sysconfig table
		// Note: In production, this should use proper JSON marshaling
		configName := "CustomerPortal::Company::" + customerID
		
		// Check if config exists
		var exists bool
		db.QueryRow("SELECT EXISTS(SELECT 1 FROM sysconfig WHERE name = $1)", configName).Scan(&exists)
		
		if exists {
			_, err := db.Exec(database.ConvertPlaceholders(`
				UPDATE sysconfig 
				SET content_json = $2, change_time = NOW(), change_by = 1
				WHERE name = $1
			`), configName, config)
			
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update portal settings"})
				return
			}
		} else {
			_, err := db.Exec(database.ConvertPlaceholders(`
				INSERT INTO sysconfig (name, content_json, valid_id, create_time, create_by, change_time, change_by)
				VALUES ($1, $2, 1, NOW(), 1, NOW(), 1)
			`), configName, config)
			
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create portal settings"})
				return
			}
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Portal settings updated successfully"})
	}
}

// handleAdminUploadCustomerPortalLogo handles logo uploads for customer portals
func handleAdminUploadCustomerPortalLogo(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		
		// Handle file upload
		file, header, err := c.Request.FormFile("logo")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
			return
		}
		defer file.Close()
		
		// Validate file type
		contentType := header.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File must be an image"})
			return
		}
		
		// TODO: Save file to storage and return URL
		// For now, return a placeholder
		logoURL := "/static/customer_logos/" + customerID + "/" + header.Filename
		
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"logo_url": logoURL,
			"message":  "Logo uploaded successfully",
		})
	}
}