// Package main provides route testing utilities.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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
	} `yaml:"metadata"`
	Spec struct {
		Prefix string  `yaml:"prefix"`
		Routes []Route `yaml:"routes"`
	} `yaml:"spec"`
}

type Route struct {
	Path        string               `yaml:"path"`
	Method      interface{}          `yaml:"method"` // Can be string or []string
	Handler     string               `yaml:"handler,omitempty"`
	Handlers    map[string]string    `yaml:"handlers,omitempty"`
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	TestCases   []TestCase           `yaml:"testCases,omitempty"`
	Params      map[string]ParamSpec `yaml:"params"`
	Auth        *AuthSpec            `yaml:"auth,omitempty"`
}

type TestCase struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description,omitempty"`
	Method      string                 `yaml:"method,omitempty"`
	Input       map[string]interface{} `yaml:"input,omitempty"`
	Headers     map[string]string      `yaml:"headers,omitempty"`
	StatusCode  int                    `yaml:"statusCode"`
	Contains    []string               `yaml:"contains,omitempty"`
	NotContains []string               `yaml:"notContains,omitempty"`
	JSONPath    map[string]interface{} `yaml:"jsonPath,omitempty"`
	Skip        bool                   `yaml:"skip,omitempty"`
	SkipReason  string                 `yaml:"skipReason,omitempty"`
}

type ParamSpec struct {
	Type        string      `yaml:"type"`
	Required    bool        `yaml:"required"`
	Description string      `yaml:"description"`
	Default     interface{} `yaml:"default"`
}

type AuthSpec struct {
	Required bool   `yaml:"required"`
	Type     string `yaml:"type"`
	Token    string `yaml:"token,omitempty"`
}

// TestRunner executes tests against YAML-defined routes.
type TestRunner struct {
	baseURL   string
	client    *http.Client
	routes    []RouteConfig
	results   []TestResult
	authToken string
	verbose   bool
}

