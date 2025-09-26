package api

import (
	"net/http"
	"os"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
)

// handleAdminRoadmap displays the ROADMAP.md file as HTML
func handleAdminRoadmap(c *gin.Context) {
	// Read the ROADMAP.md file
	content, err := os.ReadFile("ROADMAP.md")
	if err != nil {
		// If file doesn't exist, show error
		pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/roadmap.pongo2", pongo2.Context{
			"Title":       "Development Roadmap",
			"Error":       "Unable to load roadmap file: " + err.Error(),
			"User":        getUserMapForTemplate(c),
			"ActivePage":  "admin",
		})
		return
	}

	// Convert Markdown to HTML using shared utility
	htmlString := RenderMarkdown(string(content))

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/roadmap.pongo2", pongo2.Context{
		"Title":       "Development Roadmap",
		"HTMLContent": htmlString,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}