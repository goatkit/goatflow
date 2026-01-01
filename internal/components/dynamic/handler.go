// Package dynamic provides dynamic component rendering and field handling.
package dynamic

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	pongo2 "github.com/flosch/pongo2/v6"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/gotrs-io/gotrs-ce/internal/components/lambda"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/i18n"
	"github.com/gotrs-io/gotrs-ce/internal/mailaccountmeta"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

// Format: sha256$salt$hash (similar to OTRS format).
func hashPassword(password string) string {
	// Generate a random salt (16 bytes = 32 hex chars)
	salt := generateSalt()

	// Combine password and salt, then hash
	combined := password + salt
	hash := sha256.Sum256([]byte(combined))
	hashStr := hex.EncodeToString(hash[:])

	// Return in format: sha256$salt$hash
	return fmt.Sprintf("sha256$%s$%s", salt, hashStr)
}

// generateSalt generates a random salt for password hashing.
func generateSalt() string {
	// Generate 16 random bytes
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		// Fallback to timestamp-based salt if crypto/rand fails
		data := fmt.Sprintf("%d", time.Now().UnixNano())
		hash := sha256.Sum256([]byte(data))
		return hex.EncodeToString(hash[:16])
	}
	return hex.EncodeToString(salt)
}

// ModuleConfig represents a module's configuration.
type ModuleConfig struct {
	Module struct {
		Name        string `yaml:"name"`
		Singular    string `yaml:"singular"`
		Plural      string `yaml:"plural"`
		Table       string `yaml:"table"`
		Description string `yaml:"description"`
		RoutePrefix string `yaml:"route_prefix"`
	} `yaml:"module"`

	Fields []Field `yaml:"fields"`

	ComputedFields []ComputedField `yaml:"computed_fields"`

	Features struct {
		SoftDelete   bool `yaml:"soft_delete"`
		Search       bool `yaml:"search"`
		ImportCSV    bool `yaml:"import_csv"`
		ExportCSV    bool `yaml:"export_csv"`
		StatusToggle bool `yaml:"status_toggle"`
		ColorPicker  bool `yaml:"color_picker"`
	} `yaml:"features"`

	Permissions []string `yaml:"permissions"`

	Validation struct {
		UniqueFields []string `yaml:"unique_fields"`
		Required     []string `yaml:"required_fields"`
	} `yaml:"validation"`

	UI struct {
		ListColumns []string `yaml:"list_columns" json:"list_columns"`
	} `yaml:"ui" json:"ui"`

	Filters []Filter `yaml:"filters" json:"filters"`

	LambdaConfig lambda.LambdaConfig `yaml:"lambda_config" json:"lambda_config"`
}

// Field represents a field in the module.
type Field struct {
	Name            string         `yaml:"name"`
	Type            string         `yaml:"type"`
	DBColumn        string         `yaml:"db_column"`
	Label           string         `yaml:"label"`
	Required        bool           `yaml:"required"`
	Searchable      bool           `yaml:"searchable"`
	Sortable        bool           `yaml:"sortable"`
	ShowInList      bool           `yaml:"show_in_list"`
	ShowInForm      bool           `yaml:"show_in_form"`
	Default         interface{}    `yaml:"default"`
	Options         []Option       `yaml:"options"`
	Validation      string         `yaml:"validation"`
	Help            string         `yaml:"help"`
	Placeholder     string         `yaml:"placeholder"`
	ListPosition    int            `yaml:"list_position"`
	Filterable      bool           `yaml:"filterable"`
	LookupTable     string         `yaml:"lookup_table"`
	LookupKey       string         `yaml:"lookup_key"`
	LookupDisplay   string         `yaml:"lookup_display"`
	LookupCondition string         `yaml:"lookup_condition"`
	Widget          string         `yaml:"widget"`
	DisplayAs       string         `yaml:"display_as"`
	DisplayMap      map[int]string `yaml:"display_map"`
	Virtual         bool           `yaml:"virtual"`
}

// ComputedField represents a computed field that's not directly from the database.
type ComputedField struct {
	Name         string `yaml:"name"`
	Label        string `yaml:"label"`
	ShowInList   bool   `yaml:"show_in_list"`
	ShowInForm   bool   `yaml:"show_in_form"`
	Lambda       string `yaml:"lambda"`        // JavaScript lambda function
	ListPosition int    `yaml:"list_position"` // Optional position in list view
}

// Option for select fields.
type Option struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
	Group string `yaml:"group,omitempty"`
}

// Filter represents a filter configuration.
type Filter struct {
	Field         string         `yaml:"field" json:"field"`
	Type          string         `yaml:"type" json:"type"`
	Label         string         `yaml:"label" json:"label"`
	Source        string         `yaml:"source" json:"source"`
	Query         string         `yaml:"query" json:"query"`
	LookupTable   string         `yaml:"lookup_table" json:"lookup_table"`
	LookupKey     string         `yaml:"lookup_key" json:"lookup_key"`
	LookupDisplay string         `yaml:"lookup_display" json:"lookup_display"`
	Options       []FilterOption `yaml:"options" json:"options"`
}

// FilterOption represents an option in a filter dropdown.
type FilterOption struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label" json:"label"`
}

// DynamicModuleHandler handles all dynamic modules.
type DynamicModuleHandler struct {
	configs      map[string]*ModuleConfig
	mu           sync.RWMutex
	db           *sql.DB
	renderer     *pongo2.TemplateSet
	watcher      *fsnotify.Watcher
	modulesPath  string
	lambdaEngine *lambda.Engine
	ctx          context.Context
	i18n         *i18n.I18n
}

// NewDynamicModuleHandler creates a new dynamic module handler.
func NewDynamicModuleHandler(db *sql.DB, renderer *pongo2.TemplateSet, modulesPath string) (*DynamicModuleHandler, error) {
	ctx := context.Background()
	lambdaEngine, err := lambda.NewEngine(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create lambda engine: %w", err)
	}

	h := &DynamicModuleHandler{
		configs:      make(map[string]*ModuleConfig),
		db:           db,
		renderer:     renderer,
		modulesPath:  modulesPath,
		lambdaEngine: lambdaEngine,
		ctx:          ctx,
		i18n:         i18n.GetInstance(),
	}

	// Load all module configs
	if err := h.loadAllConfigs(); err != nil {
		return nil, fmt.Errorf("failed to load configs: %w", err)
	}

	// Setup file watcher
	if err := h.setupWatcher(); err != nil {
		return nil, fmt.Errorf("failed to setup watcher: %w", err)
	}

	return h, nil
}

// loadAllConfigs loads all YAML configs from modules directory.
func (h *DynamicModuleHandler) loadAllConfigs() error {
	files, err := os.ReadDir(h.modulesPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
			configPath := filepath.Join(h.modulesPath, file.Name())
			if err := h.loadConfig(configPath); err != nil {
				fmt.Printf("Warning: Failed to load %s: %v\n", file.Name(), err)
			}
		}
	}

	fmt.Printf("Loaded %d module configurations\n", len(h.configs))
	return nil
}

// loadConfig loads a single YAML config.
func (h *DynamicModuleHandler) loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config ModuleConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	h.mu.Lock()
	h.configs[config.Module.Name] = &config
	h.mu.Unlock()

	fmt.Printf("Loaded module: %s\n", config.Module.Name)
	return nil
}

// setupWatcher sets up file system watcher for hot reload.
func (h *DynamicModuleHandler) setupWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	h.watcher = watcher

	// Watch the modules directory
	if err := watcher.Add(h.modulesPath); err != nil {
		return err
	}

	// Start watching in background
	go h.watchFiles()

	return nil
}

// watchFiles watches for file changes.
func (h *DynamicModuleHandler) watchFiles() {
	for {
		select {
		case event, ok := <-h.watcher.Events:
			if !ok {
				return
			}

			if strings.HasSuffix(event.Name, ".yaml") {
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					fmt.Printf("Module config changed: %s\n", event.Name)
					h.loadConfig(event.Name)
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					// Remove config
					moduleName := strings.TrimSuffix(filepath.Base(event.Name), ".yaml")
					h.mu.Lock()
					delete(h.configs, moduleName)
					h.mu.Unlock()
					fmt.Printf("Module removed: %s\n", moduleName)
				}
			}

		case err, ok := <-h.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Watcher error: %v\n", err)
		}
	}
}

