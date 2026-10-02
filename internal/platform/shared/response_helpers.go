package shared

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// sendToastResponse sends either an HTMX toast notification or redirects with success message.
func SendToastResponse(c *gin.Context, success bool, message, redirectPath string) {
	if c.GetHeader("HX-Request") == "true" {
		// Return HTML partial with toast notification
		if success {
			html := fmt.Sprintf(`
				<div id="toast" class="fixed top-4 right-4 bg-green-500 text-white px-6 py-3 rounded-lg shadow-lg z-50">
					<div class="flex items-center">
						<svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
						</svg>
						%s
					</div>
				</div>
				<script>
					setTimeout(function() {
						var toast = document.getElementById('toast');
						if (toast) toast.remove();
					}, 3000);
				</script>
			`, message)
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		} else {
			SendToastError(c, http.StatusBadRequest, message)
		}
		return
	}

	if acceptsJSONResponse(c) {
		if !success {
			SendToastError(c, http.StatusBadRequest, message)
			return
		}

		payload := gin.H{
			"success": success,
			"message": message,
		}

		if redirectPath != "" {
			payload["redirect"] = redirectPath
		}

		c.JSON(http.StatusOK, payload)
		return
	}

	// For regular form submissions, redirect back with success message
	if success && redirectPath != "" {
		suffix := "?success=1"
		if strings.Contains(redirectPath, "?") {
			suffix = "&success=1"
		}
		c.Redirect(http.StatusFound, redirectPath+suffix)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": success, "message": message})
}

// SendToastError sends a failure toast (HTMX partial or JSON) with the given HTTP status.
func SendToastError(c *gin.Context, status int, message string) {
	if c.GetHeader("HX-Request") == "true" {
		html := fmt.Sprintf(`
				<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded" role="alert">
					%s
				</div>
			`, message)
		c.Data(status, "text/html; charset=utf-8", []byte(html))
		return
	}
	c.JSON(status, gin.H{"success": false, "message": message})
}

func acceptsJSONResponse(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	if accept == "" {
		return false
	}
	return strings.Contains(accept, "application/json")
}
