package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
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
			sendErrorResponse(c, http.StatusInternalServerError, "Database not available")
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

		var args []interface{}

		if search != "" {
			searchTerm := "%" + search + "%"
			query += " AND (LOWER(cc.name) LIKE LOWER(?) OR LOWER(cc.customer_id) LIKE LOWER(?) OR LOWER(cc.city) LIKE LOWER(?))"
			args = append(args, searchTerm, searchTerm, searchTerm)
		}

		switch validFilter {
		case "valid":
			query += " AND cc.valid_id = ?"
			args = append(args, 1)
		case "invalid":
			query += " AND cc.valid_id != ?"
			args = append(args, 1)
		}

		query += " ORDER BY cc.name"

		rows, err := db.Query(database.ConvertQuery(query), args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load customer companies"})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read customer companies"})
				return
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read customer companies"})
			return
		}

		if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
			sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
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
		actorID, ok := auditUserID(c)
		if !ok {
			return
		}

		// Check if customer ID already exists
		var exists bool
		if err := db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM customer_company WHERE customer_id = ?)"), customerID).Scan(&exists); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
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
				?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), 
				NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
				1, NOW(), ?, NOW(), ?
			)
		`), customerID, name, street, zip, city, country, url, comments, actorID, actorID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create customer company"})
			return
		}

		redirectURL := fmt.Sprintf("/admin/customer/companies/%s/edit", customerID)
		shared.SendToastResponse(c, true, "Customer company created successfully", redirectURL)
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
			WHERE customer_id = ?
		`), customerID).Scan(&company.CustomerID, &company.Name, &company.Street,
			&company.Zip, &company.City, &company.Country, &company.URL,
			&company.Comments, &company.ValidID)

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer company not found"})
			return
		}

		// Portal settings: load the company's effective config (overrides
		// merged onto global defaults) and the global defaults separately
		// so the template can show what each field will resolve to when
		// its override toggle is off.
		portalEffective := loadCustomerPortalConfigForCustomer(db, customerID)
		portalDefaults := loadCustomerPortalConfig(db)
		portalOverrides := sysconfig.CustomerPortalOverrides(db, customerID)

		// Resolve the linked gk_organisation + plugin-capture state so the
		// Portal tab can render the capture section. A customer_company
		// may not have a linked gk_organisation yet (the two tables are
		// joined by the customer_company_id FK on gk_organisation), in
		// which case we pass empty values and the tab shows a "link an
		// organisation first" hint.
		var (
			linkedOrgID    int64
			captivePlugin  string
			enabledPlugins []string
		)
		_ = db.QueryRow(database.ConvertPlaceholders(
			`SELECT id FROM gk_organisation WHERE customer_company_id = ? LIMIT 1`),
			customerID).Scan(&linkedOrgID)
		if linkedOrgID > 0 {
			var cp sql.NullString
			_ = db.QueryRow(database.ConvertPlaceholders(
				`SELECT captive_plugin FROM gk_organisation WHERE id = ?`),
				linkedOrgID).Scan(&cp)
			if cp.Valid {
				captivePlugin = cp.String
			}
			// Enabled plugins for this org — feeds the dropdown. One row
			// per (plugin_name, group), so collapse to distinct names
			// since the capture UI only cares about the plugin.
			rows, err := db.Query(database.ConvertPlaceholders(
				`SELECT DISTINCT plugin_name FROM gk_org_plugin_access WHERE org_id = ? ORDER BY plugin_name`),
				linkedOrgID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var n string
					if err := rows.Scan(&n); err == nil {
						enabledPlugins = append(enabledPlugins, n)
					}
				}
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
			"PortalEffective": portalEffective,
			"PortalDefaults":  portalDefaults,
			"PortalOverrides": portalOverrides,
			"LinkedOrgID":     linkedOrgID,
			"CaptivePlugin":   captivePlugin,
			"EnabledPlugins":  enabledPlugins,
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
				errHTML := `<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded" ` +
					`role="alert">Name is required</div>`
				c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(errHTML))
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
			}
			return
		}

		// Check if company exists first
		var exists bool
		var err error
		if db != nil {
			query := database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM customer_company WHERE customer_id = ?)")
			err = db.QueryRow(query, customerID).Scan(&exists)
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
		actorID, ok := auditUserID(c)
		if !ok {
			return
		}

		// Update company. Existence was checked above; RowsAffected is not consulted because
		// MySQL reports changed rows (0 for a same-second no-op) while PostgreSQL reports matched rows.
		_, err = db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company SET
				name = ?, street = NULLIF(?, ''), zip = NULLIF(?, ''),
				city = NULLIF(?, ''), country = NULLIF(?, ''),
				url = NULLIF(?, ''), comments = NULLIF(?, ''),
				valid_id = ?, change_time = NOW(), change_by = ?
			WHERE customer_id = ?
		`), name, street, zip, city, country, url, comments, validID, actorID, customerID)

		if err != nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Failed to update customer company", "")
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update customer company"})
			}
			return
		}

		redirectURL := fmt.Sprintf("/admin/customer/companies/%s/edit", customerID)
		shared.SendToastResponse(c, true, "Customer company updated successfully", redirectURL)
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
		actorID, ok := auditUserID(c)
		if !ok {
			return
		}

		// Soft delete by setting valid_id to 2 (invalid)
		result, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company
			SET valid_id = 2, change_time = NOW(), change_by = ?
			WHERE customer_id = ?
		`), actorID, customerID)

		if err != nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Failed to delete customer company", "")
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete customer company"})
			}
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			rowsAffected = 0
		}
		if rowsAffected == 0 && !customerCompanyExists(db, customerID) {
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
		actorID, ok := auditUserID(c)
		if !ok {
			return
		}

		// Activate by setting valid_id to 1 (valid)
		result, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_company
			SET valid_id = 1, change_time = NOW(), change_by = ?
			WHERE customer_id = ?
		`), actorID, customerID)

		if err != nil {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Failed to activate customer company", "")
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to activate customer company"})
			}
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			rowsAffected = 0
		}
		if rowsAffected == 0 && !customerCompanyExists(db, customerID) {
			if c.GetHeader("HX-Request") == "true" {
				shared.SendToastResponse(c, false, "Customer company not found", "")
			} else {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Customer company not found"})
			}
			return
		}

		redirectURL := fmt.Sprintf("/admin/customer/companies/%s/edit", customerID)
		shared.SendToastResponse(c, true, "Customer company activated successfully", redirectURL)
	}
}

// customerCompanyExists reports whether the company row exists. Used after an
// UPDATE affected 0 rows: MySQL counts changed rows, so a no-op update of an
// existing row also reports 0.
func customerCompanyExists(db *sql.DB, customerID string) bool {
	var exists bool
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT EXISTS(SELECT 1 FROM customer_company WHERE customer_id = ?)"), customerID).Scan(&exists)
	return err == nil && exists
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if c.GetHeader("HX-Request") == "true" {
				c.Header("HX-Redirect", "/admin/customer/portal/settings")
			}

			shared.SendToastResponse(c, true, "Customer portal settings saved", "/admin/customer/portal/settings")
			return
		}

		// Per-company submission: save only the fields the admin ticked
		// "Override default" for; every other field gets its per-company
		// sysconfig row deleted so reads fall back to the global
		// defaults.
		userID := c.GetInt("user_id")
		baseline := loadCustomerPortalConfig(db)
		cfg := baseline
		// The company form posts every override_* flag (hidden "0" followed by
		// the checkbox "1" when ticked), so the last value is the state.
		overrides := map[string]bool{
			"enabled": parseCheckbox(c, "override_enabled"),
			"login":   parseCheckbox(c, "override_login_required"),
			"title":   parseCheckbox(c, "override_title"),
			"footer":  parseCheckbox(c, "override_footer_text"),
			"landing": parseCheckbox(c, "override_landing_page"),
		}
		hasOverrideControls := false
		for _, name := range []string{"override_enabled", "override_login_required", "override_title", "override_footer_text", "override_landing_page"} {
			if _, ok := c.Request.PostForm[name]; ok {
				hasOverrideControls = true
				break
			}
		}
		if !hasOverrideControls {
			// Backwards-compatible API shape: older callers posted the
			// effective field values directly, before the UI gained explicit
			// per-field override checkboxes.
			for key, field := range map[string]string{
				"enabled": "enabled",
				"login":   "login_required",
				"title":   "title",
				"footer":  "footer_text",
				"landing": "landing_page",
			} {
				if _, ok := c.Request.PostForm[field]; ok {
					overrides[key] = true
				}
			}
		}
		if overrides["enabled"] {
			cfg.Enabled = parseCheckbox(c, "enabled")
		}
		if overrides["login"] {
			cfg.LoginRequired = parseCheckbox(c, "login_required")
		}
		if overrides["title"] {
			cfg.Title = strings.TrimSpace(c.PostForm("title"))
		}
		if overrides["footer"] {
			cfg.FooterText = strings.TrimSpace(c.PostForm("footer_text"))
		}
		if overrides["landing"] {
			cfg.LandingPage = strings.TrimSpace(c.PostForm("landing_page"))
		}
		if err := saveCustomerPortalConfigForCustomer(db, customerID, cfg, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Save writes every field as an override; now strip the ones the
		// admin didn't opt into so they revert to inheriting the global
		// default.
		for key, isOverride := range overrides {
			if isOverride {
				continue
			}
			if err := sysconfig.DeleteCustomerPortalConfigKeyForCompany(db, customerID, key); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("clear %s override: %v", key, err)})
				return
			}
		}

		// The company edit form is a plain HTML form: send the browser back to
		// the Portal Settings tab instead of a raw JSON document.
		if strings.Contains(c.GetHeader("Accept"), "text/html") && c.GetHeader("HX-Request") != "true" {
			c.Redirect(http.StatusSeeOther,
				"/admin/customer/companies/"+url.PathEscape(customerID)+"/edit?tab=portal&success=1")
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