// resolveTranslation resolves a translation key if it starts with @.
func (h *DynamicModuleHandler) resolveTranslation(value string, lang string) string {
	if strings.HasPrefix(value, "@") {
		raw := strings.TrimPrefix(value, "@")
		parts := strings.SplitN(raw, "|", 2)
		key := parts[0]
		fallback := ""
		if len(parts) == 2 {
			fallback = parts[1]
		}
		translated := h.i18n.T(lang, key)
		// If translation not found (returns the key), use the original value as fallback
		if translated == key && strings.Contains(value, ".") {
			if fallback != "" {
				return fallback
			}
			// Try to extract a reasonable fallback from the key
			parts := strings.Split(key, ".")
			if len(parts) > 0 {
				fallback := parts[len(parts)-1]
				// Convert snake_case or camelCase to Title Case
				fallback = strings.ReplaceAll(fallback, "_", " ")
				fallback = strings.ReplaceAll(fallback, "-", " ")
				// Capitalize first letter of each word
				words := strings.Fields(fallback)
				for i, word := range words {
					if len(word) > 0 {
						words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
					}
				}
				return strings.Join(words, " ")
			}
		}
		return translated
	}
	return value
}

// resolveConfigTranslations processes a module config and resolves all translation keys.
func (h *DynamicModuleHandler) resolveConfigTranslations(config *ModuleConfig, lang string) *ModuleConfig {
	// Create a deep copy to avoid modifying the original
	resolved := *config

	// Resolve module-level translations
	resolved.Module.Singular = h.resolveTranslation(config.Module.Singular, lang)
	resolved.Module.Plural = h.resolveTranslation(config.Module.Plural, lang)
	resolved.Module.Description = h.resolveTranslation(config.Module.Description, lang)

	// Resolve field translations
	resolved.Fields = make([]Field, len(config.Fields))
	for i, field := range config.Fields {
		resolved.Fields[i] = field
		resolved.Fields[i].Label = h.resolveTranslation(field.Label, lang)
		resolved.Fields[i].Help = h.resolveTranslation(field.Help, lang)
		resolved.Fields[i].Placeholder = h.resolveTranslation(field.Placeholder, lang)

		// Resolve options
		if len(field.Options) > 0 {
			resolved.Fields[i].Options = make([]Option, len(field.Options))
			for j, opt := range field.Options {
				resolved.Fields[i].Options[j] = Option{
					Value: opt.Value,
					Label: h.resolveTranslation(opt.Label, lang),
					Group: opt.Group,
				}
			}
		}
	}

	// Resolve computed field translations
	resolved.ComputedFields = make([]ComputedField, len(config.ComputedFields))
	for i, field := range config.ComputedFields {
		resolved.ComputedFields[i] = field
		resolved.ComputedFields[i].Label = h.resolveTranslation(field.Label, lang)
	}

	// Resolve filter translations
	resolved.Filters = make([]Filter, len(config.Filters))
	for i, filter := range config.Filters {
		resolved.Filters[i] = filter
		resolved.Filters[i].Label = h.resolveTranslation(filter.Label, lang)

		// Resolve filter options
		if len(filter.Options) > 0 {
			resolved.Filters[i].Options = make([]FilterOption, len(filter.Options))
			for j, opt := range filter.Options {
				resolved.Filters[i].Options[j] = FilterOption{
					Value: opt.Value,
					Label: h.resolveTranslation(opt.Label, lang),
				}
			}
		}
	}

	return &resolved
}

// ServeModule handles requests for any dynamic module.
func (h *DynamicModuleHandler) ServeModule(c *gin.Context) {
	moduleName := c.Param("module")

	// Special case for schema discovery
	if moduleName == "_schema" {
		h.handleSchemaDiscovery(c)
		return
	}

	h.mu.RLock()
	config, exists := h.configs[moduleName]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("Module '%s' not found", moduleName),
		})
		return
	}

	// Route to appropriate handler based on method and path
	id := c.Param("id")
	action := c.Param("action")

	// Check for export which comes as id="export"
	if id == "export" {
		h.handleExport(c, config)
		return
	}

	switch c.Request.Method {
	case "GET":
		if action != "" && id != "" {
			h.handleAction(c, config, id, action)
		} else if id != "" {
			h.handleGet(c, config, id)
		} else {
			h.handleList(c, config)
		}
	case "POST":
		if action != "" && id != "" {
			h.handleAction(c, config, id, action)
		} else {
			h.handleCreate(c, config)
		}
	case "PUT":
		if id != "" {
			h.handleUpdate(c, config, id)
		}
	case "DELETE":
		if id != "" {
			h.handleDelete(c, config, id)
		}
	}
}

