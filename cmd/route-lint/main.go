// Package main provides route linting utilities.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LintRule represents a validation rule.
type LintRule struct {
	ID          string
	Name        string
	Description string
	Severity    string // error, warning, info
	Check       func(*RouteConfig, string) []LintIssue
}

// LintIssue represents a linting issue found.
type LintIssue struct {
	Rule     string
	Severity string
	Message  string
	File     string
	Line     int
}

// RouteConfig matches our YAML route structure.
type RouteConfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string            `yaml:"name"`
		Description string            `yaml:"description"`
		Namespace   string            `yaml:"namespace"`
		Enabled     bool              `yaml:"enabled"`
		Labels      map[string]string `yaml:"labels"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		Prefix     string   `yaml:"prefix"`
		Routes     []Route  `yaml:"routes"`
		Middleware []string `yaml:"middleware"`
	} `yaml:"spec"`
}

type Route struct {
	Path        string                 `yaml:"path"`
	Method      interface{}            `yaml:"method"`
	Handler     string                 `yaml:"handler,omitempty"`
	Handlers    map[string]string      `yaml:"handlers,omitempty"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	TestCases   []interface{}          `yaml:"testCases,omitempty"`
	Params      map[string]interface{} `yaml:"params"`
	Auth        interface{}            `yaml:"auth,omitempty"`
}

var lintRules = []LintRule{
	// Naming conventions
	{
		ID:          "naming-001",
		Name:        "Route name format",
		Description: "Route names should use kebab-case",
		Severity:    "warning",
		Check:       checkRouteNaming,
	},
	{
		ID:          "naming-002",
		Name:        "File name consistency",
		Description: "File name should match metadata.name",
		Severity:    "error",
		Check:       checkFileNameConsistency,
	},

	// Required fields
	{
		ID:          "required-001",
		Name:        "Missing API version",
		Description: "apiVersion field is required",
		Severity:    "error",
		Check:       checkAPIVersion,
	},
	{
		ID:          "required-002",
		Name:        "Missing descriptions",
		Description: "Routes should have descriptions",
		Severity:    "warning",
		Check:       checkDescriptions,
	},

	// Path conventions
	{
		ID:          "path-001",
		Name:        "Path format",
		Description: "Paths should start with / and use lowercase",
		Severity:    "error",
		Check:       checkPathFormat,
	},
	{
		ID:          "path-002",
		Name:        "RESTful conventions",
		Description: "Paths should follow RESTful conventions",
		Severity:    "info",
		Check:       checkRESTfulPaths,
	},

	// Security checks
	{
		ID:          "security-001",
		Name:        "Missing authentication",
		Description: "Admin routes should require authentication",
		Severity:    "error",
		Check:       checkAuthentication,
	},
	{
		ID:          "security-002",
		Name:        "Sensitive data exposure",
		Description: "Check for potential sensitive data in paths",
		Severity:    "warning",
		Check:       checkSensitiveData,
	},

	// Performance checks
	{
		ID:          "perf-001",
		Name:        "Wildcard paths",
		Description: "Avoid excessive wildcard paths",
		Severity:    "warning",
		Check:       checkWildcardPaths,
	},
	{
		ID:          "perf-002",
		Name:        "Method consistency",
		Description: "Use specific HTTP methods, not wildcards",
		Severity:    "info",
		Check:       checkMethodSpecificity,
	},

	// Testing
	{
		ID:          "test-001",
		Name:        "Missing test cases",
		Description: "Routes should have test cases",
		Severity:    "info",
		Check:       checkTestCases,
	},

	// Documentation
	{
		ID:          "docs-001",
		Name:        "Parameter documentation",
		Description: "Parameters should be documented",
		Severity:    "warning",
		Check:       checkParameterDocs,
	},
}

