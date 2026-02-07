// Package main provides the contract test runner.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/api"
	"github.com/goatkit/goatflow/internal/routing"
)

// ContractResult represents the result of a contract test.
type ContractResult struct {
	Endpoint    string `json:"endpoint"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Passed      bool   `json:"passed"`
	Error       string `json:"error,omitempty"`
}

// ContractReport represents the overall test report.
type ContractReport struct {
	Timestamp   time.Time        `json:"timestamp"`
	TotalTests  int              `json:"total_tests"`
	Passed      int              `json:"passed"`
	Failed      int              `json:"failed"`
	Results     []ContractResult `json:"results"`
	SuccessRate float64          `json:"success_rate"`
}

func main() {
	// Set up Gin router
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Register all handlers
	registry := routing.NewHandlerRegistry()
	if err := api.RegisterWithRouting(registry); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register handlers: %v\n", err)
		os.Exit(1)
	}

	// Load routes from YAML
	if err := routing.LoadYAMLRoutes(r, "./routes", registry); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load routes: %v\n", err)
		os.Exit(1)
	}

	results := []ContractResult{}

	// Test Authentication Contracts
	authContracts := []struct {
		endpoint    string
		method      string
		description string
		body        interface{}
		expectCode  int
	}{
		{
			endpoint:    "/api/v1/auth/login",
			method:      "POST",
			description: "Valid login returns JWT tokens",
			body: map[string]string{
				"login":    "test",
				"password": "test123",
			},
			expectCode: 200,
		},
		{
			endpoint:    "/api/v1/auth/login",
			method:      "POST",
			description: "Invalid credentials return 401",
			body: map[string]string{
				"login":    "bad",
				"password": "wrong",
			},
			expectCode: 401,
		},
		{
			endpoint:    "/api/v1/auth/logout",
			method:      "POST",
			description: "Logout always succeeds",
			body:        nil,
			expectCode:  200,
		},
	}

	for _, contract := range authContracts {
		result := testContract(r, contract.method, contract.endpoint, contract.body, contract.expectCode)
		result.Description = contract.description
		results = append(results, result)
	}

	// Test Ticket Contracts
	ticketContracts := []struct {
		endpoint    string
		method      string
		description string
		body        interface{}
		headers     map[string]string
		expectCode  int
	}{
		{
			endpoint:    "/api/v1/tickets",
			method:      "GET",
			description: "List tickets requires authentication",
			body:        nil,
			headers:     nil,
			expectCode:  401,
		},
		{
			endpoint:    "/api/v1/tickets/1/close",
			method:      "POST",
			description: "Close ticket requires resolution",
			body: map[string]string{
				"resolution": "resolved",
			},
			headers: map[string]string{
				"Authorization": "Bearer token",
			},
			expectCode: 200,
		},
	}

	for _, contract := range ticketContracts {
		result := testContractWithHeaders(r, contract.method, contract.endpoint, contract.body, contract.headers, contract.expectCode)
		result.Description = contract.description
		results = append(results, result)
	}

	// Generate report
	report := generateReport(results)

	// Print results
	printReport(report)

	// Save report to file
	if err := saveReport(report); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save report: %v\n", err)
	}

	// Exit with error if any tests failed
	if report.Failed > 0 {
		os.Exit(1)
	}
}

func testContract(r *gin.Engine, method, endpoint string, body interface{}, expectCode int) ContractResult {
	return testContractWithHeaders(r, method, endpoint, body, nil, expectCode)
}

func testContractWithHeaders(r *gin.Engine, method, endpoint string, body interface{}, headers map[string]string, expectCode int) ContractResult {
	result := ContractResult{
		Endpoint: endpoint,
		Method:   method,
		Passed:   false,
	}

	// Create request
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to marshal body: %v", err)
			return result
		}
	}

	req, err := http.NewRequest(method, endpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		return result
	}

	// Add headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute request
	w := &responseRecorder{}
	r.ServeHTTP(w, req)

	// Check status code
	if w.Code != expectCode {
		result.Error = fmt.Sprintf("Expected status %d, got %d", expectCode, w.Code)
		return result
	}

	result.Passed = true
	return result
}

type responseRecorder struct {
	Code int
	Body []byte
}

func (r *responseRecorder) Header() http.Header {
	return http.Header{}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.Body = append(r.Body, b...)
	return len(b), nil
}

func (r *responseRecorder) WriteHeader(code int) {
	r.Code = code
}

func generateReport(results []ContractResult) ContractReport {
	report := ContractReport{
		Timestamp:  time.Now(),
		TotalTests: len(results),
		Results:    results,
	}

	for _, result := range results {
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}

	if report.TotalTests > 0 {
		report.SuccessRate = float64(report.Passed) / float64(report.TotalTests) * 100
	}

	return report
}

func printReport(report ContractReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("                 CONTRACT TEST REPORT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Timestamp: %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total Tests: %d\n", report.TotalTests)
	fmt.Printf("Passed: %d\n", report.Passed)
	fmt.Printf("Failed: %d\n", report.Failed)
	fmt.Printf("Success Rate: %.1f%%\n", report.SuccessRate)
	fmt.Println(strings.Repeat("-", 60))

	for _, result := range report.Results {
		status := "✅ PASS"
		if !result.Passed {
			status = "❌ FAIL"
		}

		fmt.Printf("%s %s %s\n", status, result.Method, result.Endpoint)
		fmt.Printf("   %s\n", result.Description)

		if result.Error != "" {
			fmt.Printf("   Error: %s\n", result.Error)
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 60))

	if report.Failed > 0 {
		fmt.Printf("\n⚠️  %d contract(s) failed\n", report.Failed)
	} else {
		fmt.Println("\n✅ All contracts passed!")
	}
}

func saveReport(report ContractReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile("contract-test-report.json", data, 0644)
}