// handleList handles listing all records.
func (h *DynamicModuleHandler) handleList(c *gin.Context, config *ModuleConfig) {
	// Get current language
	lang := middleware.GetLanguage(c)

	// Resolve translations in config
	config = h.resolveConfigTranslations(config, lang)

	// Get pagination parameters
	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 25 // Default page size
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	// Build SELECT query
	columns := h.getSelectColumns(config)
	baseQuery := fmt.Sprintf("SELECT %s FROM %s", columns, config.Module.Table)

	// Apply filters based on request parameters
	args := []interface{}{}
	whereClause, filterArgs := h.buildFilterWhereClause(c, config)
	if whereClause != "" {
		baseQuery += " WHERE " + whereClause
		args = append(args, filterArgs...)
	}

	// Don't filter by valid_id - show all records so users can enable/disable them
	// if config.Features.SoftDelete {
	//     if whereClause != "" {
	//         baseQuery += " AND valid_id = 1"
	//     } else {
	//         baseQuery += " WHERE valid_id = 1"
	//     }
	// }

	// Count total records for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", config.Module.Table)
	if whereClause != "" {
		countQuery += " WHERE " + whereClause
	}

	var totalCount int
	err := h.queryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		fmt.Printf("Error counting records: %v\n", err)
		totalCount = 0
	}

	// Calculate pagination values
	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	// Add ordering and pagination to query
	query := baseQuery + " ORDER BY id DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, pageSize, offset)

	rows, err := h.query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	items := h.scanRows(rows, config)

	// Process lookups for foreign key fields
	h.processLookups(items, config)

	// Process computed fields for all items
	h.processComputedFields(items, config)

	// Check if this is an API request
	if h.isAPIRequest(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    items,
			"total":   len(items),
		})
		return
	}
	// Populate database-sourced filter options
	for i, filter := range config.Filters {
		if filter.Source == "database" && filter.Query != "" {
			filterRows, err := h.query(filter.Query)
			if err != nil {
				fmt.Printf("Error loading filter options for %s: %v\n", filter.Field, err)
				continue
			}
			defer filterRows.Close()

			var options []FilterOption
			// Add "All" option first
			options = append(options, FilterOption{
				Value: "",
				Label: fmt.Sprintf("All %s", filter.Label),
			})

			for filterRows.Next() {
				var value, label string
				if err := filterRows.Scan(&value, &label); err != nil {
					fmt.Printf("Error scanning filter row: %v\n", err)
					continue
				}
				options = append(options, FilterOption{
					Value: value,
					Label: label,
				})
			}

			// Update the filter with the loaded options
			config.Filters[i].Options = options
		}
	}

	// Render the universal template
	tmpl, err := h.renderer.FromFile("pages/admin/dynamic_module.pongo2")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template not found: " + err.Error()})
		return
	}

	// Convert config to JSON string for JavaScript (exclude lambda functions)
	safeConfig := map[string]interface{}{
		"module":   config.Module,
		"features": config.Features,
		"fields":   []map[string]interface{}{},
		"filters":  []map[string]interface{}{},
	}

	// Copy fields without lambda functions
	for _, field := range config.Fields {
		safeField := map[string]interface{}{
			"Name":       field.Name,
			"Type":       field.Type,
			"Label":      field.Label,
			"Required":   field.Required,
			"ShowInForm": field.ShowInForm,
			"ShowInList": field.ShowInList,
		}
		safeConfig["fields"] = append(safeConfig["fields"].([]map[string]interface{}), safeField)
	}

	// Copy filters for JavaScript access
	for _, filter := range config.Filters {
		safeFilter := map[string]interface{}{
			"field": filter.Field,
			"type":  filter.Type,
			"label": filter.Label,
		}
		safeConfig["filters"] = append(safeConfig["filters"].([]map[string]interface{}), safeFilter)
	}

	configJSON, _ := json.Marshal(safeConfig)

	// Populate lookup options for form fields
	h.populateLookupOptions(config)

	// Template rendering with data
	// Get user from context for navigation - match the format used in getUserMapForTemplate
	userMap := make(map[string]interface{})
	if userID, exists := c.Get("user_id"); exists {
		userMap["ID"] = userID
		userMap["id"] = userID // Also provide lowercase for compatibility
	}
	if userEmail, exists := c.Get("user_email"); exists {
		userMap["Email"] = userEmail
		userMap["Login"] = userEmail // Use email as login
		userMap["email"] = userEmail // Also provide lowercase
	}
	if userRole, exists := c.Get("user_role"); exists {
		userMap["Role"] = userRole
		userMap["IsAdmin"] = (userRole == "Admin")
		userMap["role"] = userRole // Also provide lowercase
	}
	if userName, exists := c.Get("user_name"); exists {
		// Parse name into first and last
		nameParts := strings.Fields(fmt.Sprintf("%v", userName))
		firstName := ""
		lastName := ""
		if len(nameParts) > 0 {
			firstName = nameParts[0]
		}
		if len(nameParts) > 1 {
			lastName = strings.Join(nameParts[1:], " ")
		}
		userMap["FirstName"] = firstName
		userMap["LastName"] = lastName
		userMap["name"] = userName // Also provide lowercase
	}
	if isDemo, exists := c.Get("is_demo"); exists {
		userMap["is_demo"] = isDemo
		userMap["IsDemo"] = isDemo
	}
	// Set defaults for active status
	userMap["IsActive"] = true

	// Prepare all fields for display (combine regular and computed fields)
	allFields := make([]interface{}, 0)

	// If UI.ListColumns is specified, use that order
	if len(config.UI.ListColumns) > 0 {
		for _, colName := range config.UI.ListColumns {
			// Check regular fields
			for _, field := range config.Fields {
				if field.Name == colName && field.ShowInList {
					allFields = append(allFields, field)
					break
				}
			}
			// Check computed fields
			for _, field := range config.ComputedFields {
				if field.Name == colName && field.ShowInList {
					allFields = append(allFields, field)
					break
				}
			}
		}
	} else {
		// Fall back to all fields marked as show_in_list
		for _, field := range config.Fields {
			if field.ShowInList {
				allFields = append(allFields, field)
			}
		}
		for _, field := range config.ComputedFields {
			if field.ShowInList {
				allFields = append(allFields, field)
			}
		}
	}

	// Populate options for filters with database source
	for i, filter := range config.Filters {
		if filter.Source == "database" && filter.LookupTable != "" {
			// Load options from database
			query := fmt.Sprintf("SELECT %s, %s FROM %s ORDER BY %s",
				filter.LookupKey, filter.LookupDisplay, filter.LookupTable, filter.LookupDisplay)
			rows, err := h.query(query)
			if err == nil {
				defer rows.Close()
				options := []FilterOption{{Value: "", Label: fmt.Sprintf("All %s", filter.Label)}}
				for rows.Next() {
					var key, display string
					if err := rows.Scan(&key, &display); err == nil {
						options = append(options, FilterOption{Value: key, Label: display})
					}
				}
				config.Filters[i].Options = options
			}
		}
	}

	// Collect current filter values from URL parameters
	currentFilters := make(map[string]string)
	for _, filter := range config.Filters {
		if filter.Type == "date_range" {
			// For date range filters, collect both from and to values
			fromValue := c.Query("filter_" + filter.Field + "_from")
			toValue := c.Query("filter_" + filter.Field + "_to")
			if fromValue != "" {
				currentFilters["filter_"+filter.Field+"_from"] = fromValue
			}
			if toValue != "" {
				currentFilters["filter_"+filter.Field+"_to"] = toValue
			}
		} else {
			filterValue := c.Query("filter_" + filter.Field)
			if filterValue != "" {
				currentFilters["filter_"+filter.Field] = filterValue
			}
		}
	}
	// Also get search value
	searchValue := c.Query("search")
	if searchValue != "" {
		currentFilters["search"] = searchValue
	}

	// Generate page range for pagination display
	pageRange := []int{}
	startPage := page - 2
	if startPage < 1 {
		startPage = 1
	}
	endPage := startPage + 4
	if endPage > totalPages {
		endPage = totalPages
		startPage = endPage - 4
		if startPage < 1 {
			startPage = 1
		}
	}
	for i := startPage; i <= endPage; i++ {
		pageRange = append(pageRange, i)
	}

	html, err := tmpl.Execute(pongo2.Context{
		"config":         config,
		"config_json":    string(configJSON),
		"items":          items,
		"module":         config.Module,
		"fields":         config.Fields,
		"allFields":      allFields, // Combined fields for display
		"features":       config.Features,
		"filters":        config.Filters, // Pass filters configuration
		"currentFilters": currentFilters, // Pass current filter values
		"pagination": map[string]interface{}{
			"enabled":    true,
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": totalCount,
			"totalPages": totalPages,
			"pageRange":  pageRange,
			"hasNext":    page < totalPages,
			"hasPrev":    page > 1,
		},
		"User":       userMap, // Required for base template
		"ActivePage": "admin", // For navigation highlighting
		"Title":      fmt.Sprintf("%s Management", config.Module.Plural),
		// Add translation function
		"t": func(key string, args ...interface{}) string {
			return h.i18n.T(lang, key, args...)
		},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// handleExport handles exporting data to CSV with current filters.
func (h *DynamicModuleHandler) handleExport(c *gin.Context, config *ModuleConfig) {
	if !config.Features.ExportCSV {
		c.JSON(http.StatusForbidden, gin.H{"error": "Export not enabled for this module"})
		return
	}

	// Build query with filters
	columns := h.getSelectColumns(config)
	query := fmt.Sprintf("SELECT %s FROM %s", columns, config.Module.Table)

	// Check if specific IDs were requested
	idsParam := c.Query("ids")
	if idsParam != "" {
		// Export only selected items
		ids := strings.Split(idsParam, ",")
		placeholders := make([]string, len(ids))
		args := make([]interface{}, len(ids))
		for i, id := range ids {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = strings.TrimSpace(id)
		}
		query += " WHERE id IN (" + strings.Join(placeholders, ", ") + ")"
		query += " ORDER BY id DESC"

		rows, err := h.query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		items := h.scanRows(rows, config)
		h.processLookups(items, config)
		h.processComputedFields(items, config)
		h.generateCSVResponse(c, config, items)
		return
	}

	// Apply filters based on request parameters (same as handleList)
	args := []interface{}{}
	whereClause, filterArgs := h.buildFilterWhereClause(c, config)
	if whereClause != "" {
		query += " WHERE " + whereClause
		args = append(args, filterArgs...)
	}

	query += " ORDER BY id DESC"

	rows, err := h.query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	items := h.scanRows(rows, config)

	// Process lookups for foreign key fields
	h.processLookups(items, config)

	// Process computed fields for all items
	h.processComputedFields(items, config)

	// Generate CSV
	var csvData bytes.Buffer
	csvWriter := csv.NewWriter(&csvData)

	// Write header row
	var headers []string
	for _, field := range config.Fields {
		if field.ShowInList {
			headers = append(headers, field.Label)
		}
	}
	csvWriter.Write(headers)

	// Write data rows
	for _, item := range items {
		var row []string
		for _, field := range config.Fields {
			if field.ShowInList {
				value := ""
				// Check for display value first
				displayKey := field.Name + "_display"
				if displayVal, ok := item[displayKey]; ok && displayVal != nil {
					value = fmt.Sprintf("%v", displayVal)
				} else if val, ok := item[field.Name]; ok && val != nil {
					value = fmt.Sprintf("%v", val)
				}
				row = append(row, value)
			}
		}
		csvWriter.Write(row)
	}

	csvWriter.Flush()

	// Set headers for download
	filename := fmt.Sprintf("%s_export_%s.csv", config.Module.Name, time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "text/csv", csvData.Bytes())
}

// handleGet handles getting a single record.
func (h *DynamicModuleHandler) handleGet(c *gin.Context, config *ModuleConfig, id string) {
	// Get current language and resolve translations
	lang := middleware.GetLanguage(c)
	config = h.resolveConfigTranslations(config, lang)

	columns := h.getSelectColumns(config)
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", columns, config.Module.Table)

	row := h.queryRow(query, id)
	item := h.scanRow(row, config)

	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("%s not found", config.Module.Singular),
		})
		return
	}

	// For users module, fetch the user's groups
	if config.Module.Name == "users" {
		groupQuery := `
			SELECT DISTINCT g.name 
			FROM groups g 
			INNER JOIN user_groups ug ON g.id = ug.group_id 
			WHERE ug.user_id = $1
			ORDER BY g.name`

		rows, err := h.query(groupQuery, id)
		if err == nil {
			defer rows.Close()
			groups := []string{}
			for rows.Next() {
				var groupName string
				if err := rows.Scan(&groupName); err == nil {
					groups = append(groups, groupName)
				}
			}
			item["Groups"] = groups
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    item,
	})
}

