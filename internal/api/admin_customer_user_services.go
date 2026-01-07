package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// handleAdminCustomerUserServices renders the customer user services management page.
func handleAdminCustomerUserServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			var err error
			db, err = database.GetDB()
			if err != nil || db == nil {
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.String(http.StatusOK, `<!DOCTYPE html><html><head><title>Customer User Services</title></head><body>
<h1>Customer User Services</h1>
<p>Database not available</p>
</body></html>`)
				return
			}
		}

		search := c.Query("search")
		view := c.DefaultQuery("view", "customers") // "customers" or "services"

		var customerUsers []map[string]interface{}
		var services []map[string]interface{}

		// Get customer users with service count
		customerQuery := `
			SELECT cu.login, cu.first_name, cu.last_name, cu.email, cu.customer_id,
			       cc.name as company_name,
			       (SELECT COUNT(*) FROM service_customer_user WHERE customer_user_login = cu.login) as service_count
			FROM customer_user cu
			LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
			WHERE cu.valid_id = 1
		`
		if search != "" {
			customerQuery += fmt.Sprintf(` AND (
				LOWER(cu.login) LIKE LOWER('%%%s%%') OR 
				LOWER(cu.first_name) LIKE LOWER('%%%s%%') OR 
				LOWER(cu.last_name) LIKE LOWER('%%%s%%') OR 
				LOWER(cu.email) LIKE LOWER('%%%s%%') OR
				LOWER(cc.name) LIKE LOWER('%%%s%%')
			)`, search, search, search, search, search)
		}
		customerQuery += " ORDER BY cu.last_name, cu.first_name LIMIT 100"

		rows, err := db.Query(database.ConvertPlaceholders(customerQuery))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var login, firstName, lastName, email string
				var customerID, companyName sql.NullString
				var serviceCount int
				if err := rows.Scan(&login, &firstName, &lastName, &email, &customerID, &companyName, &serviceCount); err == nil {
					customerUsers = append(customerUsers, map[string]interface{}{
						"login":         login,
						"first_name":    firstName,
						"last_name":     lastName,
						"email":         email,
						"customer_id":   customerID.String,
						"company_name":  companyName.String,
						"service_count": serviceCount,
					})
				}
			}
			if err := rows.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating customer users"})
				return
			}
		}

		// Get services with customer count
		serviceQuery := `
			SELECT s.id, s.name, s.comments,
			       (SELECT COUNT(*) FROM service_customer_user WHERE service_id = s.id) as customer_count
			FROM service s
			WHERE s.valid_id = 1
			ORDER BY s.name
		`
		sRows, err := db.Query(database.ConvertPlaceholders(serviceQuery))
		if err == nil {
			defer sRows.Close()
			for sRows.Next() {
				var id int
				var name string
				var comments sql.NullString
				var customerCount int
				if err := sRows.Scan(&id, &name, &comments, &customerCount); err == nil {
					services = append(services, map[string]interface{}{
						"id":             id,
						"name":           name,
						"comments":       comments.String,
						"customer_count": customerCount,
					})
				}
			}
			if err := sRows.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating services"})
				return
			}
		}

		if getPongo2Renderer() == nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, `<h1>Customer User Services</h1><p>Template renderer not available</p>`)
			return
		}

		// Get count of default services
		defaultServicesCount := getDefaultServicesCount(db)

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_user_services.pongo2", pongo2.Context{
			"Title":                "Customer User Services",
			"CustomerUsers":        customerUsers,
			"Services":             services,
			"Search":               search,
			"View":                 view,
			"DefaultServicesCount": defaultServicesCount,
			"User":                 getUserMapForTemplate(c),
			"ActivePage":           "admin",
		})
	}
}

