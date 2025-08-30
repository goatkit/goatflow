package api

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

// handleRedirectTickets redirects to the appropriate tickets page based on user role
func handleRedirectTickets(c *gin.Context) {
	role, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	
	switch role.(string) {
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

// handleRedirectTicketsNew redirects to the appropriate new ticket page based on user role
func handleRedirectTicketsNew(c *gin.Context) {
	role, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	
	switch role.(string) {
	case "Admin", "Agent":
		// Agents create tickets via the main tickets page
		c.Redirect(http.StatusSeeOther, "/agent/tickets")
	case "Customer":
		c.Redirect(http.StatusSeeOther, "/customer/tickets/new")
	default:
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// handleRedirectQueues redirects to the appropriate queues page based on user role
func handleRedirectQueues(c *gin.Context) {
	role, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	
	switch role.(string) {
	case "Admin":
		c.Redirect(http.StatusSeeOther, "/admin/queues")
	case "Agent":
		c.Redirect(http.StatusSeeOther, "/agent/queues")
	default:
		// Customers don't have queue access
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// handleRedirectProfile redirects to the appropriate profile page based on user role
func handleRedirectProfile(c *gin.Context) {
	role, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	
	switch role.(string) {
	case "Admin", "Agent":
		// Redirect to agent profile page
		c.Redirect(http.StatusSeeOther, "/agent/profile")
	case "Customer":
		c.Redirect(http.StatusSeeOther, "/customer/profile")
	default:
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

// handleRedirectSettings redirects to the appropriate settings page based on user role
func handleRedirectSettings(c *gin.Context) {
	role, exists := c.Get("user_role")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	
	switch role.(string) {
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