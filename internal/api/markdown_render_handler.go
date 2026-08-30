package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/pkg/markdown"
)

// maxMarkdownRenderBytes caps the markdown body accepted by the render
// endpoint — well above any legitimate article/transcript, small enough that
// an abusive caller cannot force unbounded goldmark+bluemonday work.
const maxMarkdownRenderBytes = 1 << 20 // 1 MiB

func init() {
	routing.RegisterHandler("HandleMarkdownRenderAPI", handleMarkdownRenderAPI)
}

// handleMarkdownRenderAPI renders markdown to sanitized HTML (goldmark GFM +
// bluemonday, the same stack as ticket notes and plugin UI). It returns the
// bare sanitized output — deliberately NOT the Tailwind-classed
// RenderMarkdown wrapper — so consumers can style it with their own theme
// (e.g. a plugin's prose variables) instead of inheriting gray-900 utility
// classes that clash with plugin themes.
func handleMarkdownRenderAPI(c *gin.Context) {
	var req struct {
		Markdown string `json:"markdown"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	if len(req.Markdown) > maxMarkdownRenderBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "markdown too large (max 1 MiB)"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"html": markdown.Render(req.Markdown)})
}
