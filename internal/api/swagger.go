package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// Import generated docs - this registers SwaggerInfo via init()
	"github.com/goatkit/goatflow/docs/api"
	"github.com/goatkit/goatflow/internal/routing"
)

// Dark mode CSS for Swagger UI - injected into index.html
const swaggerDarkModeCSS = `<style>
/* GoatFlow Swagger Dark Mode */
body { background: #1a1a2e !important; }
.swagger-ui { background: #1a1a2e; }
.swagger-ui .topbar { background: #16213e; }
.swagger-ui .info .title, .swagger-ui .info p, .swagger-ui .info li,
.swagger-ui .opblock-tag, .swagger-ui .opblock .opblock-summary-description,
.swagger-ui .opblock-description-wrapper p, .swagger-ui .response-col_description,
.swagger-ui table thead tr th, .swagger-ui .parameter__name, .swagger-ui .parameter__type,
.swagger-ui .response-col_status, .swagger-ui label, .swagger-ui .btn,
.swagger-ui select, .swagger-ui .model-title, .swagger-ui .model,
.swagger-ui .responses-inner h4, .swagger-ui .responses-inner h5 { color: #e0e0e0 !important; }
.swagger-ui .opblock .opblock-section-header { background: #16213e; }
.swagger-ui .opblock .opblock-section-header h4 { color: #e0e0e0; }
.swagger-ui .opblock-body pre.microlight { background: #0f0f23 !important; color: #e0e0e0 !important; }
.swagger-ui .highlight-code > .microlight code { color: #e0e0e0 !important; }
.swagger-ui input[type=text], .swagger-ui textarea, .swagger-ui select {
  background: #16213e !important; color: #e0e0e0 !important; border-color: #404060 !important;
}
.swagger-ui .scheme-container { background: #16213e; }
.swagger-ui section.models { background: #16213e; border-color: #404060; }
.swagger-ui .model-box { background: #0f0f23; }
.swagger-ui .opblock.opblock-get { background: rgba(97,175,254,.1); border-color: #61affe; }
.swagger-ui .opblock.opblock-post { background: rgba(73,204,144,.1); border-color: #49cc90; }
.swagger-ui .opblock.opblock-put { background: rgba(252,161,48,.1); border-color: #fca130; }
.swagger-ui .opblock.opblock-delete { background: rgba(249,62,62,.1); border-color: #f93e3e; }
.swagger-ui .btn.authorize { background: #49cc90; border-color: #49cc90; }
.swagger-ui .authorization__btn { fill: #49cc90; }
.swagger-ui .servers > label select { background: #16213e; color: #e0e0e0; }
</style></head>`

func init() {
	// Register swagger handlers for YAML routing
	routing.RegisterHandler("handleSwagger", handleSwagger)
	routing.RegisterHandler("handleSwaggerDark", handleSwaggerDark)

	// Configure swagger to use relative URLs (works from any host)
	// Empty host means "use the same host as the page"
	api.SwaggerInfo.Host = ""
	api.SwaggerInfo.BasePath = "/api/v1"
}

// handleSwagger serves the Swagger UI (light mode).
// Access control is handled by YAML route middleware configuration.
func handleSwagger(c *gin.Context) {
	handler := ginSwagger.WrapHandler(swaggerFiles.Handler,
		ginSwagger.DefaultModelsExpandDepth(-1),
		ginSwagger.PersistAuthorization(true),
	)
	handler(c)
}

// handleSwaggerDark serves Swagger UI with dark mode CSS injected.
func handleSwaggerDark(c *gin.Context) {
	// Check if this is the index.html request
	path := c.Param("any")
	if path == "/index.html" || path == "/" || path == "" {
		// Serve custom dark mode index.html
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, swaggerDarkIndexHTML)
		return
	}
	
	// For other files (CSS, JS, doc.json, etc.), use default handler
	handler := ginSwagger.WrapHandler(swaggerFiles.Handler,
		ginSwagger.DefaultModelsExpandDepth(-1),
		ginSwagger.PersistAuthorization(true),
	)
	handler(c)
}