func main() {
	routesDir := "./routes"
	if len(os.Args) > 1 {
		routesDir = os.Args[1]
	}

	fmt.Println("🔍 GOTRS Route Linter")
	fmt.Println("====================")
	fmt.Printf("Scanning: %s\n\n", routesDir)

	issues := []LintIssue{}
	fileCount := 0

	// Walk through route files
	err := filepath.Walk(routesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		fileCount++
		fileIssues := lintFile(path)
		issues = append(issues, fileIssues...)

		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	// Display results
	displayResults(issues, fileCount)

	// Exit with error if there are error-level issues
	for _, issue := range issues {
		if issue.Severity == "error" {
			os.Exit(1)
		}
	}
}

func lintFile(path string) []LintIssue {
	issues := []LintIssue{}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		issues = append(issues, LintIssue{
			Rule:     "file-read",
			Severity: "error",
			Message:  fmt.Sprintf("Failed to read file: %v", err),
			File:     path,
		})
		return issues
	}

	// Parse YAML
	var config RouteConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		issues = append(issues, LintIssue{
			Rule:     "yaml-parse",
			Severity: "error",
			Message:  fmt.Sprintf("Invalid YAML: %v", err),
			File:     path,
		})
		return issues
	}

	// Run all lint rules
	for _, rule := range lintRules {
		ruleIssues := rule.Check(&config, path)
		for i := range ruleIssues {
			ruleIssues[i].File = path
			ruleIssues[i].Rule = rule.ID
			ruleIssues[i].Severity = rule.Severity
		}
		issues = append(issues, ruleIssues...)
	}

	return issues
}

// Lint rule implementations

func checkRouteNaming(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	// Check metadata name
	if !isKebabCase(config.Metadata.Name) {
		issues = append(issues, LintIssue{
			Message: fmt.Sprintf("Route name '%s' should use kebab-case", config.Metadata.Name),
		})
	}

	// Check individual route names
	for _, route := range config.Spec.Routes {
		if route.Name != "" && !isKebabCase(route.Name) {
			issues = append(issues, LintIssue{
				Message: fmt.Sprintf("Route name '%s' should use kebab-case", route.Name),
			})
		}
	}

	return issues
}

func checkFileNameConsistency(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	filename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if filename != config.Metadata.Name {
		issues = append(issues, LintIssue{
			Message: fmt.Sprintf("File name '%s' doesn't match metadata.name '%s'", filename, config.Metadata.Name),
		})
	}

	return issues
}

func checkAPIVersion(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	if config.APIVersion == "" {
		issues = append(issues, LintIssue{
			Message: "Missing apiVersion field",
		})
	} else if !strings.HasPrefix(config.APIVersion, "gotrs.io/") {
		issues = append(issues, LintIssue{
			Message: fmt.Sprintf("apiVersion should start with 'gotrs.io/', got '%s'", config.APIVersion),
		})
	}

	return issues
}

func checkDescriptions(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	if config.Metadata.Description == "" {
		issues = append(issues, LintIssue{
			Message: "Missing metadata.description",
		})
	}

	for i, route := range config.Spec.Routes {
		if route.Description == "" {
			issues = append(issues, LintIssue{
				Message: fmt.Sprintf("Route %d (%s) missing description", i, route.Path),
			})
		}
	}

	return issues
}

func checkPathFormat(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	for _, route := range config.Spec.Routes {
		if !strings.HasPrefix(route.Path, "/") {
			issues = append(issues, LintIssue{
				Message: fmt.Sprintf("Path '%s' should start with /", route.Path),
			})
		}

		// Check for uppercase in path (except for path params)
		pathParts := strings.Split(route.Path, "/")
		for _, part := range pathParts {
			if !strings.HasPrefix(part, ":") && part != strings.ToLower(part) {
				issues = append(issues, LintIssue{
					Message: fmt.Sprintf("Path '%s' should use lowercase", route.Path),
				})
				break
			}
		}
	}

	return issues
}

func checkRESTfulPaths(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	for _, route := range config.Spec.Routes {
		methods := getMethodList(route.Method)

		// Check common anti-patterns
		if strings.Contains(route.Path, "/get") || strings.Contains(route.Path, "/create") ||
			strings.Contains(route.Path, "/update") || strings.Contains(route.Path, "/delete") {
			issues = append(issues, LintIssue{
				Message: fmt.Sprintf("Path '%s' includes HTTP verbs - use HTTP methods instead", route.Path),
			})
		}

		// Check for appropriate methods
		if strings.HasSuffix(route.Path, "/:id") {
			if !contains(methods, "GET") && !contains(methods, "PUT") && !contains(methods, "DELETE") {
				issues = append(issues, LintIssue{
					Message: fmt.Sprintf("Resource path '%s' typically uses GET/PUT/DELETE methods", route.Path),
				})
			}
		}
	}

	return issues
}

func checkAuthentication(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	// Check if this is an admin route
	if config.Metadata.Namespace == "admin" || strings.Contains(config.Spec.Prefix, "/admin") {
		for _, route := range config.Spec.Routes {
			if route.Auth == nil {
				issues = append(issues, LintIssue{
					Message: fmt.Sprintf("Admin route '%s' should require authentication", route.Path),
				})
			}
		}
	}

	return issues
}

