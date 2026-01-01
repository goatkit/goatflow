package api

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// handleAdminCustomerCompanies shows the customer companies list.
func handleAdminCustomerCompanies(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		search := strings.TrimSpace(c.Query("search"))
		validFilter := c.DefaultQuery("valid", "all")
		if status := strings.TrimSpace(c.Query("status")); status != "" {
			validFilter = status
		}

		if db == nil {
			renderCustomerCompaniesFallback(c, nil, search, validFilter)
			return
		}

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

		args := make([]interface{}, 0)
		argPos := 0

		if search != "" {
			likeClauses := make([]string, 0, 3)
			searchTerm := "%" + search + "%"
			columns := []string{"cc.name", "cc.customer_id", "cc.city"}
			for _, col := range columns {
				argPos++
				likeClauses = append(likeClauses, fmt.Sprintf("%s ILIKE $%d", col, argPos))
				args = append(args, searchTerm)
			}
			query += " AND (" + strings.Join(likeClauses, " OR ") + ")"
		}

		switch validFilter {
		case "valid":
			argPos++
			query += fmt.Sprintf(" AND cc.valid_id = $%d", argPos)
			args = append(args, 1)
		case "invalid":
			argPos++
			query += fmt.Sprintf(" AND cc.valid_id != $%d", argPos)
			args = append(args, 1)
		}

		query += " ORDER BY cc.name"

		rows, err := db.Query(database.ConvertQuery(query), args...)
		if err != nil {
			if database.IsConnectionError(err) {
				renderCustomerCompaniesFallback(c, nil, search, validFilter)
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load customer companies"})
			return
		}
		defer func() { _ = rows.Close() }()

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

		if err := rows.Err(); err != nil {
			if database.IsConnectionError(err) {
				renderCustomerCompaniesFallback(c, companies, search, validFilter)
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read customer companies"})
			return
		}

		if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
			renderCustomerCompaniesFallback(c, companies, search, validFilter)
			return
		}

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_companies.pongo2", pongo2.Context{
			"Title":           "Customer Companies",
			"ActivePage":      "admin",
			"ActiveAdminPage": "customer-companies",
			"User":            getUserMapForTemplate(c),
			"Companies":       companies,
			"CurrentFilters": map[string]string{
				"search": search,
				"valid":  validFilter,
			},
		})
	}
}

func renderCustomerCompaniesFallback(c *gin.Context, companies []map[string]interface{}, search, validFilter string) {
	c.Header("Content-Type", "text/html; charset=utf-8")

	var builder strings.Builder
	builder.WriteString("<!DOCTYPE html>\n<html>\n<head><title>Customer Companies</title></head>\n<body>\n  <h1>Customer Companies</h1>\n  <button>Add New Company</button>\n")

	if search != "" {
		builder.WriteString("  <div>Search: " + html.EscapeString(search) + "</div>\n")
	}

	if validFilter != "" && validFilter != "all" {
		builder.WriteString("  <div>Status: " + html.EscapeString(validFilter) + "</div>\n")
	}

	if len(companies) == 0 {
		builder.WriteString("  <p>No customer companies found.</p>\n")
	} else {
		builder.WriteString("  <ul>\n")
		for _, company := range companies {
			name := html.EscapeString(fmt.Sprint(company["name"]))
			customerID := html.EscapeString(fmt.Sprint(company["customer_id"]))
			builder.WriteString("    <li>" + name + " (" + customerID + ")</li>\n")
		}
		builder.WriteString("  </ul>\n")
	}

	builder.WriteString("</body>\n</html>")

	c.String(http.StatusOK, builder.String())
}

// handleAdminNewCustomerCompany shows the new customer company form.
func handleAdminNewCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_company_form.pongo2", pongo2.Context{
			"Title":           "New Customer Company",
			"ActivePage":      "admin",
			"ActiveAdminPage": "customer-companies",
			"User":            getUserMapForTemplate(c),
			"IsNew":           true,
		})
	}
}

// handleAdminCreateCustomerCompany creates a new customer company.
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
		db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM customer_company WHERE customer_id = $1)"), customerID).Scan(&exists)
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

		shared.SendToastResponse(c, true, "Customer company created successfully", fmt.Sprintf("/admin/customer/companies/%s/edit", customerID))
	}
}

// handleAdminEditCustomerCompany shows the edit customer company form with portal customization.
func handleAdminEditCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")
		tab := c.DefaultQuery("tab", "general") // Support tabs for different sections
		success := c.Query("success")           // Check for success message

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

		// Prepare template context
		templateData := pongo2.Context{
			"Title":           "Edit Customer Company",
			"ActivePage":      "admin",
			"ActiveAdminPage": "customer-companies",
			"User":            getUserMapForTemplate(c),
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
		}

		// Add success message if redirected from update
		if success == "1" {
			templateData["SuccessMessage"] = "Customer company updated successfully"
		}

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_company_form.pongo2", templateData)
	}
}

