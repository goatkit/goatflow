package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// and returns a friendly error page if the database is down.
func DatabaseHealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip health check for static assets and health endpoint
		path := c.Request.URL.Path
		if path == "/health" || path == "/static" || len(path) > 7 && path[:7] == "/static" {
			c.Next()
			return
		}

		// Check database connection
		db, err := database.GetDB()
		if err != nil || db == nil {
			handleDatabaseError(c, "Unable to connect to the database",
				"The database server appears to be offline or unreachable. Please ensure the database container is running and try again.",
				"")
			return
		}

		// Create a context with timeout for the ping (2 seconds max)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Test the connection with a simple ping
		if err := db.PingContext(ctx); err != nil {
			handleDatabaseError(c, "Database is not responding",
				"The database connection exists but is not responding to queries. This may be a temporary issue.",
				"")
			return
		}

		c.Next()
	}
}

func handleDatabaseError(c *gin.Context, message, details, suggestion string) {
	// Check if this is an API request
	if isDatabaseCheckAPIRequest(c) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "Database connection failed",
			"message":    message,
			"details":    details,
			"suggestion": suggestion,
			"status":     http.StatusServiceUnavailable,
		})
		c.Abort()
		return
	}

	// For HTML requests, render a styled error page matching the main UI
	htmlContent := `
<!DOCTYPE html>
<html lang="en" class="h-full">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Database Connection Error - GoatFlow</title>
    
    <!-- Favicon -->
    <link rel="icon" type="image/svg+xml" href="/static/favicon.svg">
    
    <!-- Dark mode detection script (must be inline to avoid flash) -->
    <script>
        // Set dark mode class before styles load to prevent flash
        if (localStorage.theme === 'dark' || (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
            document.documentElement.classList.add('dark')
        } else {
            document.documentElement.classList.remove('dark')
        }
    </script>
    
    <!-- Tailwind CSS -->
    <link rel="stylesheet" href="/static/css/output.css">
    
    <!-- Font Awesome Icons -->
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    
    <style>
        /* Smooth transitions for dark mode */
        * {
            transition: background-color 0.2s, border-color 0.2s;
        }
    </style>
</head>
<body class="h-full bg-gray-50 dark:bg-gray-900">
    <div class="min-h-full flex items-center justify-center px-4 py-12">
        <div class="max-w-md w-full">
            <!-- Logo and Product Name -->
            <div class="text-center mb-8">
                <div class="flex justify-center items-center mb-4">
                    <img src="/static/favicon.svg" alt="GoatFlow Logo" class="h-12 w-12 mr-3">
                    <span class="text-4xl font-bold text-goatflow-600 dark:text-goatflow-400">GoatFlow</span>
                </div>
                
                <!-- Dark mode toggle -->
                <button type="button" onclick="toggleDarkMode()" class="rounded-md p-2 text-gray-400 hover:text-gray-500 dark:text-gray-300 dark:hover:text-gray-200">
                    <i class="fas fa-moon text-lg"></i>
                </button>
            </div>
            
            <!-- Error Card -->
            <div class="bg-white dark:bg-gray-800 shadow-lg rounded-lg p-8">
                <div class="flex items-center mb-4">
                    <div class="flex-shrink-0">
                        <div class="flex items-center justify-center h-12 w-12 rounded-full bg-red-100 dark:bg-red-900">
                            <i class="fas fa-database text-red-600 dark:text-red-400"></i>
                        </div>
                    </div>
                    <div class="ml-4">
                        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">Database Connection Error</h1>
                    </div>
                </div>
                
                <div class="mt-4">
                    <p class="text-sm font-medium text-gray-900 dark:text-gray-100">` + message + `</p>
                    <div class="mt-4 p-4 bg-amber-50 dark:bg-amber-900/20 border-l-4 border-amber-400 dark:border-amber-600">
                        <p class="text-sm text-gray-700 dark:text-gray-300">` + details + `</p>
                    </div>
                </div>
                
                <div class="mt-6">
                    <button onclick="location.reload()" class="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-goatflow-600 hover:bg-goatflow-700 dark:bg-goatflow-500 dark:hover:bg-goatflow-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-goatflow-500">
                        <i class="fas fa-sync-alt mr-2"></i>
                        Retry Connection
                    </button>
                </div>
            </div>
            
            <!-- Status Information -->
            <div class="mt-6 text-center">
                <p class="text-sm text-gray-500 dark:text-gray-400">
                    The system will automatically retry when you refresh the page
                </p>
            </div>
        </div>
    </div>
    
    <!-- Dark mode toggle script -->
    <script>
        function toggleDarkMode() {
            if (document.documentElement.classList.contains('dark')) {
                document.documentElement.classList.remove('dark');
                localStorage.theme = 'light';
            } else {
                document.documentElement.classList.add('dark');
                localStorage.theme = 'dark';
            }
        }
    </script>
</body>
</html>`

	c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", []byte(htmlContent))
	c.Abort()
}

func isDatabaseCheckAPIRequest(c *gin.Context) bool {
	// Check if path starts with /api/
	if len(c.Request.URL.Path) >= 5 && c.Request.URL.Path[:5] == "/api/" {
		return true
	}
	// Check Accept header
	accept := c.GetHeader("Accept")
	return accept == "application/json" ||
		c.GetHeader("Content-Type") == "application/json"
}
