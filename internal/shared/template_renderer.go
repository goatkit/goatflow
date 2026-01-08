package shared

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/i18n"
	"github.com/gotrs-io/gotrs-ce/internal/lookups"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/version"
)

// TemplateRenderer handles template rendering with pongo2.
type TemplateRenderer struct {
	templateSet *pongo2.TemplateSet
	templateDir string
}

// NewTemplateRenderer creates a new template renderer.
func NewTemplateRenderer(templateDir string) (*TemplateRenderer, error) {
	if templateDir == "" {
		return nil, fmt.Errorf("template directory is required")
	}

	// Check if directory exists
	if _, err := os.Stat(templateDir); err != nil {
		return nil, fmt.Errorf("template directory not found: %v", err)
	}

	// Normalize path
	abs, _ := filepath.Abs(templateDir)

	// Create template set
	templateSet := pongo2.NewSet("gotrs", pongo2.MustNewLocalFileSystemLoader(abs))

	return &TemplateRenderer{
		templateSet: templateSet,
		templateDir: abs,
	}, nil
}

// TemplateSet returns the underlying pongo2 template set for modules that need direct access.
func (r *TemplateRenderer) TemplateSet() *pongo2.TemplateSet {
	if r == nil {
		return nil
	}
	return r.templateSet
}

// HTML renders a template.
func (r *TemplateRenderer) HTML(c *gin.Context, code int, name string, data interface{}) {
	// Convert gin.H to pongo2.Context
	var ctx pongo2.Context
	switch v := data.(type) {
	case pongo2.Context:
		ctx = v
	case gin.H:
		ctx = pongo2.Context(v)
	case map[string]interface{}:
		ctx = pongo2.Context(v)
	default:
		ctx = pongo2.Context{"data": data}
	}

	// Language helpers injected via middleware detection
	lang := middleware.GetLanguage(c)
	i18nInst := i18n.GetInstance()
	ctx["t"] = func(key string, args ...interface{}) string {
		return translateWithFallback(i18nInst, lang, key, args...)
	}
	ctx["getLang"] = func() string { return lang }
	ctx["getDirection"] = func() string { return string(i18n.GetDirection(lang)) }
	ctx["isRTL"] = func() bool { return i18n.IsRTL(lang) }
	ctx["Countries"] = lookups.Countries()

	// Add language info directly to context
	ctx["Lang"] = lang
	ctx["Direction"] = string(i18n.GetDirection(lang))
	ctx["IsRTL"] = i18n.IsRTL(lang)

	// Add version info to context
	ctx["AppVersion"] = version.String()
	ctx["AppVersionShort"] = version.Short()
	ctx["AppVersionFull"] = version.Full()
	ctx["AppVersionInfo"] = version.GetInfo()

	// Auto-inject User and IsInAdminGroup from context for consistent nav bar
	isAdmin := false
	if flag, exists := c.Get("isInAdminGroup"); exists {
		if b, ok := flag.(bool); ok {
			isAdmin = b
		}
	}
	ctx["IsInAdminGroup"] = isAdmin

	// Inject User from context if available
	if _, hasUser := ctx["User"]; !hasUser {
		if user := getUserFromContext(c, isAdmin); user != nil {
			ctx["User"] = user
		}
	}

	// Get the template (fallback for tests when templates missing)
	if r == nil || r.templateSet == nil {
		// Minimal safe fallback for tests: render a tiny stub
		c.String(code, "GOTRS")
		return
	}
	tmpl, err := r.templateSet.FromFile(name)
	if err != nil {
		log.Printf("Template renderer failed to load template %q: %v", name, err)
		c.String(code, "Template not found: %s", name)
		return
	}

	// Set response headers
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(code)

	// Render template
	err = tmpl.ExecuteWriter(ctx, c.Writer)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template execution error: %v", err)
	}
}

// getUserFromContext extracts the user from gin context (set by JWT middleware).
func getUserFromContext(c *gin.Context, isAdmin bool) *models.User {
	// Try direct user object first
	userInterface, exists := c.Get("user")
	if exists {
		if user, ok := userInterface.(*models.User); ok {
			user.IsInAdminGroup = isAdmin
			return user
		}
		if user, ok := userInterface.(models.User); ok {
			user.IsInAdminGroup = isAdmin
			return &user
		}
	}

	// Fall back to building user from individual context values
	userID, hasID := c.Get("user_id")
	if !hasID {
		return nil
	}

	user := &models.User{IsInAdminGroup: isAdmin}

	// Set ID from context
	switch id := userID.(type) {
	case uint:
		user.ID = id
	case int:
		user.ID = uint(id)
	case int64:
		user.ID = uint(id)
	default:
		return nil
	}

	// Set role
	if role, ok := c.Get("user_role"); ok {
		if r, ok := role.(string); ok {
			user.Role = r
		}
	}

	// Set email
	if email, ok := c.Get("user_email"); ok {
		if e, ok := email.(string); ok {
			user.Email = e
			user.Login = e
		}
	}

	// Set name
	if name, ok := c.Get("user_name"); ok {
		if n, ok := name.(string); ok {
			parts := strings.SplitN(n, " ", 2)
			if len(parts) > 0 {
				user.FirstName = parts[0]
			}
			if len(parts) > 1 {
				user.LastName = parts[1]
			}
		}
	}

	return user
}

// translateWithFallback provides a fallback translation function.
func translateWithFallback(i18nInst *i18n.I18n, lang, key string, args ...interface{}) string {
	if i18nInst == nil {
		return key
	}
	return i18nInst.Translate(lang, key, args...)
}

// GetGlobalRenderer returns the global template renderer instance.
func GetGlobalRenderer() *TemplateRenderer {
	return globalTemplateRenderer
}

// SetGlobalRenderer sets the global template renderer instance.
func SetGlobalRenderer(renderer *TemplateRenderer) {
	globalTemplateRenderer = renderer
}

var globalTemplateRenderer *TemplateRenderer
