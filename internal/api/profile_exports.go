package api

import "github.com/gin-gonic/gin"

// Exported versions of handler functions for the routing system

// HandleProfile is the exported version of handleProfile
func HandleProfile(c *gin.Context) {
	handleProfile(c)
}

// HandleRedirectProfile is the exported version of redirect handler
func HandleRedirectProfile(c *gin.Context) {
	handleRedirectProfile(c)
}

// HandleRedirectTickets is the exported version
func HandleRedirectTickets(c *gin.Context) {
	handleRedirectTickets(c)
}

// HandleRedirectTicketsNew is the exported version
func HandleRedirectTicketsNew(c *gin.Context) {
	handleRedirectTicketsNew(c)
}

// HandleRedirectQueues is the exported version
func HandleRedirectQueues(c *gin.Context) {
	handleRedirectQueues(c)
}

// HandleRedirectSettings is the exported version
func HandleRedirectSettings(c *gin.Context) {
	handleRedirectSettings(c)
}