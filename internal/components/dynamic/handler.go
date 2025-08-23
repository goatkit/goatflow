package dynamic

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pongo2 "github.com/flosch/pongo2/v6"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// hashPassword hashes a password using SHA256 with salt
// Format: sha256$salt$hash (similar to OTRS format)
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

// generateSalt generates a random salt for password hashing
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

// ModuleConfig represents a module's configuration
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
		ListColumns []string `yaml:"list_columns"`
	} `yaml:"ui"`
}

// Field represents a field in the module
type Field struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	DBColumn    string      `yaml:"db_column"`
	Label       string      `yaml:"label"`
	Required    bool        `yaml:"required"`
	Searchable  bool        `yaml:"searchable"`
	Sortable    bool        `yaml:"sortable"`
	ShowInList  bool        `yaml:"show_in_list"`
	ShowInForm  bool        `yaml:"show_in_form"`
	Default     interface{} `yaml:"default"`
	Options     []Option    `yaml:"options"`
	Validation  string      `yaml:"validation"`
	Help        string      `yaml:"help"`
}

// ComputedField represents a computed field that's not directly from the database
type ComputedField struct {
	Name       string `yaml:"name"`
	Label      string `yaml:"label"`
	ShowInList bool   `yaml:"show_in_list"`
	ShowInForm bool   `yaml:"show_in_form"`
	Source     string `yaml:"source"`
}

// Option for select fields
type Option struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

// DynamicModuleHandler handles all dynamic modules
type DynamicModuleHandler struct {
	configs  map[string]*ModuleConfig
	mu       sync.RWMutex
	db       *sql.DB
	renderer *pongo2.TemplateSet
	watcher  *fsnotify.Watcher
	modulesPath string
}

