package template

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/i18n"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// Pongo2Renderer is a custom Gin renderer using Pongo2.
type Pongo2Renderer struct {
	Debug       bool
	TemplateDir string
	mu          sync.RWMutex
	cache       map[string]*pongo2.Template
}

// NewPongo2Renderer creates a new Pongo2 renderer.
func NewPongo2Renderer(templateDir string, debug bool) *Pongo2Renderer {
	return &Pongo2Renderer{
		Debug:       debug,
		TemplateDir: templateDir,
		cache:       make(map[string]*pongo2.Template),
	}
}

// Instance returns a Pongo2 template instance, using cache if not in debug mode.
func (r *Pongo2Renderer) Instance(name string, data interface{}) *pongo2.Template {
	var tmpl *pongo2.Template
	var err error
	// no shadowing warnings

	// Use absolute path for templates
	fullPath := r.TemplateDir + "/" + name
	fmt.Printf("DEBUG Instance: Loading template from %s\n", fullPath)

	if r.Debug {
		// Always load from disk in debug mode
		tmpl, err = pongo2.FromFile(fullPath)
		if err != nil {
			fmt.Printf("DEBUG Instance Error: %v\n", err)
		}
	} else {
		// Use cache in production
		r.mu.RLock()
		tmpl = r.cache[name]
		r.mu.RUnlock()

		if tmpl == nil {
			t, e := pongo2.FromFile(fullPath)
			if e == nil {
				r.mu.Lock()
				r.cache[name] = t
				r.mu.Unlock()
				tmpl = t
			} else {
				err = e
			}
		}
	}

	if err != nil {
		// Return a template that will show the error
		tmpl, _ = pongo2.FromString("Template error")
	}

	return tmpl
}

// Render renders a Pongo2 template.
func (r *Pongo2Renderer) Render(c *gin.Context, code int, name string, data interface{}) error {
	// Get language from context
	lang := middleware.GetLanguage(c)
	i18nInstance := i18n.GetInstance()

	// Debug: Log the detected language
	fmt.Printf("DEBUG: Detected language: %s\n", lang)

	// Convert data to pongo2.Context
	ctx := make(pongo2.Context)

	// Add i18n function that captures the language
	ctx["t"] = func(key string, args ...interface{}) string {
		v := i18nInstance.T(lang, key, args...)
		if v != key {
			return v
		}
		// English fallback
		enVal := i18nInstance.T("en", key, args...)
		if enVal != key {
			return enVal
		}
		// Humanize
		last := key
		if strings.Contains(key, ".") {
			parts := strings.Split(key, ".")
			last = parts[len(parts)-1]
		}
		last = strings.ReplaceAll(last, "_", " ")
		if len(last) > 0 {
			last = strings.ToUpper(last[:1]) + last[1:]
		}
		return last
	}

	// Add language helpers
	ctx["getLang"] = func() string {
		return lang
	}
	ctx["getDirection"] = func() string {
		return string(i18n.GetDirection(lang))
	}
	ctx["isRTL"] = func() bool {
		return i18n.IsRTL(lang)
	}

	// Add language info to context
	ctx["Lang"] = lang
	ctx["Direction"] = string(i18n.GetDirection(lang))
	ctx["IsRTL"] = i18n.IsRTL(lang)

	// Auto-inject User and IsInAdminGroup from context for consistent nav bar
	// isInAdminGroup is set by JWT middleware from claims.IsAdmin
	isAdmin := false
	if flag, exists := c.Get("isInAdminGroup"); exists {
		if b, ok := flag.(bool); ok {
			isAdmin = b
		}
	}
	ctx["IsInAdminGroup"] = isAdmin

	if _, hasUser := ctx["User"]; !hasUser {
		if user := r.getUserFromContext(c); user != nil {
			user.IsInAdminGroup = isAdmin
			ctx["User"] = user
		}
	}

	// Add the data
	switch v := data.(type) {
	case pongo2.Context:
		for key, value := range v {
			ctx[key] = value
		}
	case gin.H:
		for key, value := range v {
			ctx[key] = value
		}
	case map[string]interface{}:
		for key, value := range v {
			ctx[key] = value
		}
	default:
		ctx["Data"] = data
	}

	// Get template
	tmpl := r.Instance(name, data)

	// Set response headers
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(code)

	// Execute template
	return tmpl.ExecuteWriter(ctx, c.Writer)
}

// HTML renders an HTML template.
func (r *Pongo2Renderer) HTML(c *gin.Context, code int, name string, data interface{}) {
	if err := r.Render(c, code, name, data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// getUserFromContext extracts the user from gin context (set by JWT middleware).
func (r *Pongo2Renderer) getUserFromContext(c *gin.Context) *models.User {
	// Try direct user object first
	userInterface, exists := c.Get("user")
	if exists {
		if user, ok := userInterface.(*models.User); ok {
			return user
		}
		if user, ok := userInterface.(models.User); ok {
			return &user
		}
	}

	// Fall back to building user from individual context values
	userID, hasID := c.Get("user_id")
	if !hasID {
		return nil
	}

	user := &models.User{}

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