// handleCreate handles creating a new record.
func (h *DynamicModuleHandler) handleCreate(c *gin.Context, config *ModuleConfig) {
	// Get current language and resolve translations
	lang := middleware.GetLanguage(c)
	config = h.resolveConfigTranslations(config, lang)

	data := h.parseFormData(c, config)
	h.applyModuleWriteTransforms(config, data)

	// Get current user ID for audit fields
	userIDValue, exists := c.Get("user_id")
	var currentUserID int
	if exists {
		if uid, ok := userIDValue.(uint); ok {
			currentUserID = int(uid)
		} else if uid, ok := userIDValue.(int); ok {
			currentUserID = uid
		}
	}
	if currentUserID == 0 {
		currentUserID = 1 // Default to admin user if not found
	}

	// Build INSERT query
	columns := []string{}
	placeholders := []string{}
	values := []interface{}{}
	validIncluded := false

	for _, field := range config.Fields {
		if field.Virtual {
			continue
		}
		if field.DBColumn == "id" {
			continue
		}

		// Check for audit fields
		if field.DBColumn == "create_by" {
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, currentUserID)
		} else if field.DBColumn == "change_by" {
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, currentUserID)
		} else if field.DBColumn == "create_time" {
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, time.Now())
		} else if field.DBColumn == "change_time" {
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, time.Now())
		} else if field.DBColumn == "valid_id" {
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, field.Default)
			validIncluded = true
		} else if field.ShowInForm {
			if value, exists := data[field.Name]; exists {
				columns = append(columns, field.DBColumn)

				// Special handling for password fields - hash in Go instead of PostgreSQL
				if field.Type == "password" && field.DBColumn == "pw" && config.Module.Name == "users" {
					// Hash the password in Go using SHA256
					if strValue, ok := value.(string); ok && strValue != "" {
						hashedPassword := hashPassword(strValue)
						placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
						values = append(values, hashedPassword)
					} else {
						placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
						values = append(values, value)
					}
				} else {
					placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
					values = append(values, value)
				}
			} else if field.Default != nil {
				columns = append(columns, field.DBColumn)
				placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
				values = append(values, field.Default)
			}
		} else if value, exists := data[field.Name]; exists {
			// Support hidden/virtual fields that populate real columns (e.g., mail_account.trusted)
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, value)
		} else if field.Default != nil {
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, field.Default)
		}
	}

	// Add valid_id if soft delete is enabled and not already supplied
	if config.Features.SoftDelete && !validIncluded {
		columns = append(columns, "valid_id")
		placeholders = append(placeholders, "1")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		config.Module.Table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	query, useLastInsert := database.ConvertReturning(query)

	var newID int64
	var err error
	if useLastInsert && database.IsMySQL() {
		res, err := h.exec(query, values...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		lastID, err := res.LastInsertId()
		if err != nil {
			fallbackRow := h.queryRow("SELECT LAST_INSERT_ID()")
			if fallbackErr := fallbackRow.Scan(&lastID); fallbackErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fallbackErr.Error()})
				return
			}
		}
		newID = lastID
	} else {
		err = h.queryRow(query, values...).Scan(&newID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Handle group assignments for users module
	if config.Module.Name == "users" {
		if groupsStr := c.PostForm("groups"); groupsStr != "" {
			// Parse the selected group IDs
			groupIDs := strings.Split(groupsStr, ",")
			for _, groupID := range groupIDs {
				if groupID != "" {
					_, err = h.exec(`
						INSERT INTO user_groups (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
						VALUES ($1, $2, 'rw', 1, CURRENT_TIMESTAMP, $3, CURRENT_TIMESTAMP, $3)`,
						newID, groupID, currentUserID)
					if err != nil {
						// Log but don't fail the whole operation
						fmt.Printf("Warning: Failed to add user to group %s: %v\n", groupID, err)
					}
				}
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s created successfully", config.Module.Singular),
	})
}

// handleUpdate handles updating a record.
func (h *DynamicModuleHandler) handleUpdate(c *gin.Context, config *ModuleConfig, id string) {
	// Get current language and resolve translations
	lang := middleware.GetLanguage(c)
	config = h.resolveConfigTranslations(config, lang)

	data := h.parseFormData(c, config)
	h.applyModuleWriteTransforms(config, data)

	// Get current user ID for audit fields
	userIDValue, exists := c.Get("user_id")
	var currentUserID int
	if exists {
		if uid, ok := userIDValue.(uint); ok {
			currentUserID = int(uid)
		} else if uid, ok := userIDValue.(int); ok {
			currentUserID = uid
		}
	}
	if currentUserID == 0 {
		currentUserID = 1 // Default to admin user if not found
	}

	// Build UPDATE query
	sets := []string{}
	values := []interface{}{}

	for _, field := range config.Fields {
		if field.Virtual {
			continue
		}
		if field.DBColumn == "id" {
			continue
		}

		// Always update change_by if it exists
		if field.DBColumn == "change_by" {
			sets = append(sets, fmt.Sprintf("%s = $%d", field.DBColumn, len(values)+1))
			values = append(values, currentUserID)
		} else if field.DBColumn == "change_time" {
			// Update change_time to current timestamp
			sets = append(sets, fmt.Sprintf("%s = CURRENT_TIMESTAMP", field.DBColumn))
		} else if field.ShowInForm {
			if value, exists := data[field.Name]; exists {
				// Special handling for password fields - hash in Go
				if field.Type == "password" && field.DBColumn == "pw" && config.Module.Name == "users" {
					// Only update password if a new value was provided (not empty)
					if strValue, ok := value.(string); ok && strValue != "" {
						// Hash the password in Go using SHA256
						hashedPassword := hashPassword(strValue)
						sets = append(sets, fmt.Sprintf("%s = $%d", field.DBColumn, len(values)+1))
						values = append(values, hashedPassword)
					}
					// Skip updating password if empty (preserves existing password)
				} else {
					sets = append(sets, fmt.Sprintf("%s = $%d", field.DBColumn, len(values)+1))
					values = append(values, value)
				}
			}
		} else if value, exists := data[field.Name]; exists {
			// Allow hidden/non-form fields from transforms (e.g., trusted) to be persisted
			sets = append(sets, fmt.Sprintf("%s = $%d", field.DBColumn, len(values)+1))
			values = append(values, value)
		}
	}

	values = append(values, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
		config.Module.Table,
		strings.Join(sets, ", "),
		len(values))

	_, err := h.exec(query, values...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Handle group assignments for users module
	if config.Module.Name == "users" {
		// Check if groups were actually submitted in the form
		groupsStr, groupsSubmitted := c.GetPostForm("groups")

		// Only update groups if they were explicitly submitted
		if groupsSubmitted {
			// First, remove all existing group assignments
			_, err = h.exec("DELETE FROM user_groups WHERE user_id = $1", id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update groups: " + err.Error()})
				return
			}

			// Add new group assignments
			if groupsStr != "" {
				// Parse the selected group IDs
				groupIDs := strings.Split(groupsStr, ",")
				for _, groupID := range groupIDs {
					if groupID != "" {
						_, err = h.exec(`
							INSERT INTO user_groups (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
							VALUES ($1, $2, 'rw', 1, CURRENT_TIMESTAMP, $3, CURRENT_TIMESTAMP, $3)`,
							id, groupID, currentUserID)
						if err != nil {
							// Log but don't fail the whole operation
							fmt.Printf("Warning: Failed to add user to group %s: %v\n", groupID, err)
						}
					}
				}
			}
		}
		// If groups field wasn't submitted, preserve existing group memberships
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s updated successfully", config.Module.Singular),
	})
}

// handleDelete handles deleting (or soft deleting) a record.
func (h *DynamicModuleHandler) handleDelete(c *gin.Context, config *ModuleConfig, id string) {
	// Get current language and resolve translations
	lang := middleware.GetLanguage(c)
	config = h.resolveConfigTranslations(config, lang)

	var query string

	if config.Features.SoftDelete {
		query = fmt.Sprintf("UPDATE %s SET valid_id = 2 WHERE id = $1", config.Module.Table)
	} else {
		query = fmt.Sprintf("DELETE FROM %s WHERE id = $1", config.Module.Table)
	}

	_, err := h.exec(query, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	action := "deleted"
	if config.Features.SoftDelete {
		action = "deactivated"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s %s successfully", config.Module.Singular, action),
	})
}

// Helper methods

func (h *DynamicModuleHandler) getSelectColumns(config *ModuleConfig) string {
	columns := []string{}
	for _, field := range config.Fields {
		if field.Virtual || strings.TrimSpace(field.DBColumn) == "" {
			columns = append(columns, fmt.Sprintf("NULL AS %s", field.Name))
			continue
		}
		columns = append(columns, field.DBColumn)
	}
	return strings.Join(columns, ", ")
}

func (h *DynamicModuleHandler) scanRows(rows *sql.Rows, config *ModuleConfig) []map[string]interface{} {
	items := []map[string]interface{}{}

	for rows.Next() {
		// Create scanners for each field
		scanners := make([]interface{}, len(config.Fields))
		for i := range scanners {
			var val interface{}
			scanners[i] = &val
		}

		if err := rows.Scan(scanners...); err != nil {
			continue
		}

		// Build item map
		item := make(map[string]interface{})
		for i, field := range config.Fields {
			if ptr, ok := scanners[i].(*interface{}); ok && *ptr != nil {
				item[field.Name] = normalizeScannedValue(*ptr)
			}
		}

		items = append(items, item)
	}

	if len(items) > 0 {
		h.applyModuleReadTransforms(config, items)
	}
	return items
}

func (h *DynamicModuleHandler) scanRow(row *sql.Row, config *ModuleConfig) map[string]interface{} {
	scanners := make([]interface{}, len(config.Fields))
	for i := range scanners {
		var val interface{}
		scanners[i] = &val
	}

	if err := row.Scan(scanners...); err != nil {
		return nil
	}

	item := make(map[string]interface{})
	for i, field := range config.Fields {
		if ptr, ok := scanners[i].(*interface{}); ok && *ptr != nil {
			item[field.Name] = normalizeScannedValue(*ptr)
		}
	}

	h.applyModuleReadTransforms(config, []map[string]interface{}{item})
	return item
}

func (h *DynamicModuleHandler) parseFormData(c *gin.Context, config *ModuleConfig) map[string]interface{} {
	data := make(map[string]interface{})

	if c.ContentType() == "application/json" {
		c.ShouldBindJSON(&data)
	} else {
		// Parse form data
		for _, field := range config.Fields {
			if value := c.PostForm(field.Name); value != "" {
				data[field.Name] = h.convertValue(value, field.Type)
			}
		}
	}

	return data
}

func (h *DynamicModuleHandler) applyModuleWriteTransforms(config *ModuleConfig, data map[string]interface{}) {
	if config == nil || data == nil {
		return
	}
	switch config.Module.Name {
	case "mail_account":
		applyMailAccountWriteTransform(data)
	}
}

func (h *DynamicModuleHandler) applyModuleReadTransforms(config *ModuleConfig, items []map[string]interface{}) {
	if config == nil || len(items) == 0 {
		return
	}
	switch config.Module.Name {
	case "mail_account":
		for _, item := range items {
			applyMailAccountReadTransform(item)
		}
	}
}

func (h *DynamicModuleHandler) convertValue(value string, fieldType string) interface{} {
	// Type conversion logic
	switch fieldType {
	case "int", "integer":
		var i int
		fmt.Sscanf(value, "%d", &i)
		return i
	case "float", "decimal":
		var f float64
		fmt.Sscanf(value, "%f", &f)
		return f
	case "bool", "boolean":
		return value == "true" || value == "1"
	default:
		return value
	}
}

func applyMailAccountWriteTransform(data map[string]interface{}) {
	if data == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(coerceString(data["dispatching_mode"])))
	if mode == "" {
		mode = "queue"
	}
	var meta mailaccountmeta.Metadata
	if mode != "" && mode != "queue" {
		meta.DispatchingMode = mode
		data["queue_id"] = 0
	}
	allow := false
	if raw, ok := data["allow_trusted_headers"]; ok {
		if v, _ := coerceBool(raw); v {
			allow = true
		}
	}
	if allow {
		flag := true
		meta.AllowTrustedHeaders = &flag
		data["trusted"] = 1
	} else {
		data["trusted"] = 0
	}
	if poll, ok := coerceInt(data["poll_interval_seconds"]); ok && poll > 0 {
		interval := poll
		meta.PollIntervalSeconds = &interval
	}
	base := coerceString(data["comments"])
	data["comments"] = mailaccountmeta.EncodeComment(base, meta)
	delete(data, "dispatching_mode")
	delete(data, "allow_trusted_headers")
	delete(data, "poll_interval_seconds")
}

func applyMailAccountReadTransform(item map[string]interface{}) {
	if item == nil {
		return
	}
	comment := coerceString(item["comments"])
	if comment != "" {
		base, meta := mailaccountmeta.DecodeComment(comment)
		item["comments"] = base
		applyMailAccountMetadataFields(item, meta)
		return
	}
	applyMailAccountMetadataFields(item, mailaccountmeta.Metadata{})
}

func applyMailAccountMetadataFields(item map[string]interface{}, meta mailaccountmeta.Metadata) {
	mode := meta.DispatchingMode
	if mode == "" {
		if queueID, ok := coerceInt(item["queue_id"]); ok && queueID == 0 {
			mode = "from"
		} else {
			mode = "queue"
		}
	}
	item["dispatching_mode"] = mode
	allow := false
	if meta.AllowTrustedHeaders != nil {
		allow = *meta.AllowTrustedHeaders
	} else if trusted, ok := coerceInt(item["trusted"]); ok {
		allow = trusted != 0
	}
	item["allow_trusted_headers"] = allow
	if meta.PollIntervalSeconds != nil {
		item["poll_interval_seconds"] = *meta.PollIntervalSeconds
	} else {
		item["poll_interval_seconds"] = 0
	}
}

func coerceString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func normalizeScannedValue(value interface{}) interface{} {
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return v
	}
}

func coerceInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		if n, err := strconv.Atoi(trimmed); err == nil {
			return n, true
		}
	case []byte:
		if n, err := strconv.Atoi(strings.TrimSpace(string(v))); err == nil {
			return n, true
		}
	}
	return 0, false
}

func coerceBool(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true, true
		case "0", "false", "no", "off", "":
			return false, true
		}
	case []byte:
		return coerceBool(string(v))
	case int:
		return v != 0, true
	case int32:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	}
	return false, false
}

func (h *DynamicModuleHandler) isAPIRequest(c *gin.Context) bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(c.GetHeader("Accept"), "application/json")
}

// GetAvailableModules returns list of loaded modules.
func (h *DynamicModuleHandler) GetAvailableModules() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	modules := []string{}
	for name := range h.configs {
		modules = append(modules, name)
	}
	return modules
}

// handleSchemaDiscovery handles schema discovery requests.
func (h *DynamicModuleHandler) handleSchemaDiscovery(c *gin.Context) {
	action := c.Query("action")
	tableName := c.Query("table")

	discovery := NewSchemaDiscovery(h.db)

	switch action {
	case "tables":
		// List all tables
		tables, err := discovery.GetTables()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    tables,
		})

	case "columns":
		// Get columns for a specific table
		if tableName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "table parameter required"})
			return
		}
		columns, err := discovery.GetTableColumns(tableName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"table":   tableName,
			"data":    columns,
		})

	case "generate":
		// Generate module config for a table
		if tableName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "table parameter required"})
			return
		}
		config, err := discovery.GenerateModuleConfig(tableName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Convert to YAML if requested
		if c.Query("format") == "yaml" {
			yamlData, err := yaml.Marshal(config)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.Data(http.StatusOK, "text/yaml", yamlData)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"config":  config,
		})

	case "save":
		// Save generated config to file
		if tableName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "table parameter required"})
			return
		}

		config, err := discovery.GenerateModuleConfig(tableName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Save to modules directory
		filename := filepath.Join(h.modulesPath, fmt.Sprintf("%s.yaml", tableName))
		yamlData, err := yaml.Marshal(config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := os.WriteFile(filename, yamlData, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  fmt.Sprintf("Module config saved to %s", filename),
			"filename": filename,
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid action. Use: tables, columns, generate, or save",
		})
	}
}