// NewDynamicModuleHandler creates a new dynamic module handler
func NewDynamicModuleHandler(db *sql.DB, renderer *pongo2.TemplateSet, modulesPath string) (*DynamicModuleHandler, error) {
	h := &DynamicModuleHandler{
		configs:     make(map[string]*ModuleConfig),
		db:          db,
		renderer:    renderer,
		modulesPath: modulesPath,
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

// loadAllConfigs loads all YAML configs from modules directory
func (h *DynamicModuleHandler) loadAllConfigs() error {
	files, err := ioutil.ReadDir(h.modulesPath)
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

// loadConfig loads a single YAML config
func (h *DynamicModuleHandler) loadConfig(path string) error {
	data, err := ioutil.ReadFile(path)
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

// setupWatcher sets up file system watcher for hot reload
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

// watchFiles watches for file changes
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

// ServeModule handles requests for any dynamic module
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
	switch c.Request.Method {
	case "GET":
		if id := c.Param("id"); id != "" {
			h.handleGet(c, config, id)
		} else {
			h.handleList(c, config)
		}
	case "POST":
		h.handleCreate(c, config)
	case "PUT":
		if id := c.Param("id"); id != "" {
			h.handleUpdate(c, config, id)
		}
	case "DELETE":
		if id := c.Param("id"); id != "" {
			h.handleDelete(c, config, id)
		}
	}
}

// handleList handles listing all records
func (h *DynamicModuleHandler) handleList(c *gin.Context, config *ModuleConfig) {
	// Build SELECT query
	columns := h.getSelectColumns(config)
	query := fmt.Sprintf("SELECT %s FROM %s", columns, config.Module.Table)
	
	// Don't filter by valid_id - show all records so users can enable/disable them
	// if config.Features.SoftDelete {
	//     query += " WHERE valid_id = 1"
	// }
	
	query += " ORDER BY id DESC"
	
	rows, err := h.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	
	items := h.scanRows(rows, config)
	
	// For users module, add group information and computed fields
	if config.Module.Name == "users" {
		for i := range items {
			// Compute full_name from first_name and last_name
			firstName := ""
			lastName := ""
			if fn, ok := items[i]["first_name"].(string); ok {
				firstName = fn
			}
			if ln, ok := items[i]["last_name"].(string); ok {
				lastName = ln
			}
			fullName := strings.TrimSpace(firstName + " " + lastName)
			if fullName == "" {
				// Fall back to login if no name
				if login, ok := items[i]["login"].(string); ok {
					fullName = login
				}
			}
			items[i]["full_name"] = fullName
			
			// Get groups for this user
			if id, ok := items[i]["id"].(int64); ok {
				groupQuery := `
					SELECT DISTINCT g.name 
					FROM groups g 
					INNER JOIN user_groups ug ON g.id = ug.group_id 
					WHERE ug.user_id = $1
					ORDER BY g.name`
				
				groupRows, err := h.db.Query(groupQuery, id)
				if err == nil {
					var groups []string
					for groupRows.Next() {
						var groupName string
						if err := groupRows.Scan(&groupName); err == nil {
							groups = append(groups, groupName)
						}
					}
					groupRows.Close()
					items[i]["Groups"] = groups
					items[i]["group_names"] = strings.Join(groups, ", ")
				}
			}
		}
	}
	
	// Check if this is an API request
	if h.isAPIRequest(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    items,
			"total":   len(items),
		})
		return
	}
	
	// Render the universal template
	fmt.Printf("DEBUG: Loading template pages/admin/dynamic_module.pongo2 for module %s\n", config.Module.Name)
	tmpl, err := h.renderer.FromFile("pages/admin/dynamic_module.pongo2")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template not found: " + err.Error()})
		return
	}
	fmt.Printf("DEBUG: Template loaded successfully\n")
	
	// Convert config to JSON string for JavaScript
	configJSON, _ := json.Marshal(config)
	
	// Debug: Log what features are being passed
	fmt.Printf("DEBUG: Features for module %s: search=%v, soft_delete=%v\n", 
		config.Module.Name, config.Features.Search, config.Features.SoftDelete)
	
	// Template rendering with data
	// Get user from context for navigation - match the format used in getUserMapForTemplate
	userMap := make(map[string]interface{})
	if userID, exists := c.Get("user_id"); exists {
		userMap["ID"] = userID
		userMap["id"] = userID  // Also provide lowercase for compatibility
	}
	if userEmail, exists := c.Get("user_email"); exists {
		userMap["Email"] = userEmail
		userMap["Login"] = userEmail  // Use email as login
		userMap["email"] = userEmail  // Also provide lowercase
	}
	if userRole, exists := c.Get("user_role"); exists {
		userMap["Role"] = userRole
		userMap["IsAdmin"] = (userRole == "Admin")
		userMap["role"] = userRole  // Also provide lowercase
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
		userMap["name"] = userName  // Also provide lowercase
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
	
	html, err := tmpl.Execute(pongo2.Context{
		"config": config,
		"config_json": string(configJSON),
		"items":  items,
		"module": config.Module,
		"fields": config.Fields,
		"allFields": allFields,  // Combined fields for display
		"features": config.Features,
		"User": userMap,  // Required for base template
		"ActivePage": "admin",  // For navigation highlighting
		"Title": fmt.Sprintf("%s Management", config.Module.Plural),
	})
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// handleGet handles getting a single record
func (h *DynamicModuleHandler) handleGet(c *gin.Context, config *ModuleConfig, id string) {
	columns := h.getSelectColumns(config)
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", columns, config.Module.Table)
	
	row := h.db.QueryRow(query, id)
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
		
		rows, err := h.db.Query(groupQuery, id)
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

// handleCreate handles creating a new record
func (h *DynamicModuleHandler) handleCreate(c *gin.Context, config *ModuleConfig) {
	data := h.parseFormData(c, config)
	
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
	
	for _, field := range config.Fields {
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
		}
	}
	
	// Add valid_id if soft delete is enabled
	if config.Features.SoftDelete {
		columns = append(columns, "valid_id")
		placeholders = append(placeholders, "1")
	}
	
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		config.Module.Table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))
	
	var newID int64
	err := h.db.QueryRow(query, values...).Scan(&newID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Handle group assignments for users module
	if config.Module.Name == "users" {
		if groupsStr := c.PostForm("groups"); groupsStr != "" {
			// Parse the selected group IDs
			groupIDs := strings.Split(groupsStr, ",")
			for _, groupID := range groupIDs {
				if groupID != "" {
					_, err = h.db.Exec(`
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

// handleUpdate handles updating a record
func (h *DynamicModuleHandler) handleUpdate(c *gin.Context, config *ModuleConfig, id string) {
	data := h.parseFormData(c, config)
	
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
		}
	}
	
	values = append(values, id)
	
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
		config.Module.Table,
		strings.Join(sets, ", "),
		len(values))
	
	_, err := h.db.Exec(query, values...)
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
			_, err = h.db.Exec("DELETE FROM user_groups WHERE user_id = $1", id)
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
						_, err = h.db.Exec(`
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

// handleDelete handles deleting (or soft deleting) a record
func (h *DynamicModuleHandler) handleDelete(c *gin.Context, config *ModuleConfig, id string) {
	var query string
	
	if config.Features.SoftDelete {
		query = fmt.Sprintf("UPDATE %s SET valid_id = 2 WHERE id = $1", config.Module.Table)
	} else {
		query = fmt.Sprintf("DELETE FROM %s WHERE id = $1", config.Module.Table)
	}
	
	_, err := h.db.Exec(query, id)
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
				item[field.Name] = *ptr
			}
		}
		
		items = append(items, item)
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
			item[field.Name] = *ptr
		}
	}
	
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

func (h *DynamicModuleHandler) isAPIRequest(c *gin.Context) bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(c.GetHeader("Accept"), "application/json")
}

// GetAvailableModules returns list of loaded modules
func (h *DynamicModuleHandler) GetAvailableModules() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	modules := []string{}
	for name := range h.configs {
		modules = append(modules, name)
	}
	return modules
}

// handleSchemaDiscovery handles schema discovery requests
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
			"data": tables,
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
			"table": tableName,
			"data": columns,
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
			"config": config,
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
		
		if err := ioutil.WriteFile(filename, yamlData, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("Module config saved to %s", filename),
			"filename": filename,
		})
		
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid action. Use: tables, columns, generate, or save",
		})
	}
}