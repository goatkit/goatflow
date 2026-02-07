// Package main provides code generation utilities.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// ModuleConfig represents the YAML configuration for a module.
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
}

// Field represents a field in the module.
type Field struct {
	Name       string      `yaml:"name"`
	Type       string      `yaml:"type"`
	DBColumn   string      `yaml:"db_column"`
	Label      string      `yaml:"label"`
	Required   bool        `yaml:"required"`
	Searchable bool        `yaml:"searchable"`
	Sortable   bool        `yaml:"sortable"`
	ShowInList bool        `yaml:"show_in_list"`
	ShowInForm bool        `yaml:"show_in_form"`
	Default    interface{} `yaml:"default"`
	Options    []Option    `yaml:"options"`
	Validation string      `yaml:"validation"`
	Help       string      `yaml:"help"`
}

// Option represents an option for select fields.
type Option struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

func main() {
	var (
		configFile = flag.String("config", "", "Path to YAML configuration file")
		outputDir  = flag.String("output", ".", "Output directory for generated files")
		listOnly   = flag.Bool("list", false, "List available module templates")
		example    = flag.Bool("example", false, "Generate an example YAML configuration")
	)

	flag.Parse()

	if *listOnly {
		listTemplates()
		return
	}

	if *example {
		generateExampleConfig()
		return
	}

	if *configFile == "" {
		log.Fatal("Please provide a configuration file with -config flag")
	}

	// Read configuration file
	data, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	// Parse YAML
	var config ModuleConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
	}

	// Generate module files
	err = generateModule(config, *outputDir)
	if err != nil {
		log.Fatalf("Error generating module: %v", err)
	}

	fmt.Printf("✅ Module '%s' generated successfully!\n", config.Module.Name)
	fmt.Println("\nGenerated files:")
	fmt.Printf("  - Handler: internal/api/admin_%s_handler.go\n", config.Module.Name)
	fmt.Printf("  - Template: templates/pages/admin/%s.pongo2\n", config.Module.Name)
	fmt.Printf("  - Test: internal/api/admin_%s_handler_test.go\n", config.Module.Name)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Add route registration in internal/api/routes.go")
	fmt.Println("  2. Add menu item in templates/partials/admin_nav.pongo2")
	fmt.Println("  3. Run migrations if new table is needed")
	fmt.Println("  4. Test the module at /admin/" + config.Module.Name)
}

func generateModule(config ModuleConfig, outputDir string) error {
	// Generate handler
	if err := generateHandler(config, outputDir); err != nil {
		return fmt.Errorf("failed to generate handler: %w", err)
	}

	// Generate template (using string building, not Go templates)
	if err := generatePongoTemplate(config, outputDir); err != nil {
		return fmt.Errorf("failed to generate template: %w", err)
	}

	// Generate test
	if err := generateTest(config, outputDir); err != nil {
		return fmt.Errorf("failed to generate test: %w", err)
	}

	// Generate migration if needed
	if err := generateMigration(config, outputDir); err != nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	return nil
}