// populateLookupOptions populates Options for fields with lookup tables.
func (h *DynamicModuleHandler) populateLookupOptions(config *ModuleConfig) {
	for i, field := range config.Fields {
		if field.LookupTable != "" && field.ShowInForm {
			keyColumn := field.LookupKey
			if keyColumn == "" {
				keyColumn = "id"
			}
			displayColumn := field.LookupDisplay
			if displayColumn == "" {
				displayColumn = "name"
			}

			baseQuery := fmt.Sprintf("SELECT %s, %s FROM %s", keyColumn, displayColumn, field.LookupTable)
			orderClause := fmt.Sprintf(" ORDER BY %s", displayColumn)
			condition := strings.TrimSpace(field.LookupCondition)
			if condition == "" {
				condition = "valid_id = 1"
			}

			query := baseQuery
			if condition != "" {
				query += " WHERE " + condition
			}
			query += orderClause

			rows, err := h.query(query)
			if err != nil && field.LookupCondition == "" && strings.Contains(strings.ToLower(err.Error()), "unknown column 'valid_id'") {
				rows, err = h.query(baseQuery + orderClause)
			}
			if err != nil {
				fmt.Printf("Error loading lookup options for %s: %v\n", field.Name, err)
				continue
			}

			options := []Option{}
			for rows.Next() {
				var value, label string
				if err := rows.Scan(&value, &label); err == nil {
					options = append(options, Option{
						Value: value,
						Label: label,
					})
				}
			}
			rows.Close()

			config.Fields[i].Options = options
			config.Fields[i].Type = "select"
		}
	}
}

// processLookups resolves foreign key lookups for display.
func (h *DynamicModuleHandler) processLookups(items []map[string]interface{}, config *ModuleConfig) {
	for _, field := range config.Fields {
		// Skip if no lookup table defined
		if field.LookupTable == "" {
			continue
		}

		fmt.Printf("DEBUG: Processing lookup for field %s with table %s, display_as=%s\n", field.Name, field.LookupTable, field.DisplayAs)

		// Collect unique IDs to lookup (normalize to string for consistent matching across drivers)
		idMap := make(map[string]bool)
		for _, item := range items {
			if val, exists := item[field.Name]; exists && val != nil {
				idMap[coerceString(val)] = true
			}
		}

		if len(idMap) == 0 {
			continue
		}

		// Build lookup query
		lookupKey := field.LookupKey
		if lookupKey == "" {
			lookupKey = "id"
		}
		lookupDisplay := field.LookupDisplay
		if lookupDisplay == "" {
			lookupDisplay = "name"
		}

		// Create ID list for IN clause
		var ids []string
		for id := range idMap {
			ids = append(ids, fmt.Sprintf("'%s'", strings.ReplaceAll(id, "'", "''")))
		}

		query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s IN (%s)",
			lookupKey, lookupDisplay, field.LookupTable, lookupKey, strings.Join(ids, ","))

		rows, err := h.query(query)
		if err != nil {
			fmt.Printf("Lookup query error for field %s: %v\n", field.Name, err)
			continue
		}
		defer rows.Close()

		// Build lookup map
		lookupMap := make(map[string]string)
		for rows.Next() {
			var id interface{}
			var displayValue string
			if err := rows.Scan(&id, &displayValue); err == nil {
				lookupMap[coerceString(id)] = displayValue
			}
		}

		// Update items with lookup values
		lookupFieldName := field.Name + "_display"
		for _, item := range items {
			if val, exists := item[field.Name]; exists && val != nil {
				key := coerceString(val)
				if displayVal, found := lookupMap[key]; found {
					item[lookupFieldName] = displayVal
					// If display_as is chip, store the display configuration
					if field.DisplayAs == "chip" {
						chipKey := field.Name + "_chip"
						// Determine chip type based on field name or lookup table
						chipType := "default"
						if strings.Contains(strings.ToLower(field.Name), "group") || field.LookupTable == "groups" {
							chipType = "group"
						} else if strings.Contains(strings.ToLower(field.Name), "valid") || field.LookupTable == "valid" {
							chipType = "status"
						}

						item[chipKey] = map[string]interface{}{
							"value":      val,
							"label":      displayVal,
							"display_as": "chip",
							"type":       chipType,
						}
						fmt.Printf("DEBUG: Added chip for field %s: %s = %v\n", field.Name, chipKey, item[chipKey])
					}
				}
			}
		}
	}
}

// processComputedFields processes all computed fields for the given items.
func (h *DynamicModuleHandler) processComputedFields(items []map[string]interface{}, config *ModuleConfig) {
	if len(config.ComputedFields) == 0 {
		return
	}

	// Get lambda configuration with defaults
	lambdaConfig := config.LambdaConfig
	if lambdaConfig.TimeoutMs == 0 {
		lambdaConfig = lambda.DefaultLambdaConfig()
	}

	// Create safe database interface for lambda execution
	dbInterface := h.createDatabaseInterface()

	for i := range items {
		for _, field := range config.ComputedFields {
			// Skip if no lambda is defined
			if field.Lambda == "" {
				continue
			}

			var value interface{}
			var err error

			// Process lambda functions
			value, err = h.executeLambda(field.Lambda, items[i], dbInterface, lambdaConfig)
			if err != nil {
				// Log error and use fallback value
				fmt.Printf("Lambda execution error for field %s: %v\n", field.Name, err)
				value = fmt.Sprintf("Error: %v", err)
			}

			// Set the computed value
			if value != nil {
				items[i][field.Name] = value
			}
		}
	}
}

// executeLambda executes a JavaScript lambda function for a computed field.
func (h *DynamicModuleHandler) executeLambda(lambdaCode string, item map[string]interface{}, dbInterface *lambda.SafeDBInterface, config lambda.LambdaConfig) (string, error) {
	// Create execution context
	execCtx := lambda.ExecutionContext{
		Item: item,
		DB:   dbInterface,
	}

	// Execute the lambda
	result, err := h.lambdaEngine.ExecuteLambda(lambdaCode, execCtx, config)
	if err != nil {
		return "", fmt.Errorf("lambda execution failed: %w", err)
	}

	return result, nil
}

// createDatabaseInterface creates a safe database interface for lambda execution.
func (h *DynamicModuleHandler) createDatabaseInterface() *lambda.SafeDBInterface {
	return lambda.NewSafeDBInterface(&simpleDatabaseWrapper{db: h.db})
}