type TestResult struct {
	RouteGroup   string        `json:"route_group"`
	RouteName    string        `json:"route_name"`
	TestName     string        `json:"test_name"`
	Method       string        `json:"method"`
	URL          string        `json:"url"`
	Expected     int           `json:"expected_status"`
	Actual       int           `json:"actual_status"`
	Duration     time.Duration `json:"duration"`
	Passed       bool          `json:"passed"`
	Error        string        `json:"error,omitempty"`
	ResponseBody string        `json:"response_body,omitempty"`
	Skipped      bool          `json:"skipped,omitempty"`
	SkipReason   string        `json:"skip_reason,omitempty"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("🧪 GOTRS Route Testing Framework")
		fmt.Println("")
		fmt.Println("Usage: route-test <routes-dir> <base-url> [options]")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  route-test ./routes http://localhost:8080")
		fmt.Println("  route-test ./routes http://localhost:8080 --verbose")
		fmt.Println("  route-test ./routes http://localhost:8080 --auth Bearer:token123")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --verbose     Show detailed test output")
		fmt.Println("  --auth TYPE:TOKEN  Authentication (e.g., Bearer:abc123)")
		fmt.Println("  --output FILE      Save results to JSON file")
		os.Exit(1)
	}

	routesDir := os.Args[1]
	baseURL := os.Args[2]

	runner := &TestRunner{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects - we want to test the actual response
				return http.ErrUseLastResponse
			},
		},
	}

	// Parse command line options
	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "--verbose":
			runner.verbose = true
		case strings.HasPrefix(arg, "--auth"):
			if i+1 < len(os.Args) {
				authParts := strings.SplitN(os.Args[i+1], ":", 2)
				if len(authParts) == 2 {
					runner.authToken = fmt.Sprintf("%s %s", authParts[0], authParts[1])
					i++ // Skip next arg
				}
			}
		}
	}

	if err := runner.LoadRoutes(routesDir); err != nil {
		log.Fatalf("Failed to load routes: %v", err)
	}

	fmt.Printf("🚀 Testing %d route groups against %s\n", len(runner.routes), baseURL)
	if runner.authToken != "" {
		fmt.Printf("🔐 Using authentication: %s\n", strings.Split(runner.authToken, " ")[0])
	}
	fmt.Println()

	start := time.Now()
	runner.RunTests()
	duration := time.Since(start)

	runner.PrintResults()

	// Print summary
	passed := 0
	failed := 0
	skipped := 0

	for _, result := range runner.results {
		if result.Skipped {
			skipped++
		} else if result.Passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("\n📊 Test Summary (completed in %s)\n", duration.Round(time.Millisecond))
	fmt.Printf("✅ Passed: %d\n", passed)
	if failed > 0 {
		fmt.Printf("❌ Failed: %d\n", failed)
	}
	if skipped > 0 {
		fmt.Printf("⏭️  Skipped: %d\n", skipped)
	}
	fmt.Printf("📈 Total: %d tests\n", len(runner.results))

	if failed > 0 {
		os.Exit(1)
	}
}

func (tr *TestRunner) LoadRoutes(routesDir string) error {
	return filepath.Walk(routesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		var config RouteConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		// Skip disabled route groups
		if !config.Metadata.Enabled {
			if tr.verbose {
				fmt.Printf("⏭️  Skipping disabled route group: %s\n", config.Metadata.Name)
			}
			return nil
		}

		tr.routes = append(tr.routes, config)
		return nil
	})
}

func (tr *TestRunner) RunTests() {
	for _, routeGroup := range tr.routes {
		fmt.Printf("🔍 Testing route group: %s (%s)\n",
			routeGroup.Metadata.Name, routeGroup.Metadata.Namespace)

		for _, route := range routeGroup.Spec.Routes {
			tr.testRoute(routeGroup, route)
		}
	}
}

func (tr *TestRunner) testRoute(routeGroup RouteConfig, route Route) {
	fullPath := routeGroup.Spec.Prefix + route.Path

	// If no test cases defined, create basic tests
	if len(route.TestCases) == 0 {
		tr.createBasicTests(routeGroup, route, fullPath)
		return
	}

	// Run defined test cases
	for _, testCase := range route.TestCases {
		if testCase.Skip {
			tr.results = append(tr.results, TestResult{
				RouteGroup: routeGroup.Metadata.Name,
				RouteName:  route.Name,
				TestName:   testCase.Name,
				Skipped:    true,
				SkipReason: testCase.SkipReason,
			})
			if tr.verbose {
				fmt.Printf("  ⏭️  %s: %s (skipped: %s)\n",
					route.Name, testCase.Name, testCase.SkipReason)
			}
			continue
		}

		method := testCase.Method
		if method == "" {
			// Use first method from route definition
			switch v := route.Method.(type) {
			case string:
				method = v
			case []interface{}:
				if len(v) > 0 {
					method = v[0].(string)
				}
			}
		}

		tr.runTestCase(routeGroup, route, testCase, fullPath, method)
	}
}

func (tr *TestRunner) createBasicTests(routeGroup RouteConfig, route Route, fullPath string) {
	methods := []string{}
	switch v := route.Method.(type) {
	case string:
		methods = append(methods, v)
	case []interface{}:
		for _, method := range v {
			methods = append(methods, method.(string))
		}
	}

	for _, method := range methods {
		testCase := TestCase{
			Name:       fmt.Sprintf("Basic %s test", method),
			StatusCode: 200, // Assume success, but redirects (3xx) are also ok
		}

		// Adjust expected status for different methods and auth requirements
		if method == "POST" || method == "PUT" || method == "DELETE" {
			testCase.StatusCode = 200 // Could be 201, 202, etc.
		}

		tr.runTestCase(routeGroup, route, testCase, fullPath, method)
	}
}

func (tr *TestRunner) runTestCase(routeGroup RouteConfig, route Route, testCase TestCase, fullPath, method string) {
	start := time.Now()

	// Replace path parameters with test values
	testURL := tr.baseURL + fullPath
	testURL = strings.ReplaceAll(testURL, ":id", "1")
	testURL = strings.ReplaceAll(testURL, ":username", "testuser")

	// Prepare request body
	var body io.Reader
	if len(testCase.Input) > 0 {
		if method == "GET" {
			// Add as query parameters
			params := make([]string, 0)
			for key, value := range testCase.Input {
				params = append(params, fmt.Sprintf("%s=%v", key, value))
			}
			if len(params) > 0 {
				testURL += "?" + strings.Join(params, "&")
			}
		} else {
			// Add as JSON body
			jsonData, _ := json.Marshal(testCase.Input)
			body = bytes.NewReader(jsonData)
		}
	}

	// Create request
	req, err := http.NewRequest(method, testURL, body)
	if err != nil {
		tr.recordResult(routeGroup, route, testCase, testURL, method, 0, 0, time.Since(start), false, err.Error(), "")
		return
	}

	// Add headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tr.authToken != "" {
		req.Header.Set("Authorization", tr.authToken)
	}
	for key, value := range testCase.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := tr.client.Do(req)
	if err != nil {
		tr.recordResult(routeGroup, route, testCase, testURL, method, testCase.StatusCode, 0, time.Since(start), false, err.Error(), "")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		responseBody = []byte("Error reading response body")
	}

	duration := time.Since(start)

	// Check result
	passed := tr.checkResult(testCase, resp.StatusCode, string(responseBody))

	tr.recordResult(routeGroup, route, testCase, testURL, method, testCase.StatusCode, resp.StatusCode, duration, passed, "", string(responseBody))
}

func (tr *TestRunner) checkResult(testCase TestCase, actualStatus int, responseBody string) bool {
	// Check status code (allow some flexibility for redirects and auth)
	statusOK := false
	if testCase.StatusCode == actualStatus {
		statusOK = true
	} else if testCase.StatusCode == 200 {
		// Accept 2xx, 3xx as success for basic tests
		statusOK = actualStatus >= 200 && actualStatus < 400
	}

	if !statusOK {
		return false
	}

	// Check contains
	for _, contain := range testCase.Contains {
		if !strings.Contains(responseBody, contain) {
			return false
		}
	}

	// Check not contains
	for _, notContain := range testCase.NotContains {
		if strings.Contains(responseBody, notContain) {
			return false
		}
	}

	return true
}

func (tr *TestRunner) recordResult(routeGroup RouteConfig, route Route, testCase TestCase, url, method string, expected, actual int, duration time.Duration, passed bool, error, responseBody string) {
	result := TestResult{
		RouteGroup:   routeGroup.Metadata.Name,
		RouteName:    route.Name,
		TestName:     testCase.Name,
		Method:       method,
		URL:          url,
		Expected:     expected,
		Actual:       actual,
		Duration:     duration,
		Passed:       passed,
		Error:        error,
		ResponseBody: responseBody,
	}

	tr.results = append(tr.results, result)

	// Print immediate feedback
	if tr.verbose || !passed {
		status := "✅"
		if !passed {
			status = "❌"
		}

		fmt.Printf("  %s %s %s %s -> %d (expected %d) [%s]\n",
			status, method, route.Name, testCase.Name, actual, expected, duration.Round(time.Millisecond))

		if !passed && error != "" {
			fmt.Printf("     Error: %s\n", error)
		}
	}
}

func (tr *TestRunner) PrintResults() {
	if !tr.verbose {
		return
	}

	fmt.Println("\n📋 Detailed Results:")
	fmt.Println("=" + strings.Repeat("=", 80))

	for _, result := range tr.results {
		if result.Skipped {
			continue
		}

		status := "✅ PASS"
		if !result.Passed {
			status = "❌ FAIL"
		}

		fmt.Printf("%s | %s | %s %s\n", status, result.RouteGroup, result.Method, result.RouteName)
		fmt.Printf("     URL: %s\n", result.URL)
		fmt.Printf("     Expected: %d, Got: %d, Duration: %s\n",
			result.Expected, result.Actual, result.Duration.Round(time.Millisecond))

		if result.Error != "" {
			fmt.Printf("     Error: %s\n", result.Error)
		}

		if !result.Passed && len(result.ResponseBody) > 0 && len(result.ResponseBody) < 200 {
			fmt.Printf("     Response: %s\n", strings.TrimSpace(result.ResponseBody))
		}

		fmt.Println()
	}
}