// Custom index.html with dark mode CSS embedded
const swaggerDarkIndexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>GoatFlow API Documentation</title>
  <link rel="stylesheet" type="text/css" href="./swagger-ui.css" >
  <link rel="icon" type="image/png" href="./favicon-32x32.png" sizes="32x32" />
  <style>
    /* GoatFlow Swagger Dark Mode */
    html { box-sizing: border-box; overflow-y: scroll; }
    body { margin: 0; background: #1a1a2e !important; }
    .swagger-ui { background: #1a1a2e; }
    .swagger-ui .topbar { background: #16213e; padding: 10px 0; }
    .swagger-ui .topbar .download-url-wrapper .download-url-button { background: #49cc90; }
    .swagger-ui .info .title, .swagger-ui .info p, .swagger-ui .info li,
    .swagger-ui .opblock-tag, .swagger-ui .opblock .opblock-summary-description,
    .swagger-ui .opblock-description-wrapper p, .swagger-ui .response-col_description,
    .swagger-ui table thead tr th, .swagger-ui .parameter__name, .swagger-ui .parameter__type,
    .swagger-ui .response-col_status, .swagger-ui label, .swagger-ui .btn,
    .swagger-ui select, .swagger-ui .model-title, .swagger-ui .model,
    .swagger-ui .responses-inner h4, .swagger-ui .responses-inner h5,
    .swagger-ui .opblock-summary-method { color: #e0e0e0 !important; }
    .swagger-ui .opblock .opblock-section-header { background: #16213e; }
    .swagger-ui .opblock .opblock-section-header h4 { color: #e0e0e0; }
    .swagger-ui .opblock-body pre.microlight { background: #0f0f23 !important; }
    .swagger-ui .microlight { color: #e0e0e0 !important; }
    .swagger-ui input[type=text], .swagger-ui textarea, .swagger-ui select {
      background: #16213e !important; color: #e0e0e0 !important; border-color: #404060 !important;
    }
    .swagger-ui .scheme-container { background: #16213e; box-shadow: none; }
    .swagger-ui section.models { background: #16213e; border-color: #404060; }
    .swagger-ui .model-box { background: #0f0f23; }
    .swagger-ui .opblock.opblock-get { background: rgba(97,175,254,.1); border-color: #61affe; }
    .swagger-ui .opblock.opblock-post { background: rgba(73,204,144,.1); border-color: #49cc90; }
    .swagger-ui .opblock.opblock-put { background: rgba(252,161,48,.1); border-color: #fca130; }
    .swagger-ui .opblock.opblock-delete { background: rgba(249,62,62,.1); border-color: #f93e3e; }
    .swagger-ui .opblock.opblock-patch { background: rgba(80,227,194,.1); border-color: #50e3c2; }
    .swagger-ui .btn.authorize { background: #49cc90; border-color: #49cc90; color: #fff; }
    .swagger-ui .authorization__btn { fill: #49cc90; }
    .swagger-ui .servers > label select { background: #16213e; color: #e0e0e0; }
    .swagger-ui .response-col_links { color: #e0e0e0; }
    .swagger-ui .tab li button.tablinks { background: #16213e; color: #e0e0e0; }
    .swagger-ui .copy-to-clipboard { background: #16213e; }
    .swagger-ui .copy-to-clipboard button { background: #16213e; }
  </style>
</head>
<body>
<div id="swagger-ui"></div>
<script src="./swagger-ui-bundle.js" charset="UTF-8"></script>
<script src="./swagger-ui-standalone-preset.js" charset="UTF-8"></script>
<script>
window.onload = function() {
  window.ui = SwaggerUIBundle({
    url: "./doc.json",
    dom_id: '#swagger-ui',
    deepLinking: true,
    persistAuthorization: true,
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
    plugins: [SwaggerUIBundle.plugins.DownloadUrl],
    layout: "StandaloneLayout",
    defaultModelsExpandDepth: -1
  });
};
</script>
</body>
</html>`