// simpleDatabaseWrapper implements the database.IDatabase interface for lambda use.
type simpleDatabaseWrapper struct {
	db *sql.DB
}

func (w *simpleDatabaseWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	remapped := database.RemapArgsForMySQL(query, args)
	converted := database.ConvertPlaceholders(query)
	return w.db.QueryRow(converted, remapped...)
}

func (w *simpleDatabaseWrapper) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	remapped := database.RemapArgsForMySQL(query, args)
	converted := database.ConvertPlaceholders(query)
	return w.db.Query(converted, remapped...)
}

func (w *simpleDatabaseWrapper) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	remapped := database.RemapArgsForMySQL(query, args)
	converted := database.ConvertPlaceholders(query)
	return w.db.Exec(converted, remapped...)
}

// Implement the required interface methods (minimal implementation for lambda use).
func (w *simpleDatabaseWrapper) Connect() error                     { return nil }
func (w *simpleDatabaseWrapper) Close() error                       { return nil }
func (w *simpleDatabaseWrapper) Ping() error                        { return w.db.Ping() }
func (w *simpleDatabaseWrapper) GetType() database.DatabaseType     { return database.PostgreSQL }
func (w *simpleDatabaseWrapper) GetConfig() database.DatabaseConfig { return database.DatabaseConfig{} }
func (w *simpleDatabaseWrapper) Begin(ctx context.Context) (database.ITransaction, error) {
	return nil, fmt.Errorf("transactions not supported in lambda")
}
func (w *simpleDatabaseWrapper) BeginTx(ctx context.Context, opts *sql.TxOptions) (database.ITransaction, error) {
	return nil, fmt.Errorf("transactions not supported in lambda")
}
func (w *simpleDatabaseWrapper) TableExists(ctx context.Context, tableName string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}
func (w *simpleDatabaseWrapper) GetTableColumns(ctx context.Context, tableName string) ([]database.ColumnInfo, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *simpleDatabaseWrapper) CreateTable(ctx context.Context, definition *database.TableDefinition) error {
	return fmt.Errorf("not supported")
}
func (w *simpleDatabaseWrapper) DropTable(ctx context.Context, tableName string) error {
	return fmt.Errorf("not supported")
}
func (w *simpleDatabaseWrapper) CreateIndex(ctx context.Context, tableName, indexName string, columns []string, unique bool) error {
	return fmt.Errorf("not supported")
}
func (w *simpleDatabaseWrapper) DropIndex(ctx context.Context, tableName, indexName string) error {
	return fmt.Errorf("not supported")
}
func (w *simpleDatabaseWrapper) Quote(identifier string) string { return `"` + identifier + `"` }
func (w *simpleDatabaseWrapper) QuoteValue(value interface{}) string {
	return fmt.Sprintf("'%v'", value)
}
func (w *simpleDatabaseWrapper) BuildInsert(tableName string, data map[string]interface{}) (string, []interface{}) {
	return "", nil
}
func (w *simpleDatabaseWrapper) BuildUpdate(tableName string, data map[string]interface{}, where string, whereArgs []interface{}) (string, []interface{}) {
	return "", nil
}
func (w *simpleDatabaseWrapper) BuildSelect(tableName string, columns []string, where string, orderBy string, limit int) string {
	return ""
}
func (w *simpleDatabaseWrapper) GetLimitClause(limit, offset int) string {
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}
func (w *simpleDatabaseWrapper) GetDateFunction() string { return "NOW()" }
func (w *simpleDatabaseWrapper) GetConcatFunction(fields []string) string {
	return strings.Join(fields, " || ")
}
func (w *simpleDatabaseWrapper) SupportsReturning() bool { return true }
func (w *simpleDatabaseWrapper) Stats() sql.DBStats      { return w.db.Stats() }
func (w *simpleDatabaseWrapper) IsHealthy() bool         { return w.db.Ping() == nil }

// handleAction handles special actions on records.
func (h *DynamicModuleHandler) handleAction(c *gin.Context, config *ModuleConfig, id, action string) {
	switch action {
	case "details":
		h.handleDetails(c, config, id)
	case "reset":
		h.handleReset(c, config, id)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown action: " + action})
	}
}

// handleDetails handles detailed information requests.
func (h *DynamicModuleHandler) handleDetails(c *gin.Context, config *ModuleConfig, id string) {
	// For sysconfig module, 'id' is actually the config name
	if config.Module.Name == "sysconfig" {
		h.handleSysconfigDetails(c, config, id)
		return
	}

	// For other modules, use regular record lookup
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1", config.Module.Table)
	row := h.queryRow(query, id)

	item := h.scanRow(row, config)
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    item,
	})
}

