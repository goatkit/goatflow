package api

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAdminCustomerUsersList handles GET /admin/customer-users.
func HandleAdminCustomerUsersList(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Get search and filter parameters
	search := c.Query("search")
	validFilter := c.Query("valid")
	customerFilter := c.Query("customer")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	// Build query
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if search != "" {
		whereClause += fmt.Sprintf(" AND (cu.login ILIKE $%d OR cu.email ILIKE $%d OR cu.first_name ILIKE $%d OR cu.last_name ILIKE $%d OR cc.name ILIKE $%d)",
			argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4)
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
		argIndex += 5
	}

	if validFilter != "" {
		validID, _ := strconv.Atoi(validFilter)
		whereClause += fmt.Sprintf(" AND cu.valid_id = $%d", argIndex)
		args = append(args, validID)
		argIndex++
	}

	if customerFilter != "" {
		whereClause += fmt.Sprintf(" AND cu.customer_id = $%d", argIndex)
		args = append(args, customerFilter)
		argIndex++
	}

	// Main query
	query := fmt.Sprintf(`
		SELECT cu.id, cu.login, cu.email, cu.customer_id, cu.first_name, cu.last_name,
		       cu.phone, cu.city, cu.country, cu.valid_id,
		       cc.name as company_name,
		       cu.create_time,
		       (SELECT COUNT(*) FROM ticket WHERE customer_user_id = cu.login) as ticket_count
		FROM customer_user cu
		LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
		%s
		ORDER BY cu.last_name, cu.first_name
		LIMIT $%d OFFSET $%d`, whereClause, argIndex, argIndex+1)
	query = database.ConvertPlaceholders(query)

	args = append(args, limit, offset)

	// Count query for pagination
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM customer_user cu
		LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
		%s`, whereClause)
	countQuery = database.ConvertPlaceholders(countQuery)

	var totalCount int
	if len(args) > 2 { // Has filters
		countArgs := args[:len(args)-2] // Remove limit and offset
		err = db.QueryRow(countQuery, countArgs...).Scan(&totalCount)
	} else {
		err = db.QueryRow(countQuery).Scan(&totalCount)
	}

	if err != nil {
		totalCount = 0
	}

	// Execute main query
	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch customer users: " + err.Error(),
		})
		return
	}
	defer func() { _ = rows.Close() }()

	var customers []map[string]interface{}
	for rows.Next() {
		var customer = make(map[string]interface{})
		var companyName sql.NullString
		var firstName, lastName, phone, city, country sql.NullString
		var ticketCount int
		var id int
		var login, email, customerID string
		var validID int
		var createTime time.Time

		err := rows.Scan(
			&id,
			&login,
			&email,
			&customerID,
			&firstName,
			&lastName,
			&phone,
			&city,
			&country,
			&validID,
			&companyName,
			&createTime,
			&ticketCount,
		)

		if err != nil {
			continue
		}

		customer["id"] = id
		customer["login"] = login
		customer["email"] = email
		customer["customer_id"] = customerID
		customer["valid_id"] = validID
		customer["create_time"] = createTime
		customer["first_name"] = firstName.String
		customer["last_name"] = lastName.String
		customer["phone"] = phone.String
		customer["city"] = city.String
		customer["country"] = country.String
		customer["company_name"] = companyName.String
		customer["ticket_count"] = ticketCount
		customer["full_name"] = strings.TrimSpace(firstName.String + " " + lastName.String)

		customers = append(customers, customer)
	}

	// Get companies for filter dropdown
	companiesQuery := "SELECT DISTINCT customer_id, name FROM customer_company WHERE valid_id = 1 ORDER BY name"
	companyRows, _ := db.Query(companiesQuery)
	var companies []map[string]interface{}
	if companyRows != nil {
		defer companyRows.Close()
		for companyRows.Next() {
			var company = make(map[string]interface{})
			var companyCustomerID, companyName string
			companyRows.Scan(&companyCustomerID, &companyName)
			company["customer_id"] = companyCustomerID
			company["name"] = companyName
			companies = append(companies, company)
		}
	}

	// Check if this is an HTMX request
	if c.GetHeader("HX-Request") == "true" {
		// Return JSON for HTMX/AJAX requests
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    customers,
			"pagination": gin.H{
				"current_page": page,
				"total_count":  totalCount,
				"total_pages":  (totalCount + limit - 1) / limit,
				"per_page":     limit,
			},
		})
		return
	}

	// Render template for regular HTTP requests
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_users.pongo2", pongo2.Context{
		"User":           getUserFromContext(c),
		"ActivePage":     "admin",
		"Title":          "Customer User Management",
		"customers":      customers,
		"companies":      companies,
		"search":         search,
		"validFilter":    validFilter,
		"customerFilter": customerFilter,
		"totalCount":     totalCount,
		"currentPage":    page,
		"totalPages":     (totalCount + limit - 1) / limit,
		"perPage":        limit,
	})
}

// HandleAdminCustomerUsersGet handles GET /admin/customer-users/:id.
func HandleAdminCustomerUsersGet(c *gin.Context) {
	customerID := c.Param("id")
	id, err := strconv.Atoi(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid customer user ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Get customer user details
	query := `
		SELECT cu.id, cu.login, cu.email, cu.customer_id, cu.pw, cu.title, 
		       cu.first_name, cu.last_name, cu.phone, cu.fax, cu.mobile, 
		       cu.street, cu.zip, cu.city, cu.country, cu.comments, cu.valid_id,
		       cc.name as company_name
		FROM customer_user cu
		LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
		WHERE cu.id = $1`
	query = database.ConvertPlaceholders(query)

	var customer = make(map[string]interface{})
	var companyName sql.NullString
	var pw, title, firstName, lastName, phone, fax, mobile sql.NullString
	var street, zip, city, country, comments sql.NullString
	var custID int
	var login, email, customerIDFromDB string
	var validID int

	err = db.QueryRow(query, id).Scan(
		&custID,
		&login,
		&email,
		&customerIDFromDB,
		&pw,
		&title,
		&firstName,
		&lastName,
		&phone,
		&fax,
		&mobile,
		&street,
		&zip,
		&city,
		&country,
		&comments,
		&validID,
		&companyName,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Customer user not found",
		})
		return
	}

	customer["id"] = custID
	customer["login"] = login
	customer["email"] = email
	customer["customer_id"] = customerIDFromDB
	customer["valid_id"] = validID
	customer["pw"] = pw.String
	customer["title"] = title.String
	customer["first_name"] = firstName.String
	customer["last_name"] = lastName.String
	customer["phone"] = phone.String
	customer["fax"] = fax.String
	customer["mobile"] = mobile.String
	customer["street"] = street.String
	customer["zip"] = zip.String
	customer["city"] = city.String
	customer["country"] = country.String
	customer["comments"] = comments.String
	customer["company_name"] = companyName.String

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    customer,
	})
}

// HandleAdminCustomerUsersCreate handles POST /admin/customer-users.
func HandleAdminCustomerUsersCreate(c *gin.Context) {
	var req struct {
		Login      string `json:"login" form:"login" binding:"required"`
		Email      string `json:"email" form:"email" binding:"required,email"`
		CustomerID string `json:"customer_id" form:"customer_id" binding:"required"`
		Password   string `json:"password" form:"password"`
		Title      string `json:"title" form:"title"`
		FirstName  string `json:"first_name" form:"first_name"`
		LastName   string `json:"last_name" form:"last_name"`
		Phone      string `json:"phone" form:"phone"`
		Fax        string `json:"fax" form:"fax"`
		Mobile     string `json:"mobile" form:"mobile"`
		Street     string `json:"street" form:"street"`
		Zip        string `json:"zip" form:"zip"`
		City       string `json:"city" form:"city"`
		Country    string `json:"country" form:"country"`
		Comments   string `json:"comments" form:"comments"`
		ValidID    int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Set default valid_id if not provided
	if req.ValidID == 0 {
		req.ValidID = 1
	}

	// Check if login already exists
	var existingID int
	checkQuery := database.ConvertPlaceholders("SELECT id FROM customer_user WHERE login = $1")
	err = db.QueryRow(checkQuery, req.Login).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Login already exists",
		})
		return
	}

	// Create customer user
	insertQuery := `
		INSERT INTO customer_user (
			login, email, customer_id, pw, title, first_name, last_name,
			phone, fax, mobile, street, zip, city, country, comments,
			valid_id, create_by, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, 1, 1
		) RETURNING id`
	insertQuery = database.ConvertPlaceholders(insertQuery)
	var newID int
	err = db.QueryRow(insertQuery,
		req.Login, req.Email, req.CustomerID, req.Password, req.Title,
		req.FirstName, req.LastName, req.Phone, req.Fax, req.Mobile,
		req.Street, req.Zip, req.City, req.Country, req.Comments, req.ValidID,
	).Scan(&newID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create customer user: " + err.Error(),
		})
		return
	}

	// Check if this is a form submission (redirect) or API call (JSON)
	if c.GetHeader("Content-Type") == "application/x-www-form-urlencoded" ||
		c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/customer-users")
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"message": "Customer user created successfully",
			"id":      newID,
		})
	} else {
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"message": "Customer user created successfully",
			"id":      newID,
		})
	}
}

// HandleAdminCustomerUsersUpdate handles PUT /admin/customer-users/:id.
func HandleAdminCustomerUsersUpdate(c *gin.Context) {
	customerID := c.Param("id")
	id, err := strconv.Atoi(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid customer user ID",
		})
		return
	}

	var req struct {
		Login      string `json:"login" form:"login" binding:"required"`
		Email      string `json:"email" form:"email" binding:"required,email"`
		CustomerID string `json:"customer_id" form:"customer_id" binding:"required"`
		Password   string `json:"password" form:"password"`
		Title      string `json:"title" form:"title"`
		FirstName  string `json:"first_name" form:"first_name"`
		LastName   string `json:"last_name" form:"last_name"`
		Phone      string `json:"phone" form:"phone"`
		Fax        string `json:"fax" form:"fax"`
		Mobile     string `json:"mobile" form:"mobile"`
		Street     string `json:"street" form:"street"`
		Zip        string `json:"zip" form:"zip"`
		City       string `json:"city" form:"city"`
		Country    string `json:"country" form:"country"`
		Comments   string `json:"comments" form:"comments"`
		ValidID    int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Check if login exists for another user
	var existingID int
	checkQuery := database.ConvertPlaceholders("SELECT id FROM customer_user WHERE login = $1 AND id != $2")
	err = db.QueryRow(checkQuery, req.Login, id).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Login already exists for another user",
		})
		return
	}

	// Build update query
	updateQuery := `
		UPDATE customer_user SET 
			login = $1, email = $2, customer_id = $3, title = $4,
			first_name = $5, last_name = $6, phone = $7, fax = $8,
			mobile = $9, street = $10, zip = $11, city = $12,
			country = $13, comments = $14, valid_id = $15,
			change_time = CURRENT_TIMESTAMP, change_by = 1`

	args := []interface{}{
		req.Login, req.Email, req.CustomerID, req.Title,
		req.FirstName, req.LastName, req.Phone, req.Fax,
		req.Mobile, req.Street, req.Zip, req.City,
		req.Country, req.Comments, req.ValidID,
	}

	// Add password if provided
	if req.Password != "" {
		updateQuery += ", pw = $16 WHERE id = $17"
		args = append(args, req.Password, id)
	} else {
		updateQuery += " WHERE id = $16"
		args = append(args, id)
	}
	updateQuery = database.ConvertPlaceholders(updateQuery)

	result, err := db.Exec(updateQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update customer user: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Customer user not found",
		})
		return
	}

	// Check if this is a form submission (redirect) or API call (JSON)
	if c.GetHeader("Content-Type") == "application/x-www-form-urlencoded" ||
		c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/customer-users")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Customer user updated successfully",
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Customer user updated successfully",
		})
	}
}

// HandleAdminCustomerUsersDelete handles DELETE /admin/customer-users/:id (soft delete).
func HandleAdminCustomerUsersDelete(c *gin.Context) {
	customerID := c.Param("id")
	id, err := strconv.Atoi(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid customer user ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Soft delete by setting valid_id to 2 (invalid)
	updateQuery := `
		UPDATE customer_user 
		SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = 1
		WHERE id = $1`
	updateQuery = database.ConvertPlaceholders(updateQuery)

	result, err := db.Exec(updateQuery, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete customer user: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Customer user not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Customer user deleted successfully",
	})
}

// HandleAdminCustomerUsersTickets handles GET /admin/customer-users/:id/tickets.
func HandleAdminCustomerUsersTickets(c *gin.Context) {
	customerID := c.Param("id")
	id, err := strconv.Atoi(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid customer user ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Get customer login first
	var customerLogin string
	err = db.QueryRow(database.ConvertPlaceholders("SELECT login FROM customer_user WHERE id = $1"), id).Scan(&customerLogin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Customer user not found",
		})
		return
	}

	// Get tickets for this customer
	query := `
		SELECT t.id, t.title, t.ticket_number, t.create_time,
		       ts.name as state, tp.name as priority, q.name as queue
		FROM ticket t
		LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
		LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
		LEFT JOIN queue q ON t.queue_id = q.id
		WHERE t.customer_user_id = $1
		ORDER BY t.create_time DESC
		LIMIT 100`
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, customerLogin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch tickets: " + err.Error(),
		})
		return
	}
	defer func() { _ = rows.Close() }()

	var tickets []map[string]interface{}
	for rows.Next() {
		var ticket = make(map[string]interface{})
		var state, priority, queue sql.NullString
		var ticketID int
		var title, ticketNumber string
		var createTime time.Time

		err := rows.Scan(
			&ticketID,
			&title,
			&ticketNumber,
			&createTime,
			&state,
			&priority,
			&queue,
		)

		if err != nil {
			continue
		}

		ticket["id"] = ticketID
		ticket["title"] = title
		ticket["ticket_number"] = ticketNumber
		ticket["create_time"] = createTime
		ticket["state"] = state.String
		ticket["priority"] = priority.String
		ticket["queue"] = queue.String

		tickets = append(tickets, ticket)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tickets,
	})
}

// HandleAdminCustomerUsersImportForm handles GET /admin/customer-users/import.
func HandleAdminCustomerUsersImportForm(c *gin.Context) {
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_users_import.pongo2", pongo2.Context{
		"User":       getUserFromContext(c),
		"ActivePage": "admin",
		"Title":      "Import Customer Users",
	})
}

// HandleAdminCustomerUsersImport handles POST /admin/customer-users/import.
func HandleAdminCustomerUsersImport(c *gin.Context) {
	file, _, err := c.Request.FormFile("csv_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No file uploaded",
		})
		return
	}
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to parse CSV: " + err.Error(),
		})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "CSV must contain at least a header row and one data row",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Parse header row to get column mapping
	headers := records[0]
	columnMap := make(map[string]int)
	for i, header := range headers {
		columnMap[strings.ToLower(strings.TrimSpace(header))] = i
	}

	var imported, failed int
	var errors []string

	// Process data rows
	for i := 1; i < len(records); i++ {
		record := records[i]

		// Helper function to get column value safely
		getColumn := func(name string) string {
			if idx, exists := columnMap[name]; exists && idx < len(record) {
				return strings.TrimSpace(record[idx])
			}
			return ""
		}

		login := getColumn("login")
		email := getColumn("email")
		customerID := getColumn("customer_id")

		if login == "" || email == "" || customerID == "" {
			errors = append(errors, fmt.Sprintf("Row %d: Missing required fields (login, email, customer_id)", i+1))
			failed++
			continue
		}

		// Check if customer user already exists
		var existingID int
		checkQuery := database.ConvertPlaceholders("SELECT id FROM customer_user WHERE login = $1")
		err = db.QueryRow(checkQuery, login).Scan(&existingID)
		if err == nil {
			errors = append(errors, fmt.Sprintf("Row %d: Login %s already exists", i+1, login))
			failed++
			continue
		}

		// Insert customer user
		insertQuery := `
			INSERT INTO customer_user (
				login, email, customer_id, title, first_name, last_name,
				phone, fax, mobile, street, zip, city, country, comments,
				valid_id, create_by, change_by
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 1, 1, 1
			)`
		insertQuery = database.ConvertPlaceholders(insertQuery)

		_, err = db.Exec(insertQuery,
			login,
			email,
			customerID,
			getColumn("title"),
			getColumn("first_name"),
			getColumn("last_name"),
			getColumn("phone"),
			getColumn("fax"),
			getColumn("mobile"),
			getColumn("street"),
			getColumn("zip"),
			getColumn("city"),
			getColumn("country"),
			getColumn("comments"),
		)

		if err != nil {
			errors = append(errors, fmt.Sprintf("Row %d: Database error: %s", i+1, err.Error()))
			failed++
		} else {
			imported++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  fmt.Sprintf("Import completed: %d imported, %d failed", imported, failed),
		"imported": imported,
		"failed":   failed,
		"errors":   errors,
	})
}

// HandleAdminCustomerUsersExport handles GET /admin/customer-users/export.
func HandleAdminCustomerUsersExport(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	query := `
		SELECT cu.login, cu.email, cu.customer_id, cu.title, cu.first_name, cu.last_name,
		       cu.phone, cu.fax, cu.mobile, cu.street, cu.zip, cu.city, cu.country,
		       cu.comments, cu.valid_id, cc.name as company_name
		FROM customer_user cu
		LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
		ORDER BY cu.last_name, cu.first_name`

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch customer users: " + err.Error(),
		})
		return
	}
	defer func() { _ = rows.Close() }()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=customer_users.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write header
	header := []string{
		"login", "email", "customer_id", "title", "first_name", "last_name",
		"phone", "fax", "mobile", "street", "zip", "city", "country",
		"comments", "valid_id", "company_name",
	}
	writer.Write(header)

	// Write data
	for rows.Next() {
		var record = make([]string, 16)
		var nullableFields = make([]*sql.NullString, 16)

		for i := range nullableFields {
			nullableFields[i] = &sql.NullString{}
		}

		err := rows.Scan(
			&record[0],         // login
			&record[1],         // email
			&record[2],         // customer_id
			nullableFields[3],  // title
			nullableFields[4],  // first_name
			nullableFields[5],  // last_name
			nullableFields[6],  // phone
			nullableFields[7],  // fax
			nullableFields[8],  // mobile
			nullableFields[9],  // street
			nullableFields[10], // zip
			nullableFields[11], // city
			nullableFields[12], // country
			nullableFields[13], // comments
			&record[14],        // valid_id
			nullableFields[15], // company_name
		)

		if err != nil {
			continue
		}

		// Convert nullable fields to strings
		for i := 3; i < 16; i++ {
			if i == 14 { // valid_id is not nullable
				continue
			}
			if nullableFields[i].Valid {
				record[i] = nullableFields[i].String
			} else {
				record[i] = ""
			}
		}

		writer.Write(record)
	}
}

// HandleAdminCustomerUsersBulkAction handles POST /admin/customer-users/bulk-action.
func HandleAdminCustomerUsersBulkAction(c *gin.Context) {
	var req struct {
		Action string   `json:"action" form:"action" binding:"required"`
		IDs    []string `json:"ids" form:"ids" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No customer users selected",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	var setClause string
	var message string

	switch req.Action {
	case "enable":
		setClause = "valid_id = 1, change_time = CURRENT_TIMESTAMP, change_by = 1"
		message = "Customer users enabled successfully"
	case "disable":
		setClause = "valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = 1"
		message = "Customer users disabled successfully"
	case "delete":
		setClause = "valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = 1"
		message = "Customer users deleted successfully"
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid action",
		})
		return
	}

	// Convert string IDs to integers for validation
	var intIDs []int
	for _, idStr := range req.IDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid customer user ID: " + idStr,
			})
			return
		}
		intIDs = append(intIDs, id)
	}

	placeholders := make([]string, len(intIDs))
	args := make([]interface{}, len(intIDs))
	for i, id := range intIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf("UPDATE customer_user SET %s WHERE id IN (%s)", setClause, strings.Join(placeholders, ","))
	query = database.ConvertPlaceholders(query)

	result, err := db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to perform bulk action: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       message,
		"rows_affected": rowsAffected,
	})
}