// handleAdminCustomerUserServicesAllocate shows service allocation for a specific customer user.
func handleAdminCustomerUserServicesAllocate(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			var err error
			db, err = database.GetDB()
			if err != nil || db == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
				return
			}
		}

		customerUserLogin := c.Param("login")

		// Get customer user details
		var customerUser struct {
			Login      string
			FirstName  string
			LastName   string
			Email      string
			CustomerID sql.NullString
		}
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT login, first_name, last_name, email, customer_id
			FROM customer_user WHERE login = ?
		`), customerUserLogin).Scan(
			&customerUser.Login, &customerUser.FirstName, &customerUser.LastName,
			&customerUser.Email, &customerUser.CustomerID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer user not found"})
			return
		}

		// Get all services and their assignment status for this customer
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT s.id, s.name, s.comments,
			       CASE WHEN scu.service_id IS NOT NULL THEN 1 ELSE 0 END as assigned
			FROM service s
			LEFT JOIN service_customer_user scu ON s.id = scu.service_id AND scu.customer_user_login = ?
			WHERE s.valid_id = 1
			ORDER BY s.name
		`), customerUserLogin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load services"})
			return
		}
		defer rows.Close()

		services := []map[string]interface{}{}
		for rows.Next() {
			var id, assigned int
			var name string
			var comments sql.NullString
			if err := rows.Scan(&id, &name, &comments, &assigned); err == nil {
				services = append(services, map[string]interface{}{
					"id":       id,
					"name":     name,
					"comments": comments.String,
					"assigned": assigned == 1,
				})
			}
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating services"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"customer_user": map[string]interface{}{
				"login":       customerUser.Login,
				"first_name":  customerUser.FirstName,
				"last_name":   customerUser.LastName,
				"email":       customerUser.Email,
				"customer_id": customerUser.CustomerID.String,
			},
			"services": services,
		})
	}
}

// handleAdminCustomerUserServicesUpdate updates service assignments for a customer user.
func handleAdminCustomerUserServicesUpdate(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			var err error
			db, err = database.GetDB()
			if err != nil || db == nil {
				shared.SendToastResponse(c, false, "Database not available", "")
				return
			}
		}

		customerUserLogin := c.Param("login")

		// Get selected services
		var selectedServices []string
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var req struct {
				Services []string `json:"services"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				shared.SendToastResponse(c, false, "Invalid request", "")
				return
			}
			selectedServices = req.Services
		} else {
			selectedServices = c.PostFormArray("services")
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			shared.SendToastResponse(c, false, "Transaction failed", "")
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Clear existing assignments
		deleteQuery := "DELETE FROM service_customer_user WHERE customer_user_login = ?"
		_, err = tx.Exec(database.ConvertPlaceholders(deleteQuery), customerUserLogin)
		if err != nil {
			shared.SendToastResponse(c, false, "Failed to clear existing services", "")
			return
		}

		// Add new assignments
		for _, serviceID := range selectedServices {
			_, err = tx.Exec(database.ConvertPlaceholders(`
				INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
				VALUES (?, ?, NOW(), 1)
			`), customerUserLogin, serviceID)
			if err != nil {
				shared.SendToastResponse(c, false, "Failed to assign service", "")
				return
			}
		}

		if err := tx.Commit(); err != nil {
			shared.SendToastResponse(c, false, "Failed to save changes", "")
			return
		}

		shared.SendToastResponse(c, true, "Services updated successfully", "")
	}
}

// handleAdminServiceCustomerUsersAllocate shows customer user allocation for a specific service.
func handleAdminServiceCustomerUsersAllocate(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			var err error
			db, err = database.GetDB()
			if err != nil || db == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
				return
			}
		}

		serviceID := c.Param("id")

		// Get service details
		var service struct {
			ID       int
			Name     string
			Comments sql.NullString
		}
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT id, name, comments FROM service WHERE id = ?
		`), serviceID).Scan(&service.ID, &service.Name, &service.Comments)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
			return
		}

		// Get all customer users and their assignment status for this service
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT cu.login, cu.first_name, cu.last_name, cu.email, cu.customer_id,
			       cc.name as company_name,
			       CASE WHEN scu.customer_user_login IS NOT NULL THEN 1 ELSE 0 END as assigned
			FROM customer_user cu
			LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
			LEFT JOIN service_customer_user scu ON cu.login = scu.customer_user_login AND scu.service_id = ?
			WHERE cu.valid_id = 1
			ORDER BY cu.last_name, cu.first_name
		`), serviceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customer users"})
			return
		}
		defer rows.Close()

		customerUsers := []map[string]interface{}{}
		for rows.Next() {
			var login, firstName, lastName, email string
			var customerID, companyName sql.NullString
			var assigned int
			if err := rows.Scan(&login, &firstName, &lastName, &email, &customerID, &companyName, &assigned); err == nil {
				customerUsers = append(customerUsers, map[string]interface{}{
					"login":        login,
					"first_name":   firstName,
					"last_name":    lastName,
					"email":        email,
					"customer_id":  customerID.String,
					"company_name": companyName.String,
					"assigned":     assigned == 1,
				})
			}
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating customer users"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"service": map[string]interface{}{
				"id":       service.ID,
				"name":     service.Name,
				"comments": service.Comments.String,
			},
			"customer_users": customerUsers,
		})
	}
}

// handleAdminServiceCustomerUsersUpdate updates customer user assignments for a service.
func handleAdminServiceCustomerUsersUpdate(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			var err error
			db, err = database.GetDB()
			if err != nil || db == nil {
				shared.SendToastResponse(c, false, "Database not available", "")
				return
			}
		}

		serviceID := c.Param("id")

		// Get selected customer users
		var selectedUsers []string
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var req struct {
				CustomerUsers []string `json:"customer_users"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				shared.SendToastResponse(c, false, "Invalid request", "")
				return
			}
			selectedUsers = req.CustomerUsers
		} else {
			selectedUsers = c.PostFormArray("customer_users")
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			shared.SendToastResponse(c, false, "Transaction failed", "")
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Clear existing assignments for this service
		_, err = tx.Exec(database.ConvertPlaceholders("DELETE FROM service_customer_user WHERE service_id = ?"), serviceID)
		if err != nil {
			shared.SendToastResponse(c, false, "Failed to clear existing assignments", "")
			return
		}

		// Add new assignments
		for _, userLogin := range selectedUsers {
			_, err = tx.Exec(database.ConvertPlaceholders(`
				INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
				VALUES (?, ?, NOW(), 1)
			`), userLogin, serviceID)
			if err != nil {
				shared.SendToastResponse(c, false, "Failed to assign customer user", "")
				return
			}
		}

		if err := tx.Commit(); err != nil {
			shared.SendToastResponse(c, false, "Failed to save changes", "")
			return
		}

		shared.SendToastResponse(c, true, "Customer users updated successfully", "")
	}
}