func checkSensitiveData(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	sensitivePatterns := []string{"password", "token", "secret", "key", "credential"}

	for _, route := range config.Spec.Routes {
		for _, pattern := range sensitivePatterns {
			if strings.Contains(strings.ToLower(route.Path), pattern) {
				issues = append(issues, LintIssue{
					Message: fmt.Sprintf("Path '%s' may expose sensitive data", route.Path),
				})
			}
		}
	}

	return issues
}

func checkWildcardPaths(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	wildcardCount := 0
	for _, route := range config.Spec.Routes {
		if strings.Contains(route.Path, "*") {
			wildcardCount++
			issues = append(issues, LintIssue{
				Message: fmt.Sprintf("Wildcard path '%s' may impact performance", route.Path),
			})
		}
	}

	if wildcardCount > 3 {
		issues = append(issues, LintIssue{
			Message: fmt.Sprintf("Too many wildcard paths (%d) in route group", wildcardCount),
		})
	}

	return issues
}

func checkMethodSpecificity(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	for _, route := range config.Spec.Routes {
		methods := getMethodList(route.Method)
		if contains(methods, "*") || contains(methods, "ANY") {
			issues = append(issues, LintIssue{
				Message: fmt.Sprintf("Route '%s' uses wildcard method - be specific", route.Path),
			})
		}
	}

	return issues
}

func checkTestCases(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	routesWithoutTests := 0
	for _, route := range config.Spec.Routes {
		if len(route.TestCases) == 0 {
			routesWithoutTests++
		}
	}

	if routesWithoutTests > 0 {
		issues = append(issues, LintIssue{
			Message: fmt.Sprintf("%d routes lack test cases", routesWithoutTests),
		})
	}

	return issues
}

func checkParameterDocs(config *RouteConfig, path string) []LintIssue {
	issues := []LintIssue{}

	for _, route := range config.Spec.Routes {
		// Check if path has parameters
		if strings.Contains(route.Path, ":") {
			paramCount := strings.Count(route.Path, ":")
			if len(route.Params) < paramCount {
				issues = append(issues, LintIssue{
					Message: fmt.Sprintf("Route '%s' has undocumented path parameters", route.Path),
				})
			}
		}
	}

	return issues
}

// Helper functions

func isKebabCase(s string) bool {
	return regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`).MatchString(s)
}

func getMethodList(method interface{}) []string {
	methods := []string{}
	switch v := method.(type) {
	case string:
		methods = append(methods, v)
	case []interface{}:
		for _, m := range v {
			if ms, ok := m.(string); ok {
				methods = append(methods, ms)
			}
		}
	}
	return methods
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func displayResults(issues []LintIssue, fileCount int) {
	// Group issues by severity
	errors := []LintIssue{}
	warnings := []LintIssue{}
	infos := []LintIssue{}

	for _, issue := range issues {
		switch issue.Severity {
		case "error":
			errors = append(errors, issue)
		case "warning":
			warnings = append(warnings, issue)
		case "info":
			infos = append(infos, issue)
		}
	}

	// Display grouped issues
	if len(errors) > 0 {
		fmt.Printf("\n❌ Errors (%d):\n", len(errors))
		displayIssueGroup(errors)
	}

	if len(warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings (%d):\n", len(warnings))
		displayIssueGroup(warnings)
	}

	if len(infos) > 0 {
		fmt.Printf("\nℹ️  Info (%d):\n", len(infos))
		displayIssueGroup(infos)
	}

	// Summary
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("   Files scanned: %d\n", fileCount)
	fmt.Printf("   Errors:        %d\n", len(errors))
	fmt.Printf("   Warnings:      %d\n", len(warnings))
	fmt.Printf("   Info:          %d\n", len(infos))

	if len(issues) == 0 {
		fmt.Printf("\n✅ All checks passed!\n")
	} else if len(errors) == 0 {
		fmt.Printf("\n✅ No errors found (warnings exist)\n")
	} else {
		fmt.Printf("\n❌ Fix errors before proceeding\n")
	}
}

func displayIssueGroup(issues []LintIssue) {
	// Group by file
	byFile := make(map[string][]LintIssue)
	for _, issue := range issues {
		byFile[issue.File] = append(byFile[issue.File], issue)
	}

	// Sort files
	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)

	// Display
	for _, file := range files {
		relFile, _ := filepath.Rel(".", file)
		fmt.Printf("\n   %s:\n", relFile)
		for _, issue := range byFile[file] {
			fmt.Printf("      [%s] %s\n", issue.Rule, issue.Message)
		}
	}
}
