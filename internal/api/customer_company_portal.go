package api

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/routing"
)

func init() {
	routing.RegisterHandler("handleCustomerCompanyInfo", handleCustomerCompanyInfo)
	routing.RegisterHandler("handleCustomerCompanyUsers", handleCustomerCompanyUsers)
}

// portalCompany is the customer company of the logged-in customer user, as
// shown on the customer portal. Agent-only fields (comments) are not loaded.
type portalCompany struct {
	CustomerID string
	Name       string
	Street     string
	Zip        string
	City       string
	Country    string
	URL        string
}

// loadPortalCompany returns the valid customer_company row that the customer
// user `login` belongs to (customer_user.customer_id). It returns nil when the
// user has no company or the company is invalid: the portal then shows the
// "not linked to a company" state rather than another company's data.
func loadPortalCompany(c *gin.Context, db *sql.DB, login string) (*portalCompany, error) {
	validID, err := lookups.ID(c.Request.Context(), db, lookups.ValidLookup, "valid")
	if err != nil {
		return nil, err
	}
	var (
		company                                     portalCompany
		street, zip, city, country, url, customerID sql.NullString
	)
	err = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`
		SELECT cc.customer_id, cc.name, cc.street, cc.zip, cc.city, cc.country, cc.url
		FROM customer_user cu
		JOIN customer_company cc ON cc.customer_id = cu.customer_id
		WHERE cu.login = ? AND cc.valid_id = ?`), login, validID).Scan(
		&customerID, &company.Name, &street, &zip, &city, &country, &url)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	company.CustomerID = customerID.String
	company.Street = street.String
	company.Zip = zip.String
	company.City = city.String
	company.Country = country.String
	company.URL = safePortalURL(url.String)
	return &company, nil
}

// safePortalURL only lets http(s) company URLs become links; anything else
// (javascript:, data:, ...) is dropped.
func safePortalURL(raw string) string {
	u := strings.TrimSpace(raw)
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return u
	}
	return ""
}

// portalCompanyUser is a colleague listed on the customer portal company page.
type portalCompanyUser struct {
	FullName string
	Title    string
	Email    string
	IsSelf   bool
}

// loadPortalCompanyUsers lists the valid customer users of the given company.
func loadPortalCompanyUsers(c *gin.Context, db *sql.DB, customerID, selfLogin string) ([]portalCompanyUser, error) {
	validID, err := lookups.ID(c.Request.Context(), db, lookups.ValidLookup, "valid")
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(c.Request.Context(), database.ConvertPlaceholders(`
		SELECT login, title, first_name, last_name, email
		FROM customer_user
		WHERE customer_id = ? AND valid_id = ?
		ORDER BY last_name, first_name, login`), customerID, validID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []portalCompanyUser{}
	for rows.Next() {
		var (
			login, firstName, lastName, email string
			title                             sql.NullString
		)
		if err := rows.Scan(&login, &title, &firstName, &lastName, &email); err != nil {
			return nil, err
		}
		fullName := strings.TrimSpace(firstName + " " + lastName)
		if fullName == "" {
			fullName = login
		}
		users = append(users, portalCompanyUser{
			FullName: fullName,
			Title:    title.String,
			Email:    email,
			IsSelf:   login == selfLogin,
		})
	}
	return users, rows.Err()
}

// handleCustomerCompanyInfo renders GET /customer/company: the details of the
// logged-in customer user's own company.
func handleCustomerCompanyInfo(c *gin.Context) {
	if !requireCustomerAuth(c) {
		return
	}
	db, ok := mustGetDB(c)
	if !ok {
		return
	}
	username := c.GetString("username")
	cfg := customerPortalConfigFromContext(c, db)

	company, err := loadPortalCompany(c, db, username)
	if err != nil {
		log.Printf("customer portal: loading company for %s: %v", username, err)
		c.String(http.StatusInternalServerError, "Failed to load company")
		return
	}
	ctx := pongo2.Context{
		"ActivePage": "company",
	}
	if company != nil {
		ctx["Company"] = company
		users, err := loadPortalCompanyUsers(c, db, company.CustomerID, username)
		if err != nil {
			log.Printf("customer portal: loading company users for %s: %v", username, err)
			c.String(http.StatusInternalServerError, "Failed to load company")
			return
		}
		ctx["UserCount"] = len(users)
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/company_info.pongo2",
		withPortalContextAndCustomer(ctx, cfg, db, username))
}

// handleCustomerCompanyUsers renders GET /customer/company/users: the valid
// customer users of the logged-in customer user's own company.
func handleCustomerCompanyUsers(c *gin.Context) {
	if !requireCustomerAuth(c) {
		return
	}
	db, ok := mustGetDB(c)
	if !ok {
		return
	}
	username := c.GetString("username")
	cfg := customerPortalConfigFromContext(c, db)

	company, err := loadPortalCompany(c, db, username)
	if err != nil {
		log.Printf("customer portal: loading company for %s: %v", username, err)
		c.String(http.StatusInternalServerError, "Failed to load company users")
		return
	}
	ctx := pongo2.Context{
		"ActivePage": "company",
	}
	if company != nil {
		ctx["Company"] = company
		users, err := loadPortalCompanyUsers(c, db, company.CustomerID, username)
		if err != nil {
			log.Printf("customer portal: loading company users for %s: %v", username, err)
			c.String(http.StatusInternalServerError, "Failed to load company users")
			return
		}
		ctx["Users"] = users
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/company_users.pongo2",
		withPortalContextAndCustomer(ctx, cfg, db, username))
}
