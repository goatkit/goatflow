// Package main generates route documentation (Markdown, OpenAPI route map,
// HTML) from the YAML route definitions in routes/.
package main

import (
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	texttemplate "text/template"

	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// ginParamPattern matches gin path parameters (:id) and wildcards (*path).
var ginParamPattern = regexp.MustCompile(`[:*]([A-Za-z0-9_]+)`)

// authMiddleware are the middleware names that require a logged-in caller.
var authMiddleware = map[string]bool{"auth": true, "unified_auth": true, "api_token": true}

type docGroup struct {
	File        string
	Name        string
	Namespace   string
	Description string
	Prefix      string
	Middleware  []string
	Routes      []docRoute
}

type docRoute struct {
	Method      string
	Path        string // absolute path as the server registers it (gin syntax)
	Handler     string
	Middleware  []string
	Description string
	Condition   string
}

// DocumentationGenerator generates API documentation from YAML routes.
type DocumentationGenerator struct {
	routesDir string
	outputDir string
	groups    []docGroup
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: route-docs <routes-dir> <output-dir>")
		fmt.Println("Example: route-docs ./routes ./docs/api")
		os.Exit(1)
	}

	generator := &DocumentationGenerator{routesDir: os.Args[1], outputDir: os.Args[2]}
	if err := generator.Generate(); err != nil {
		log.Fatalf("Failed to generate documentation: %v", err)
	}

	fmt.Printf("✅ API documentation generated in %s\n", generator.outputDir)
}

// Generate loads the routes and writes api.md, openapi.json and index.html.
func (g *DocumentationGenerator) Generate() error {
	if err := g.loadRoutes(); err != nil {
		return fmt.Errorf("loading routes: %w", err)
	}
	if err := os.MkdirAll(g.outputDir, 0750); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}
	if err := g.writeFile("api.md", g.generateMarkdown); err != nil {
		return fmt.Errorf("generating markdown: %w", err)
	}
	if err := g.writeFile("openapi.json", g.generateOpenAPI); err != nil {
		return fmt.Errorf("generating OpenAPI: %w", err)
	}
	if err := g.writeFile("index.html", g.generateHTML); err != nil {
		return fmt.Errorf("generating HTML: %w", err)
	}
	return nil
}

