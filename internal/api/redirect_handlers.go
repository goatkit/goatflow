package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleRedirectTickets redirects to the appropriate tickets page based on user role.
func handleRedirectTickets(c *gin.Context) {
	roleVal, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	role, _ := roleVal.(string) //nolint:errcheck // Defaults to empty
	switch role {
	case "Admin":
		c.Redirect(http.StatusSeeOther, "/agent/tickets")
	case "Agent":
		c.Redirect(http.StatusSeeOther, "/agent/tickets")
	case "Customer":
		c.Redirect(http.StatusSeeOther, "/customer/tickets")
	default:
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// handleRedirectTicketsNew redirects to the appropriate new ticket page based on user role.
func handleRedirectTicketsNew(c *gin.Context) {
	roleVal, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	role, _ := roleVal.(string) //nolint:errcheck // Defaults to empty
	switch role {
	case "Admin", "Agent":
		// Agents create tickets via the main tickets page
		c.Redirect(http.StatusSeeOther, "/agent/tickets")
	case "Customer":
		c.Redirect(http.StatusSeeOther, "/customer/tickets/new")
	default:
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// handleRedirectQueues redirects to the appropriate queues page based on user role.
func handleRedirectQueues(c *gin.Context) {
	roleVal, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	role, _ := roleVal.(string) //nolint:errcheck // Defaults to empty
	switch role {
	case "Admin", "Agent":
		// Show the standard queues list page (not the admin management UI)
		// to keep the primary nav consistent. Avoid redirect to admin queues.
		// Directly invoke handleQueues to prevent redirect loops if this handler
		// is bound at /queues.
		handleQueues(c)
		return
	default:
		// Customers don't have queue access
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// handleRedirectProfile redirects to the appropriate profile page based on user role.
func handleRedirectProfile(c *gin.Context) {
	roleVal, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	role, _ := roleVal.(string) //nolint:errcheck // Defaults to empty
	switch role {
	case "Admin", "Agent":
		// Redirect to agent profile page
		c.Redirect(http.StatusSeeOther, "/agent/profile")
	case "Customer":
		c.Redirect(http.StatusSeeOther, "/customer/profile")
	default:
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// handleRedirectSettings redirects to the appropriate settings page based on user role.
func handleRedirectSettings(c *gin.Context) {
	roleVal, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	role, _ := roleVal.(string) //nolint:errcheck // Defaults to empty
	switch role {
	case "Admin":
		c.Redirect(http.StatusSeeOther, "/admin")
	case "Agent":
		c.Redirect(http.StatusSeeOther, "/agent/dashboard")
	case "Customer":
		c.Redirect(http.StatusSeeOther, "/customer/profile")
	default:
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}
