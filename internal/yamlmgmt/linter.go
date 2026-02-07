package yamlmgmt

import (
	"fmt"
	"regexp"
	"strings"
)

// UniversalLinter provides linting for all YAML document types.
type UniversalLinter struct {
	rules map[YAMLKind][]LintRule
}

// NewUniversalLinter creates a new universal linter.
func NewUniversalLinter() *UniversalLinter {
	ul := &UniversalLinter{
		rules: make(map[YAMLKind][]LintRule),
	}

	// Register default rules
	ul.registerDefaultRules()

	return ul
}

// Lint performs linting on a document.
func (ul *UniversalLinter) Lint(doc *YAMLDocument) ([]LintIssue, error) {
	kind := YAMLKind(doc.Kind)
	issues := []LintIssue{}

	// Apply universal rules
	issues = append(issues, ul.applyUniversalRules(doc)...)

	// Apply kind-specific rules
	if rules, exists := ul.rules[kind]; exists {
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}

			ruleIssues := ul.applyRule(rule, doc)
			issues = append(issues, ruleIssues...)
		}
	}

	return issues, nil
}

// GetRules returns rules for a specific kind.
func (ul *UniversalLinter) GetRules(kind YAMLKind) []LintRule {
	if rules, exists := ul.rules[kind]; exists {
		return rules
	}
	return []LintRule{}
}

// RegisterRule registers a new lint rule.
func (ul *UniversalLinter) RegisterRule(kind YAMLKind, rule LintRule) {
	if _, exists := ul.rules[kind]; !exists {
		ul.rules[kind] = []LintRule{}
	}
	ul.rules[kind] = append(ul.rules[kind], rule)
}

// applyUniversalRules applies rules that work for all document types.
func (ul *UniversalLinter) applyUniversalRules(doc *YAMLDocument) []LintIssue {
	issues := []LintIssue{}

	// Check metadata
	if doc.Metadata.Name == "" {
		issues = append(issues, LintIssue{
			Severity: "error",
			Rule:     "universal-001",
			Message:  "Missing metadata.name",
			Path:     "metadata.name",
		})
	}

	// Check naming conventions
	if doc.Metadata.Name != "" && !isValidName(doc.Metadata.Name) {
		issues = append(issues, LintIssue{
			Severity: "warning",
			Rule:     "universal-002",
			Message:  fmt.Sprintf("Name '%s' should use kebab-case", doc.Metadata.Name),
			Path:     "metadata.name",
		})
	}

	// Check description
	if doc.Metadata.Description == "" {
		issues = append(issues, LintIssue{
			Severity: "info",
			Rule:     "universal-003",
			Message:  "Missing metadata.description",
			Path:     "metadata.description",
		})
	}

	// Check API version format
	if doc.APIVersion != "" {
		if !regexp.MustCompile(`^[a-z]+(\.[a-z]+)?/v[0-9]+$`).MatchString(doc.APIVersion) {
			issues = append(issues, LintIssue{
				Severity: "warning",
				Rule:     "universal-004",
				Message:  fmt.Sprintf("API version '%s' should follow format: domain/version (e.g., goatflow.io/v1)", doc.APIVersion),
				Path:     "apiVersion",
			})
		}
	}

	return issues
}