// getDefaultServicesCount returns the count of default services configured.
func getDefaultServicesCount(db *sql.DB) int {
	var count int
	row := db.QueryRow(database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM service_customer_user WHERE customer_user_login = '<DEFAULT>'
	`))
	_ = row.Scan(&count) //nolint:errcheck // Count defaults to 0 on error
	return count
}

// handleAdminDefaultServices shows the default services allocation page.
func handleAdminDefaultServices(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			var err error
			db, err = database.GetDB()
			if err != nil || db == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
				return
			}
		}

		// Get all services and their default assignment status
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT s.id, s.name, s.comments,
			       CASE WHEN scu.service_id IS NOT NULL THEN 1 ELSE 0 END as assigned
			FROM service s
			LEFT JOIN service_customer_user scu ON s.id = scu.service_id AND scu.customer_user_login = '<DEFAULT>'
			WHERE s.valid_id = 1
			ORDER BY s.name
		`))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load services"})
			return
		}
		defer rows.Close()

		services := []map[string]interface{}{}
		for rows.Next() {
			var id, assigned int
			var name string
			var comments sql.NullString
			if err := rows.Scan(&id, &name, &comments, &assigned); err == nil {
				services = append(services, map[string]interface{}{
					"id":       id,
					"name":     name,
					"comments": comments.String,
					"assigned": assigned == 1,
				})
			}
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating services"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"services": services,
		})
	}
}

// handleAdminDefaultServicesUpdate updates default service assignments.
func handleAdminDefaultServicesUpdate(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			var err error
			db, err = database.GetDB()
			if err != nil || db == nil {
				shared.SendToastResponse(c, false, "Database not available", "")
				return
			}
		}

		// Get selected services
		var selectedServices []string
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var req struct {
				Services []string `json:"services"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				shared.SendToastResponse(c, false, "Invalid request", "")
				return
			}
			selectedServices = req.Services
		} else {
			selectedServices = c.PostFormArray("services")
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			shared.SendToastResponse(c, false, "Transaction failed", "")
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Clear existing default service assignments
		_, err = tx.Exec(database.ConvertPlaceholders("DELETE FROM service_customer_user WHERE customer_user_login = '<DEFAULT>'"))
		if err != nil {
			shared.SendToastResponse(c, false, "Failed to clear existing default services", "")
			return
		}

		// Add new default service assignments
		for _, serviceID := range selectedServices {
			_, err = tx.Exec(database.ConvertPlaceholders(`
				INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
				VALUES ('<DEFAULT>', ?, NOW(), 1)
			`), serviceID)
			if err != nil {
				shared.SendToastResponse(c, false, "Failed to assign default service", "")
				return
			}
		}

		if err := tx.Commit(); err != nil {
			shared.SendToastResponse(c, false, "Failed to save changes", "")
			return
		}

		shared.SendToastResponse(c, true, "Default services updated successfully", "")
	}
}
