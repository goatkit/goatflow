package yamlmgmt

import (
	"testing"
)

func TestNewUniversalLinter(t *testing.T) {
	ul := NewUniversalLinter()

	if ul == nil {
		t.Fatal("expected non-nil UniversalLinter")
	}
	if ul.rules == nil {
		t.Error("expected rules map to be initialized")
	}

	// Check default rules are registered
	routeRules := ul.GetRules(KindRoute)
	if len(routeRules) == 0 {
		t.Error("expected route rules to be registered")
	}

	configRules := ul.GetRules(KindConfig)
	if len(configRules) == 0 {
		t.Error("expected config rules to be registered")
	}

	dashboardRules := ul.GetRules(KindDashboard)
	if len(dashboardRules) == 0 {
		t.Error("expected dashboard rules to be registered")
	}
}

func TestUniversalLinter_RegisterRule(t *testing.T) {
	ul := &UniversalLinter{rules: make(map[YAMLKind][]LintRule)}

	rule := LintRule{
		ID:          "test-001",
		Name:        "Test Rule",
		Description: "A test rule",
		Severity:    "warning",
		Enabled:     true,
	}

	ul.RegisterRule(KindRoute, rule)

	rules := ul.GetRules(KindRoute)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "test-001" {
		t.Errorf("expected rule ID 'test-001', got %s", rules[0].ID)
	}
}

func TestUniversalLinter_GetRules_Empty(t *testing.T) {
	ul := &UniversalLinter{rules: make(map[YAMLKind][]LintRule)}

	rules := ul.GetRules(YAMLKind("nonexistent"))
	if len(rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(rules))
	}
}

func TestUniversalLinter_Lint_MissingName(t *testing.T) {
	ul := NewUniversalLinter()

	doc := &YAMLDocument{
		APIVersion: "goatflow.io/v1",
		Kind:       "Route",
		Metadata:   Metadata{Name: ""},
	}

	issues, err := ul.Lint(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.Rule == "universal-001" {
			found = true
			if issue.Severity != "error" {
				t.Errorf("expected severity 'error', got %s", issue.Severity)
			}
			if issue.Path != "metadata.name" {
				t.Errorf("expected path 'metadata.name', got %s", issue.Path)
			}
		}
	}
	if !found {
		t.Error("expected universal-001 issue for missing name")
	}
}