// applyRule applies a specific rule to a document.
func (ul *UniversalLinter) applyRule(rule LintRule, doc *YAMLDocument) []LintIssue {
	issues := []LintIssue{}

	// This is simplified - real implementation would be more sophisticated
	switch rule.ID {
	case "route-security-001":
		// Check for missing authentication on admin routes
		if spec, ok := doc.Spec.(map[string]interface{}); ok {
			if prefix, ok := spec["prefix"].(string); ok && strings.Contains(prefix, "/admin") {
				if routes, ok := spec["routes"].([]interface{}); ok {
					for i, route := range routes {
						if r, ok := route.(map[string]interface{}); ok {
							if auth := r["auth"]; auth == nil {
								issues = append(issues, LintIssue{
									Severity: rule.Severity,
									Rule:     rule.ID,
									Message:  fmt.Sprintf("Admin route missing authentication: %v", r["path"]),
									Path:     fmt.Sprintf("spec.routes[%d]", i),
								})
							}
						}
					}
				}
			}
		}

	case "config-security-001":
		// Check for sensitive values in configuration
		if settings, ok := doc.Data["settings"].([]interface{}); ok {
			for i, setting := range settings {
				if s, ok := setting.(map[string]interface{}); ok {
					if name, ok := s["name"].(string); ok {
						if containsSensitiveWord(name) && s["readonly"] != true {
							issues = append(issues, LintIssue{
								Severity: rule.Severity,
								Rule:     rule.ID,
								Message:  fmt.Sprintf("Sensitive setting '%s' should be readonly", name),
								Path:     fmt.Sprintf("settings[%d]", i),
							})
						}
					}
				}
			}
		}

	case "dashboard-perf-001":
		// Check for too many tiles on dashboard
		if spec, ok := doc.Spec.(map[string]interface{}); ok {
			if dashboard, ok := spec["dashboard"].(map[string]interface{}); ok {
				if tiles, ok := dashboard["tiles"].([]interface{}); ok && len(tiles) > 20 {
					issues = append(issues, LintIssue{
						Severity: rule.Severity,
						Rule:     rule.ID,
						Message:  fmt.Sprintf("Dashboard has %d tiles, consider pagination (recommended: max 20)", len(tiles)),
						Path:     "spec.dashboard.tiles",
					})
				}
			}
		}
	}

	return issues
}

// registerDefaultRules registers built-in lint rules.
func (ul *UniversalLinter) registerDefaultRules() {
	// Route rules
	ul.RegisterRule(KindRoute, LintRule{
		ID:          "route-security-001",
		Name:        "Admin route authentication",
		Description: "Admin routes should require authentication",
		Severity:    "error",
		Enabled:     true,
	})

	ul.RegisterRule(KindRoute, LintRule{
		ID:          "route-path-001",
		Name:        "RESTful path conventions",
		Description: "Paths should follow RESTful conventions",
		Severity:    "warning",
		Enabled:     true,
	})

	ul.RegisterRule(KindRoute, LintRule{
		ID:          "route-test-001",
		Name:        "Missing test cases",
		Description: "Routes should have test cases",
		Severity:    "info",
		Enabled:     true,
	})

	// Config rules
	ul.RegisterRule(KindConfig, LintRule{
		ID:          "config-security-001",
		Name:        "Sensitive settings protection",
		Description: "Sensitive settings should be readonly",
		Severity:    "error",
		Enabled:     true,
	})

	ul.RegisterRule(KindConfig, LintRule{
		ID:          "config-validation-001",
		Name:        "Missing validation rules",
		Description: "Settings should have validation rules",
		Severity:    "warning",
		Enabled:     true,
	})

	ul.RegisterRule(KindConfig, LintRule{
		ID:          "config-defaults-001",
		Name:        "Missing default values",
		Description: "Settings should have default values",
		Severity:    "info",
		Enabled:     true,
	})

	// Dashboard rules
	ul.RegisterRule(KindDashboard, LintRule{
		ID:          "dashboard-perf-001",
		Name:        "Too many tiles",
		Description: "Dashboard should not have too many tiles",
		Severity:    "warning",
		Enabled:     true,
	})

	ul.RegisterRule(KindDashboard, LintRule{
		ID:          "dashboard-access-001",
		Name:        "Missing access control",
		Description: "Dashboard should define access control",
		Severity:    "warning",
		Enabled:     true,
	})

	ul.RegisterRule(KindDashboard, LintRule{
		ID:          "dashboard-color-001",
		Name:        "Color consistency",
		Description: "Dashboard should use consistent color scheme",
		Severity:    "info",
		Enabled:     true,
	})
}

// Helper functions

func isValidName(name string) bool {
	// Check for kebab-case
	return regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`).MatchString(name)
}

func containsSensitiveWord(s string) bool {
	sensitive := []string{"password", "secret", "key", "token", "credential", "auth"}
	lower := strings.ToLower(s)
	for _, word := range sensitive {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}