// handleSysconfigDetails handles sysconfig-specific details.
func (h *DynamicModuleHandler) handleSysconfigDetails(c *gin.Context, config *ModuleConfig, configName string) {
	query := `
		SELECT name, description, navigation, effective_value, xml_content_parsed,
		       user_modification_possible, is_readonly, is_required
		FROM sysconfig_default 
		WHERE name = $1 AND is_valid = 1
	`

	var details struct {
		Name                     string `json:"name"`
		Description              string `json:"description"`
		Navigation               string `json:"navigation"`
		EffectiveValue           string `json:"effective_value"`
		XMLContentParsed         string `json:"xml_content_parsed"`
		UserModificationPossible bool   `json:"user_modification_possible"`
		IsReadonly               bool   `json:"is_readonly"`
		IsRequired               bool   `json:"is_required"`
	}

	err := h.queryRow(query, configName).Scan(
		&details.Name, &details.Description, &details.Navigation,
		&details.EffectiveValue, &details.XMLContentParsed,
		&details.UserModificationPossible, &details.IsReadonly, &details.IsRequired,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Configuration not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// Parse XML content to get additional metadata
	var configData map[string]interface{}
	if details.XMLContentParsed != "" {
		json.Unmarshal([]byte(details.XMLContentParsed), &configData)
	}

	// Prepare response data
	response := map[string]interface{}{
		"name":                       details.Name,
		"description":                details.Description,
		"navigation":                 details.Navigation,
		"effective_value":            details.EffectiveValue,
		"user_modification_possible": details.UserModificationPossible,
		"is_readonly":                details.IsReadonly,
		"is_required":                details.IsRequired,
	}

	// Add parsed config data
	if configData != nil {
		if t, ok := configData["type"].(string); ok {
			response["type"] = t
		}
		if def, ok := configData["default"]; ok {
			response["default_value"] = def
		}
		if val, ok := configData["validation"].(string); ok {
			response["validation"] = val
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// handleReset handles reset-to-default requests.
func (h *DynamicModuleHandler) handleReset(c *gin.Context, config *ModuleConfig, id string) {
	// For sysconfig module, 'id' is actually the config name
	if config.Module.Name == "sysconfig" {
		h.handleSysconfigReset(c, config, id)
		return
	}

	// For other modules, this action doesn't make sense
	c.JSON(http.StatusBadRequest, gin.H{"error": "Reset action not supported for this module"})
}

// handleSysconfigReset handles sysconfig reset to default.
func (h *DynamicModuleHandler) handleSysconfigReset(c *gin.Context, config *ModuleConfig, configName string) {
	// Remove any custom value from sysconfig_modified table
	query := `DELETE FROM sysconfig_modified WHERE name = $1`

	_, err := h.exec(query, configName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration reset to default successfully",
	})
}

// generateCSVResponse generates a CSV response from items.
func (h *DynamicModuleHandler) generateCSVResponse(c *gin.Context, config *ModuleConfig, items []map[string]interface{}) {
	var csvData bytes.Buffer
	csvWriter := csv.NewWriter(&csvData)

	// Write header row
	var headers []string
	for _, field := range config.Fields {
		if field.ShowInList {
			headers = append(headers, field.Label)
		}
	}
	// Add computed field headers
	for _, field := range config.ComputedFields {
		if field.ShowInList {
			headers = append(headers, field.Label)
		}
	}
	csvWriter.Write(headers)

	// Write data rows
	for _, item := range items {
		var row []string
		// Regular fields
		for _, field := range config.Fields {
			if field.ShowInList {
				value := ""
				// Check for display value first (from lookups)
				displayKey := field.Name + "_display"
				if displayVal, ok := item[displayKey]; ok && displayVal != nil {
					value = fmt.Sprintf("%v", displayVal)
				} else if val, ok := item[field.Name]; ok && val != nil {
					// Handle special formatting for specific types
					switch field.Type {
					case "datetime":
						if t, ok := val.(time.Time); ok {
							value = t.Format("2006-01-02 15:04:05")
						} else {
							value = fmt.Sprintf("%v", val)
						}
					case "bool":
						if b, ok := val.(bool); ok {
							if b {
								value = "Yes"
							} else {
								value = "No"
							}
						} else {
							value = fmt.Sprintf("%v", val)
						}
					default:
						// Check for display map
						if field.DisplayMap != nil {
							if intVal, ok := val.(int64); ok {
								if display, exists := field.DisplayMap[int(intVal)]; exists {
									value = display
								} else {
									value = fmt.Sprintf("%v", val)
								}
							} else {
								value = fmt.Sprintf("%v", val)
							}
						} else {
							value = fmt.Sprintf("%v", val)
						}
					}
				}
				row = append(row, value)
			}
		}
		// Computed fields
		for _, field := range config.ComputedFields {
			if field.ShowInList {
				value := ""
				if val, ok := item[field.Name]; ok && val != nil {
					// Strip HTML tags for CSV export
					strVal := fmt.Sprintf("%v", val)
					// Simple HTML stripping - could be improved with regex
					strVal = strings.ReplaceAll(strVal, "<span class=\"text-gray-400\">", "")
					strVal = strings.ReplaceAll(strVal, "</span>", "")
					strVal = strings.ReplaceAll(strVal, "<div class=\"flex flex-wrap gap-1\">", "")
					strVal = strings.ReplaceAll(strVal, "</div>", "")
					// Replace badge HTML with comma-separated list
					if strings.Contains(strVal, "px-2 py-1") {
						// Extract text from badges
						parts := strings.Split(strVal, ">")
						var badges []string
						for i, part := range parts {
							if i > 0 && strings.Contains(parts[i-1], "px-2 py-1") {
								if idx := strings.Index(part, "<"); idx > 0 {
									badges = append(badges, part[:idx])
								}
							}
						}
						if len(badges) > 0 {
							strVal = strings.Join(badges, ", ")
						}
					}
					value = strVal
				}
				row = append(row, value)
			}
		}
		csvWriter.Write(row)
	}

	csvWriter.Flush()

	// Set headers for download
	filename := fmt.Sprintf("%s_export_%s.csv", config.Module.Name, time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "text/csv", csvData.Bytes())
}

// query executes SQL with driver-compatible placeholders.
func (h *DynamicModuleHandler) query(query string, args ...interface{}) (*sql.Rows, error) {
	remappedArgs := database.RemapArgsForMySQL(query, args)
	converted := database.ConvertPlaceholders(query)
	rows, err := h.db.Query(converted, remappedArgs...)
	if err != nil {
		fmt.Printf("Dynamic module query failed: %s (args=%v): %v\n", converted, remappedArgs, err)
	}
	return rows, err
}

// queryRow mirrors query but returns a single row handle.
func (h *DynamicModuleHandler) queryRow(query string, args ...interface{}) *sql.Row {
	remappedArgs := database.RemapArgsForMySQL(query, args)
	converted := database.ConvertPlaceholders(query)
	return h.db.QueryRow(converted, remappedArgs...)
}

// exec runs statements with automatic placeholder conversion.
func (h *DynamicModuleHandler) exec(query string, args ...interface{}) (sql.Result, error) {
	remappedArgs := database.RemapArgsForMySQL(query, args)
	converted := database.ConvertPlaceholders(query)
	res, err := h.db.Exec(converted, remappedArgs...)
	if err != nil {
		fmt.Printf("Dynamic module exec failed: %s (args=%v): %v\n", converted, remappedArgs, err)
	}
	return res, err
}

// buildFilterWhereClause builds WHERE clause based on filter parameters.
func (h *DynamicModuleHandler) buildFilterWhereClause(c *gin.Context, config *ModuleConfig) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// Process search parameter - search across searchable fields and their lookup display values
	searchValue := c.Query("search")
	if searchValue != "" {
		searchConditions := []string{}
		// Find searchable fields and add ILIKE conditions for each
		for _, field := range config.Fields {
			if field.Searchable {
				// Add condition for the direct field value
				searchConditions = append(searchConditions, config.Module.Table+"."+field.DBColumn+" ILIKE $"+fmt.Sprintf("%d", len(args)+1))
				args = append(args, "%"+searchValue+"%")

				// If this field has a lookup configuration, also search in the display value
				if field.LookupTable != "" && field.LookupDisplay != "" {
					// Add a subquery condition to search in the lookup table's display column
					subquery := fmt.Sprintf("%s.%s IN (SELECT %s FROM %s WHERE %s ILIKE $%d)",
						config.Module.Table, field.DBColumn, field.LookupKey,
						field.LookupTable, field.LookupDisplay, len(args)+1)
					searchConditions = append(searchConditions, subquery)
					args = append(args, "%"+searchValue+"%")
				}
			}
		}

		// If we have searchable fields, combine them with OR
		if len(searchConditions) > 0 {
			conditions = append(conditions, "("+strings.Join(searchConditions, " OR ")+")")
		}
	}

	// Process each configured filter
	for _, filter := range config.Filters {
		// Get the filter value from query parameters
		filterValue := c.Query("filter_" + filter.Field)
		if filterValue == "" {
			continue // Skip empty filters
		}

		// Add condition based on filter field
		switch filter.Type {
		case "select":
			// For select filters, do exact match
			conditions = append(conditions, filter.Field+" = $"+fmt.Sprintf("%d", len(args)+1))
			args = append(args, filterValue)
		case "text":
			// For text filters, do LIKE search
			conditions = append(conditions, filter.Field+" ILIKE $"+fmt.Sprintf("%d", len(args)+1))
			args = append(args, "%"+filterValue+"%")
		case "date_range":
			// Handle date range filters - expect from and to parameters
			fromDate := c.Query("filter_" + filter.Field + "_from")
			toDate := c.Query("filter_" + filter.Field + "_to")

			if fromDate != "" {
				conditions = append(conditions, filter.Field+" >= $"+fmt.Sprintf("%d", len(args)+1))
				args = append(args, fromDate)
			}
			if toDate != "" {
				conditions = append(conditions, filter.Field+" <= $"+fmt.Sprintf("%d", len(args)+1))
				args = append(args, toDate+" 23:59:59")
			}
		case "multi_select":
			// Handle multi-select filters - expect comma-separated values
			values := strings.Split(filterValue, ",")

			// Special handling for users module with group filter
			if config.Module.Name == "users" && filter.Field == "group_id" {
				// For users, we need to join with user_groups table
				placeholders := make([]string, len(values))
				for i, val := range values {
					placeholders[i] = "$" + fmt.Sprintf("%d", len(args)+1)
					args = append(args, strings.TrimSpace(val))
				}
				// Add subquery condition for users in selected groups
				conditions = append(conditions, "id IN (SELECT user_id FROM user_groups WHERE group_id IN ("+strings.Join(placeholders, ", ")+"))")
			} else {
				// Regular multi-select for other fields
				placeholders := make([]string, len(values))
				for i, val := range values {
					placeholders[i] = "$" + fmt.Sprintf("%d", len(args)+1)
					args = append(args, strings.TrimSpace(val))
				}
				conditions = append(conditions, filter.Field+" IN ("+strings.Join(placeholders, ", ")+")")
			}
		default:
			// Default to exact match
			conditions = append(conditions, filter.Field+" = $"+fmt.Sprintf("%d", len(args)+1))
			args = append(args, filterValue)
		}
	}

	// Join conditions with AND
	if len(conditions) > 0 {
		return strings.Join(conditions, " AND "), args
	}

	return "", nil
}

// Close closes the lambda engine.
func (h *DynamicModuleHandler) Close() {
	if h.lambdaEngine != nil {
		h.lambdaEngine.Close()
	}
}