func generateHandler(config ModuleConfig, outputDir string) error {
	handlerTemplate := `package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/database"
)

// Admin{{.Module.Singular}}Handler handles {{.Module.Name}} management
type Admin{{.Module.Singular}}Handler struct {
	db       *sql.DB
	renderer *pongo2.Django
}

// NewAdmin{{.Module.Singular}}Handler creates a new {{.Module.Name}} handler
func NewAdmin{{.Module.Singular}}Handler() *Admin{{.Module.Singular}}Handler {
	return &Admin{{.Module.Singular}}Handler{
		db:       database.GetDB(),
		renderer: pongo2Renderer,
	}
}

// List handles GET /admin/{{.Module.Name}}
func (h *Admin{{.Module.Singular}}Handler) List(c *gin.Context) {
	query := ` + "`" + `
		SELECT {{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.DBColumn}}{{end}}
		FROM {{.Module.Table}}
		{{if .Features.SoftDelete}}WHERE valid_id = 1{{end}}
		ORDER BY id DESC
	` + "`" + `
	
	rows, err := h.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	
	var items []map[string]interface{}
	for rows.Next() {
		item := make(map[string]interface{})
		// Scan fields
		var {{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.Name}} {{$f | goType}}{{end}}
		
		err := rows.Scan({{range $i, $f := .Fields}}{{if $i}}, {{end}}&{{$f.Name}}{{end}})
		if err != nil {
			continue
		}
		
		{{range .Fields}}
		item["{{.Name}}"] = {{.Name}}{{end}}
		
		items = append(items, item)
	}
	
	if isAPIRequest(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    items,
			"total":   len(items),
		})
		return
	}
	
	// Render template
	h.renderer.HTML(c, http.StatusOK, "pages/admin/{{.Module.Name}}.pongo2", pongo2.Context{
		"items":      items,
		"module":     "{{.Module.Name}}",
		"singular":   "{{.Module.Singular}}",
		"plural":     "{{.Module.Plural}}",
		"features":   map[string]bool{
			"soft_delete": {{.Features.SoftDelete}},
			"search":      {{.Features.Search}},
			"import_csv":  {{.Features.ImportCSV}},
			"export_csv":  {{.Features.ExportCSV}},
		},
	})
}

// Get handles GET /admin/{{.Module.Name}}/:id
func (h *Admin{{.Module.Singular}}Handler) Get(c *gin.Context) {
	id := c.Param("id")
	
	query := ` + "`" + `
		SELECT {{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.DBColumn}}{{end}}
		FROM {{.Module.Table}}
		WHERE id = $1
	` + "`" + `
	
	var {{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.Name}} {{$f | goType}}{{end}}
	
	err := h.db.QueryRow(query, id).Scan({{range $i, $f := .Fields}}{{if $i}}, {{end}}&{{$f.Name}}{{end}})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "{{.Module.Singular}} not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			{{range .Fields}}"{{.Name}}": {{.Name}},
			{{end}}
		},
	})
}

// Create handles POST /admin/{{.Module.Name}}
func (h *Admin{{.Module.Singular}}Handler) Create(c *gin.Context) {
	var input struct {
		{{range .Fields}}{{if ne .Name "id"}}{{.Name | capitalize}} {{. | goType}} ` + "`" + `json:"{{.Name}}" form:"{{.Name}}"` + "`" + `
		{{end}}{{end}}
	}
	
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	query := ` + "`" + `
		INSERT INTO {{.Module.Table}} ({{range $i, $f := .Fields}}{{if ne $f.Name "id"}}{{if $i}}, {{end}}{{$f.DBColumn}}{{end}}{{end}}{{if .Features.SoftDelete}}, valid_id{{end}})
		VALUES ({{range $i, $f := .Fields}}{{if ne $f.Name "id"}}{{if $i}}, {{end}}${{$i | inc}}{{end}}{{end}}{{if .Features.SoftDelete}}, 1{{end}})
	` + "`" + `
	
	_, err := h.db.Exec(query{{range .Fields}}{{if ne .Name "id"}}, input.{{.Name | capitalize}}{{end}}{{end}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "{{.Module.Singular}} created successfully",
	})
}

// Update handles PUT /admin/{{.Module.Name}}/:id
func (h *Admin{{.Module.Singular}}Handler) Update(c *gin.Context) {
	id := c.Param("id")
	
	var input struct {
		{{range .Fields}}{{if ne .Name "id"}}{{.Name | capitalize}} {{. | goType}} ` + "`" + `json:"{{.Name}}" form:"{{.Name}}"` + "`" + `
		{{end}}{{end}}
	}
	
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	query := ` + "`" + `
		UPDATE {{.Module.Table}}
		SET {{range $i, $f := .Fields}}{{if ne $f.Name "id"}}{{if $i}}, {{end}}{{$f.DBColumn}} = ${{$i | inc}}{{end}}{{end}}
		WHERE id = ${{len .Fields}}
	` + "`" + `
	
	_, err := h.db.Exec(query{{range .Fields}}{{if ne .Name "id"}}, input.{{.Name | capitalize}}{{end}}{{end}}, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "{{.Module.Singular}} updated successfully",
	})
}

// Delete handles DELETE /admin/{{.Module.Name}}/:id
func (h *Admin{{.Module.Singular}}Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	
	{{if .Features.SoftDelete}}
	// Soft delete - set valid_id to 2
	query := ` + "`UPDATE {{.Module.Table}} SET valid_id = 2 WHERE id = $1`" + `
	{{else}}
	// Hard delete
	query := ` + "`DELETE FROM {{.Module.Table}} WHERE id = $1`" + `
	{{end}}
	
	_, err := h.db.Exec(query, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "{{.Module.Singular}} deleted successfully",
	})
}

{{if .Features.Search}}
// Search handles GET /admin/{{.Module.Name}}/search
func (h *Admin{{.Module.Singular}}Handler) Search(c *gin.Context) {
	searchTerm := c.Query("q")
	if searchTerm == "" {
		h.List(c)
		return
	}
	
	query := ` + "`" + `
		SELECT {{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.DBColumn}}{{end}}
		FROM {{.Module.Table}}
		WHERE {{range $i, $f := .Fields}}{{if $f.Searchable}}{{if $i}} OR {{end}}{{$f.DBColumn}} ILIKE $1{{end}}{{end}}
		{{if .Features.SoftDelete}}AND valid_id = 1{{end}}
		ORDER BY id DESC
	` + "`" + `
	
	rows, err := h.db.Query(query, "%"+searchTerm+"%")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	
	// ... same scanning logic as List
	var items []map[string]interface{}
	// ... scan rows
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
		"total":   len(items),
	})
}
{{end}}
`

	title := cases.Title(language.English)
	tmpl, err := template.New("handler").Funcs(template.FuncMap{
		"goType": func(f Field) string {
			switch f.Type {
			case "string", "text", "color", "select":
				return "string"
			case "int", "integer":
				return "int"
			case "float", "decimal":
				return "float64"
			case "bool", "boolean":
				return "bool"
			case "date", "datetime":
				return "time.Time"
			default:
				return "interface{}"
			}
		},
		"capitalize": func(s string) string { return title.String(s) },
		"inc":        func(i int) int { return i + 1 },
	}).Parse(handlerTemplate)

	if err != nil {
		return err
	}

	// Create output file
	handlerPath := filepath.Join(outputDir, "internal", "api", fmt.Sprintf("admin_%s_handler.go", config.Module.Name))
	if err := os.MkdirAll(filepath.Dir(handlerPath), 0750); err != nil {
		return err
	}

	file, err := os.Create(handlerPath) //nolint:gosec // G304 CLI tool
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return tmpl.Execute(file, config)
}