func (g *DocumentationGenerator) writeFile(name string, render func(io.Writer) error) error {
	file, err := os.Create(filepath.Join(g.outputDir, name)) // #nosec G304 -- output dir is the operator's CLI flag, name is a fixed doc filename
	if err != nil {
		return err
	}
	if err := render(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// loadRoutes reads the enabled route groups the same way the server's YAML
// loader does: disabled groups are skipped and every path is resolved with
// routing.FullRoutePath.
func (g *DocumentationGenerator) loadRoutes() error {
	return filepath.Walk(g.routesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		data, err := os.ReadFile(path) // #nosec G304 G122 -- dev CLI reading the operator-named routes tree; no privilege boundary for a symlink swap
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		configs, err := routing.ParseYAMLDocuments[routing.RouteConfig](data)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		for i := range configs {
			cfg := &configs[i]
			if cfg.Kind != "RouteGroup" || cfg.Metadata.Name == "" || !cfg.Metadata.Enabled {
				continue
			}
			g.groups = append(g.groups, buildGroup(filepath.Base(path), cfg))
		}
		return nil
	})
}

func buildGroup(file string, cfg *routing.RouteConfig) docGroup {
	group := docGroup{
		File:        file,
		Name:        cfg.Metadata.Name,
		Namespace:   cfg.Metadata.Namespace,
		Description: cfg.Metadata.Description,
		Prefix:      cfg.Spec.Prefix,
		Middleware:  cfg.Spec.Middleware,
	}
	for i := range cfg.Spec.Routes {
		rt := &cfg.Spec.Routes[i]
		route := docRoute{
			Path:        routing.FullRoutePath(cfg.Spec.Prefix, rt.Path),
			Middleware:  rt.Middleware,
			Description: rt.Description,
			Condition:   rt.Condition,
		}
		if rt.Handler != "" {
			for _, method := range rt.GetMethods() {
				route.Method = strings.ToUpper(method)
				route.Handler = rt.Handler
				group.Routes = append(group.Routes, route)
			}
			continue
		}
		methods := make([]string, 0, len(rt.Handlers))
		for method := range rt.Handlers {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		for _, method := range methods {
			route.Method = strings.ToUpper(method)
			route.Handler = rt.Handlers[method]
			group.Routes = append(group.Routes, route)
		}
	}
	return group
}

const markdownTemplate = `# GoatFlow route reference

Generated by ` + "`make generate-route-docs`" + ` from ` + "`routes/*.yaml`" + `. Paths are the
absolute paths the server registers. The curated REST API contract is
` + "`api/openapi.yaml`" + `.
{{range .}}
## {{.Name}}

{{.Description}}

- **File:** ` + "`routes/{{.File}}`" + `
- **Prefix:** ` + "`{{.Prefix}}`" + `
- **Group middleware:** {{if .Middleware}}{{range $i, $m := .Middleware}}{{if $i}}, {{end}}` + "`{{$m}}`" + `{{end}}{{else}}none{{end}}

| Method | Path | Handler | Route middleware | Description |
|--------|------|---------|------------------|-------------|
{{range .Routes}}| {{.Method}} | ` + "`{{.Path}}`" + ` | ` + "`{{.Handler}}`" + ` | {{range $i, $m := .Middleware}}{{if $i}}, {{end}}` + "`{{$m}}`" + `{{end}} | {{cell .Description}}{{if .Condition}} (only when ` + "`{{.Condition}}`" + `){{end}} |
{{end}}{{end}}`

func (g *DocumentationGenerator) generateMarkdown(w io.Writer) error {
	t, err := texttemplate.New("markdown").Funcs(texttemplate.FuncMap{
		// cell keeps a description inside its Markdown table cell.
		"cell": func(s string) string {
			return strings.ReplaceAll(strings.Join(strings.Fields(s), " "), "|", `\|`)
		},
	}).Parse(markdownTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, g.groups)
}

func (g *DocumentationGenerator) generateOpenAPI(w io.Writer) error {
	paths := map[string]map[string]interface{}{}
	for _, group := range g.groups {
		for _, rt := range group.Routes {
			oaPath := ginParamPattern.ReplaceAllString(rt.Path, "{$1}")
			if paths[oaPath] == nil {
				paths[oaPath] = map[string]interface{}{}
			}

			middleware := append(append([]string{}, group.Middleware...), rt.Middleware...)
			security := []map[string][]string{}
			for _, mw := range middleware {
				if authMiddleware[mw] {
					security = []map[string][]string{{"bearerAuth": {}}, {"cookieAuth": {}}}
					break
				}
			}

			params := []map[string]interface{}{}
			for _, m := range ginParamPattern.FindAllStringSubmatch(rt.Path, -1) {
				params = append(params, map[string]interface{}{
					"name": m[1], "in": "path", "required": true,
					"schema": map[string]string{"type": "string"},
				})
			}

			summary := rt.Description
			if summary == "" {
				summary = rt.Method + " " + rt.Path
			}
			op := map[string]interface{}{
				"summary":      summary,
				"tags":         []string{group.Name},
				"security":     security,
				"x-handler":    rt.Handler,
				"x-middleware": middleware,
				"responses": map[string]interface{}{
					"default": map[string]string{"description": "Defined by the handler"},
				},
			}
			if len(params) > 0 {
				op["parameters"] = params
			}
			paths[oaPath][strings.ToLower(rt.Method)] = op
		}
	}

	doc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "GoatFlow route map",
			"description": "Every route in routes/*.yaml, generated by make generate-route-docs. Request and response contracts for the REST API are in api/openapi.yaml.",
			"version":     version.Short(),
		},
		"servers": []map[string]string{{"url": "/"}},
		"paths":   paths,
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]string{"type": "http", "scheme": "bearer"},
				"cookieAuth": map[string]string{"type": "apiKey", "in": "cookie", "name": "auth_token"},
			},
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GoatFlow route reference</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 20px; background: #f5f7fa; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .route-group { margin-bottom: 40px; border: 1px solid #e2e8f0; border-radius: 8px; overflow: hidden; }
        .group-header { background: #4299e1; color: white; padding: 15px 20px; }
        .group-content { padding: 20px; }
        .route { margin-bottom: 12px; padding: 10px 15px; border-left: 4px solid #4299e1; background: #f7fafc; }
        .method { display: inline-block; min-width: 60px; padding: 4px 8px; border-radius: 4px; font-weight: bold; color: white; margin-right: 10px; background: #718096; }
        .GET { background: #48bb78; } .POST { background: #ed8936; } .PUT, .PATCH { background: #4299e1; } .DELETE { background: #f56565; }
        .path { font-family: Monaco, monospace; background: #2d3748; color: #e2e8f0; padding: 4px 8px; border-radius: 4px; }
        .mw { display: inline-block; background: #9f7aea; color: white; padding: 2px 8px; border-radius: 12px; font-size: 12px; margin: 2px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>GoatFlow route reference</h1>
        <p>Generated from routes/*.yaml. Paths are the absolute paths the server registers.</p>
        {{range .}}
        <div class="route-group">
            <div class="group-header">
                <h2>{{.Name}}</h2>
                <p>{{.Description}} (routes/{{.File}}, prefix <code>{{.Prefix}}</code>)</p>
            </div>
            <div class="group-content">
                <p>{{range .Middleware}}<span class="mw">{{.}}</span>{{end}}</p>
                {{range .Routes}}
                <div class="route">
                    <span class="method {{.Method}}">{{.Method}}</span>
                    <code class="path">{{.Path}}</code>
                    {{range .Middleware}}<span class="mw">{{.}}</span>{{end}}
                    <p>{{.Description}} <small>({{.Handler}})</small></p>
                </div>
                {{end}}
            </div>
        </div>
        {{end}}
    </div>
</body>
</html>
`

func (g *DocumentationGenerator) generateHTML(w io.Writer) error {
	t, err := htmltemplate.New("html").Parse(htmlTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, g.groups)
}