func TestUniversalLinter_Lint_InvalidNameFormat(t *testing.T) {
	ul := NewUniversalLinter()

	doc := &YAMLDocument{
		APIVersion: "goatflow.io/v1",
		Kind:       "Route",
		Metadata:   Metadata{Name: "InvalidCamelCase"},
	}

	issues, err := ul.Lint(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.Rule == "universal-002" {
			found = true
			if issue.Severity != "warning" {
				t.Errorf("expected severity 'warning', got %s", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected universal-002 issue for invalid name format")
	}
}

func TestUniversalLinter_Lint_MissingDescription(t *testing.T) {
	ul := NewUniversalLinter()

	doc := &YAMLDocument{
		APIVersion: "goatflow.io/v1",
		Kind:       "Route",
		Metadata:   Metadata{Name: "valid-name", Description: ""},
	}

	issues, err := ul.Lint(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.Rule == "universal-003" {
			found = true
			if issue.Severity != "info" {
				t.Errorf("expected severity 'info', got %s", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected universal-003 issue for missing description")
	}
}

func TestUniversalLinter_Lint_InvalidAPIVersion(t *testing.T) {
	ul := NewUniversalLinter()

	doc := &YAMLDocument{
		APIVersion: "invalid-format",
		Kind:       "Route",
		Metadata:   Metadata{Name: "valid-name"},
	}

	issues, err := ul.Lint(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.Rule == "universal-004" {
			found = true
			if issue.Severity != "warning" {
				t.Errorf("expected severity 'warning', got %s", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected universal-004 issue for invalid API version")
	}
}

func TestUniversalLinter_Lint_ValidDocument(t *testing.T) {
	ul := NewUniversalLinter()

	doc := &YAMLDocument{
		APIVersion: "goatflow.io/v1",
		Kind:       "Route",
		Metadata: Metadata{
			Name:        "valid-name",
			Description: "A valid description",
		},
	}

	issues, err := ul.Lint(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	universalIssues := 0
	for _, issue := range issues {
		if issue.Rule == "universal-001" || issue.Rule == "universal-002" ||
			issue.Rule == "universal-003" || issue.Rule == "universal-004" {
			universalIssues++
		}
	}
	if universalIssues > 0 {
		t.Errorf("expected no universal issues for valid doc, got %d", universalIssues)
	}
}

func TestUniversalLinter_Lint_DisabledRule(t *testing.T) {
	ul := &UniversalLinter{rules: make(map[YAMLKind][]LintRule)}

	ul.RegisterRule(KindRoute, LintRule{
		ID:       "disabled-rule",
		Name:     "Disabled Rule",
		Severity: "error",
		Enabled:  false,
	})

	doc := &YAMLDocument{
		Kind:     string(KindRoute),
		Metadata: Metadata{Name: "test"},
	}

	issues, _ := ul.Lint(doc)

	for _, issue := range issues {
		if issue.Rule == "disabled-rule" {
			t.Error("disabled rule should not produce issues")
		}
	}
}

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"valid-name", true},
		{"simple", true},
		{"multi-word-name", true},
		{"name123", true},
		{"name-123-test", true},
		{"InvalidCamelCase", false},
		{"UPPERCASE", false},
		{"with_underscore", false},
		{"with spaces", false},
		{"-starts-with-dash", false},
		{"ends-with-dash-", false},
		{"123startswithnumber", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidName(tt.name)
			if result != tt.expected {
				t.Errorf("isValidName(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestContainsSensitiveWord(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"password", true},
		{"user_password", true},
		{"PASSWORD", true},
		{"api_secret", true},
		{"secret_key", true},
		{"access_token", true},
		{"auth_token", true},
		{"credential", true},
		{"credentials", true},
		{"api_key", true},
		{"username", false},
		{"email", false},
		{"name", false},
		{"title", false},
		{"description", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsSensitiveWord(tt.input)
			if result != tt.expected {
				t.Errorf("containsSensitiveWord(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLintRule_Fields(t *testing.T) {
	rule := LintRule{
		ID:          "test-rule-001",
		Name:        "Test Rule",
		Description: "A test rule for testing",
		Severity:    "warning",
		Enabled:     true,
	}

	if rule.ID != "test-rule-001" {
		t.Errorf("expected ID 'test-rule-001', got %s", rule.ID)
	}
	if rule.Name != "Test Rule" {
		t.Errorf("expected Name 'Test Rule', got %s", rule.Name)
	}
	if rule.Description != "A test rule for testing" {
		t.Errorf("expected Description 'A test rule for testing', got %s", rule.Description)
	}
	if rule.Severity != "warning" {
		t.Errorf("expected Severity 'warning', got %s", rule.Severity)
	}
	if !rule.Enabled {
		t.Error("expected Enabled to be true")
	}
}

func TestLintIssue_Fields(t *testing.T) {
	issue := LintIssue{
		Severity: "error",
		Rule:     "rule-001",
		Message:  "Something is wrong",
		Path:     "spec.routes[0]",
	}

	if issue.Severity != "error" {
		t.Errorf("expected Severity 'error', got %s", issue.Severity)
	}
	if issue.Rule != "rule-001" {
		t.Errorf("expected Rule 'rule-001', got %s", issue.Rule)
	}
	if issue.Message != "Something is wrong" {
		t.Errorf("expected Message 'Something is wrong', got %s", issue.Message)
	}
	if issue.Path != "spec.routes[0]" {
		t.Errorf("expected Path 'spec.routes[0]', got %s", issue.Path)
	}
}

func TestUniversalLinter_ApplyRule_RouteSecurityAdmin(t *testing.T) {
	ul := NewUniversalLinter()

	doc := &YAMLDocument{
		Kind:     string(KindRoute),
		Metadata: Metadata{Name: "admin-routes"},
		Spec: map[string]interface{}{
			"prefix": "/admin",
			"routes": []interface{}{
				map[string]interface{}{
					"path": "/users",
					// Missing "auth" field
				},
			},
		},
	}

	issues, _ := ul.Lint(doc)

	found := false
	for _, issue := range issues {
		if issue.Rule == "route-security-001" {
			found = true
		}
	}
	if !found {
		t.Error("expected route-security-001 issue for admin route without auth")
	}
}

func TestUniversalLinter_ApplyRule_ConfigSecurity(t *testing.T) {
	ul := NewUniversalLinter()

	doc := &YAMLDocument{
		Kind:     string(KindConfig),
		Metadata: Metadata{Name: "config"},
		Data: map[string]interface{}{
			"settings": []interface{}{
				map[string]interface{}{
					"name":     "api_password",
					"readonly": false,
				},
			},
		},
	}

	issues, _ := ul.Lint(doc)

	found := false
	for _, issue := range issues {
		if issue.Rule == "config-security-001" {
			found = true
		}
	}
	if !found {
		t.Error("expected config-security-001 issue for sensitive setting without readonly")
	}
}

func TestUniversalLinter_ApplyRule_DashboardPerformance(t *testing.T) {
	ul := NewUniversalLinter()

	tiles := make([]interface{}, 25)
	for i := 0; i < 25; i++ {
		tiles[i] = map[string]interface{}{"id": i}
	}

	doc := &YAMLDocument{
		Kind:     string(KindDashboard),
		Metadata: Metadata{Name: "dashboard"},
		Spec: map[string]interface{}{
			"dashboard": map[string]interface{}{
				"tiles": tiles,
			},
		},
	}

	issues, _ := ul.Lint(doc)

	found := false
	for _, issue := range issues {
		if issue.Rule == "dashboard-perf-001" {
			found = true
		}
	}
	if !found {
		t.Error("expected dashboard-perf-001 issue for too many tiles")
	}
}