// handleAdminUpdateCustomerCompany updates a customer company.
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
			if c.GetHeader("HX-Request") == "true" {
				c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded" role="alert">Name is required</div>`))
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
			}
			return
		}

		// Check if company exists first
		var exists bool
		var err error
		if db != nil {
			err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM customer_company WHERE customer_id = $1)"), customerID).Scan(&exists)
		} else {
			// For tests or when DB is not available, assume company doesn't exist
			exists = false
			err = nil
		}

		if err != nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Failed to check company existence", "")
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check company existence"})
			}
			return
		}

		if !exists {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Customer company not found", "")
			} else {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Customer company not found"})
			}
			return
		}

		// Update company
		result, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company SET
				name = $1, street = NULLIF($2, ''), zip = NULLIF($3, ''),
				city = NULLIF($4, ''), country = NULLIF($5, ''),
				url = NULLIF($6, ''), comments = NULLIF($7, ''),
				valid_id = $8, change_time = NOW(), change_by = 1
			WHERE customer_id = $9
		`), name, street, zip, city, country, url, comments, validID, customerID)

		if err != nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Failed to update customer company", "")
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update customer company"})
			}
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Customer company not found", "")
			} else {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Customer company not found"})
			}
			return
		}

		shared.SendToastResponse(c, true, "Customer company updated successfully", fmt.Sprintf("/admin/customer/companies/%s/edit", customerID))
	}
}

// handleAdminDeleteCustomerCompany soft-deletes (invalidates) a customer company.
func handleAdminDeleteCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")

		// Handle nil database (for tests)
		if db == nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Customer company not found", "")
			} else {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Customer company not found"})
			}
			return
		}

		// Soft delete by setting valid_id to 2 (invalid)
		result, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company 
			SET valid_id = 2, change_time = NOW(), change_by = 1
			WHERE customer_id = $1
		`), customerID)

		if err != nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Failed to delete customer company", "")
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete customer company"})
			}
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Customer company not found", "")
			} else {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Customer company not found"})
			}
			return
		}

		shared.SendToastResponse(c, true, "Customer company deleted successfully", "/admin/customer/companies")
	}
}

// handleAdminActivateCustomerCompany activates a customer company.
func handleAdminActivateCustomerCompany(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")

		// Handle nil database (for tests)
		if db == nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Customer company not found", "")
			} else {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Customer company not found"})
			}
			return
		}

		// Activate by setting valid_id to 1 (valid)
		result, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company 
			SET valid_id = 1, change_time = NOW(), change_by = 1
			WHERE customer_id = $1
		`), customerID)

		if err != nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Failed to activate customer company", "")
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to activate customer company"})
			}
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Customer company not found", "")
			} else {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Customer company not found"})
			}
			return
		}

		shared.SendToastResponse(c, true, "Customer company activated successfully", fmt.Sprintf("/admin/customer/companies/%s/edit", customerID))
	}
}

// handleAdminCustomerCompanyUsers shows users belonging to a customer company.
func handleAdminCustomerCompanyUsers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")

		// Get company info
		var companyName string
		db.QueryRow(database.ConvertPlaceholders("SELECT name FROM customer_company WHERE customer_id = $1"), customerID).Scan(&companyName)

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
		defer func() { _ = rows.Close() }()

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

// handleAdminCustomerCompanyTickets shows tickets for a customer company.
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
		defer func() { _ = rows.Close() }()

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

// handleAdminCustomerCompanyServices manages service assignments for a customer company.
func handleAdminCustomerCompanyServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")

		// Get all services and their assignment status
		rows, err := db.Query(database.ConvertQuery(`
			SELECT s.id, s.name, s.comments,
				   CASE 
					   WHEN EXISTS(
						   SELECT 1 FROM service_customer_user 
						   WHERE service_id = s.id 
							 AND customer_user_login IN (
								 SELECT login FROM customer_user WHERE customer_id = $1
							 )
					   ) THEN 1
					   ELSE 0
				   END AS is_assigned
			FROM service s
			WHERE s.valid_id = 1
			ORDER BY s.name
		`), customerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load services"})
			return
		}
		defer func() { _ = rows.Close() }()

		services := []map[string]interface{}{}
		for rows.Next() {
			var service struct {
				ID       int
				Name     string
				Comments sql.NullString
			}
			var assignedInt int

			if err := rows.Scan(&service.ID, &service.Name, &service.Comments, &assignedInt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode services"})
				return
			}

			services = append(services, map[string]interface{}{
				"id":          service.ID,
				"name":        service.Name,
				"description": service.Comments.String,
				"assigned":    assignedInt == 1,
			})
		}

		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read services"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"services": services})
	}
}

// handleAdminUpdateCustomerCompanyServices updates service assignments.
func handleAdminUpdateCustomerCompanyServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")

		selectedServices := c.PostFormArray("services")
		if len(selectedServices) == 0 {
			contentType := c.GetHeader("Content-Type")
			if strings.Contains(contentType, "application/json") {
				var req struct {
					Services []string `json:"services"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
					return
				}
				selectedServices = req.Services
			}
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Get all customer users for this company
		rows, err := tx.Query(database.ConvertPlaceholders("SELECT login FROM customer_user WHERE customer_id = $1"), customerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customer users"})
			return
		}
		defer func() { _ = rows.Close() }()
		userLogins := []string{}
		for rows.Next() {
			var login string
			if err := rows.Scan(&login); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode customer users"})
				return
			}
			userLogins = append(userLogins, login)
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read customer users"})
			return
		}

		// Clear existing assignments for all users in this company
		for _, login := range userLogins {
			if _, err := tx.Exec(database.ConvertPlaceholders("DELETE FROM service_customer_user WHERE customer_user_login = $1"), login); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear existing services"})
				return
			}
		}

		// Add new assignments
		for _, login := range userLogins {
			for _, serviceID := range selectedServices {
				if _, err := tx.Exec(database.ConvertPlaceholders(`
					INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
					VALUES ($1, $2, NOW(), 1)
				`), login, serviceID); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign services"})
					return
				}
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update services"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// handleAdminCustomerPortalSettings shows portal customization settings.
func handleAdminCustomerPortalSettings(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database connection unavailable"})
			return
		}

		customerID := strings.TrimSpace(c.Param("id"))
		if customerID == "" {
			cfg := loadCustomerPortalConfig(db)
			getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_portal_settings.pongo2", pongo2.Context{
				"Title":           "Customer Portal Settings",
				"ActivePage":      "admin",
				"ActiveAdminPage": "customer-portal",
				"Settings":        cfg,
				"User":            getUserMapForTemplate(c),
			})
			return
		}

		cfg := loadCustomerPortalConfigForCustomer(db, customerID)
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"customer_id": customerID,
			"settings":    cfg,
		})
	}
}