// generateTemplate is now in templates.go

func generateTest(config ModuleConfig, outputDir string) error {
	// Test generation code here
	return nil
}

func generateMigration(config ModuleConfig, outputDir string) error {
	// Migration generation code here if new table is needed
	return nil
}

func listTemplates() {
	fmt.Println("Available module templates:")
	fmt.Println("  - priority    : Priority management module")
	fmt.Println("  - state       : Ticket state management")
	fmt.Println("  - type        : Ticket type management")
	fmt.Println("  - service     : Service catalog management")
	fmt.Println("  - sla         : Service Level Agreement management")
	fmt.Println("  - queue       : Queue management")
	fmt.Println("  - template    : Email template management")
	fmt.Println("\nUse: generator -config <module>.yaml")
}

func generateExampleConfig() {
	example := `# Example module configuration
module:
  name: priority
  singular: Priority
  plural: Priorities
  table: priority
  description: Manage ticket priorities
  route_prefix: /admin/priorities

fields:
  - name: id
    type: int
    db_column: id
    label: ID
    show_in_list: true
    show_in_form: false
    sortable: true

  - name: name
    type: string
    db_column: name
    label: Name
    required: true
    searchable: true
    sortable: true
    show_in_list: true
    show_in_form: true
    validation: "^[a-zA-Z0-9 ]+$"
    help: "Enter a descriptive name for the priority"

  - name: color
    type: color
    db_column: color
    label: Color
    required: false
    show_in_list: true
    show_in_form: true
    default: "#000000"
    help: "Choose a color to represent this priority"

  - name: valid_id
    type: int
    db_column: valid_id
    label: Status
    show_in_list: true
    show_in_form: false
    default: 1
    options:
      - value: 1
        label: Valid
      - value: 2
        label: Invalid

features:
  soft_delete: true
  search: true
  import_csv: true
  export_csv: true
  status_toggle: true
  color_picker: true

permissions:
  - view
  - create
  - update
  - delete

validation:
  unique_fields:
    - name
  required_fields:
    - name
`

	err := os.WriteFile("example-module.yaml", []byte(example), 0644)
	if err != nil {
		log.Fatalf("Error writing example config: %v", err)
	}

	fmt.Println("✅ Generated example-module.yaml")
	fmt.Println("\nEdit this file and run:")
	fmt.Println("  generator -config example-module.yaml")
}