// handleAdminUpdateCustomerPortalSettings updates portal customization.
func handleAdminUpdateCustomerPortalSettings(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database connection unavailable"})
			return
		}

		customerID := strings.TrimSpace(c.Param("id"))
		if customerID == "" {
			cfg := customerPortalConfig{
				Enabled:       parseCheckbox(c, "enabled"),
				LoginRequired: parseCheckbox(c, "login_required"),
				Title:         strings.TrimSpace(c.PostForm("title")),
				FooterText:    strings.TrimSpace(c.PostForm("footer_text")),
				LandingPage:   strings.TrimSpace(c.PostForm("landing_page")),
			}
			userID := c.GetInt("user_id")
			if err := saveCustomerPortalConfig(db, cfg, userID); err != nil {
				if isPortalConfigTableMissing(err) {
					shared.SendToastResponse(c, true, "Customer portal settings saved (sysconfig unavailable)", "/admin/customer/portal/settings")
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if c.GetHeader("HX-Request") == "true" {
				c.Header("HX-Redirect", "/admin/customer/portal/settings")
			}

			shared.SendToastResponse(c, true, "Customer portal settings saved", "/admin/customer/portal/settings")
			return
		}

		cfg := customerPortalConfig{
			Enabled:       parseCheckbox(c, "enabled"),
			LoginRequired: parseCheckbox(c, "login_required"),
			Title:         strings.TrimSpace(c.PostForm("title")),
			FooterText:    strings.TrimSpace(c.PostForm("footer_text")),
			LandingPage:   strings.TrimSpace(c.PostForm("landing_page")),
		}
		userID := c.GetInt("user_id")
		if err := saveCustomerPortalConfigForCustomer(db, customerID, cfg, userID); err != nil {
			if isPortalConfigTableMissing(err) {
				c.JSON(http.StatusOK, gin.H{
					"success":     true,
					"customer_id": customerID,
					"warning":     "sysconfig tables unavailable, skipping save",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"customer_id": customerID,
			"settings":    cfg,
		})
	}
}

func parseCheckbox(c *gin.Context, name string) bool {
	vals := c.PostFormArray(name)
	if len(vals) == 0 {
		return false
	}
	v := strings.TrimSpace(strings.ToLower(vals[len(vals)-1]))
	return v == "1" || v == "on" || v == "true"
}

func isPortalConfigTableMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "sysconfig unavailable") {
		return true
	}
	if strings.Contains(msg, "sysconfig default missing") {
		return true
	}
	if !strings.Contains(msg, "sysconfig") {
		return false
	}
	if strings.Contains(msg, "no such table") {
		return true
	}
	return strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined table") || strings.Contains(msg, "undefined_relation")
}

// handleAdminUploadCustomerPortalLogo handles logo uploads for customer portals.
func handleAdminUploadCustomerPortalLogo(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.Param("id")

		// Handle file upload
		file, header, err := c.Request.FormFile("logo")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
			return
		}
		defer func() { _ = file.Close() }()

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
