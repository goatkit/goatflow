package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

// Contract defines an API contract to be tested.
type Contract struct {
	Name        string
	Description string
	Method      string
	Path        string
	Headers     map[string]string
	Body        interface{}
	Expected    Response
}

// Response defines expected response characteristics.
type Response struct {
	Status      int
	Headers     map[string]string
	BodySchema  interface{} // Expected response structure
	Validations []Validation
}

// Validation is a custom validation function.
type Validation func(body []byte) error

// ContractTest runs a contract test against a handler.
type ContractTest struct {
	t         *testing.T
	contracts []Contract
	router    *gin.Engine
}

// NewContractTest creates a new contract test runner.
func NewContractTest(t *testing.T, router *gin.Engine) *ContractTest {
	return &ContractTest{
		t:      t,
		router: router,
	}
}

// AddContract adds a contract to test.
func (ct *ContractTest) AddContract(contract Contract) {
	ct.contracts = append(ct.contracts, contract)
}

// Run executes all contract tests.
func (ct *ContractTest) Run() {
	for _, contract := range ct.contracts {
		ct.t.Run(contract.Name, func(t *testing.T) {
			ct.runContract(t, contract)
		})
	}
}

// runContract executes a single contract test.
func (ct *ContractTest) runContract(t *testing.T, contract Contract) {
	// Prepare request body
	var bodyReader io.Reader
	if contract.Body != nil {
		body, err := json.Marshal(contract.Body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	// Create request
	req := httptest.NewRequest(contract.Method, contract.Path, bodyReader)
	for key, value := range contract.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	w := httptest.NewRecorder()
	ct.router.ServeHTTP(w, req)

	// Validate status code
	if w.Code != contract.Expected.Status {
		t.Errorf("Status code mismatch: expected %d, got %d", contract.Expected.Status, w.Code)
	}

	// Validate headers
	for key, expectedValue := range contract.Expected.Headers {
		actualValue := w.Header().Get(key)
		if actualValue != expectedValue {
			t.Errorf("Header %s mismatch: expected %s, got %s", key, expectedValue, actualValue)
		}
	}

	// Validate response body schema
	if contract.Expected.BodySchema != nil {
		var responseBody interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
			return
		}

		if err := ValidateSchema(responseBody, contract.Expected.BodySchema); err != nil {
			t.Errorf("Schema validation failed: %v", err)
		}
	}

	// Run custom validations
	for _, validation := range contract.Expected.Validations {
		if err := validation(w.Body.Bytes()); err != nil {
			t.Errorf("Custom validation failed: %v", err)
		}
	}
}

// ValidateSchema validates that a response matches expected schema.
func ValidateSchema(actual interface{}, expected interface{}) error {
	// Handle different types
	switch exp := expected.(type) {
	case Schema:
		return exp.Validate(actual)
	case map[string]interface{}:
		return validateMap(actual, exp)
	default:
		return validateBasicType(actual, expected)
	}
}

// validateMap validates map structures.
func validateMap(actual interface{}, expected map[string]interface{}) error {
	actualMap, ok := actual.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map, got %T", actual)
	}

	for key, expectedValue := range expected {
		actualValue, exists := actualMap[key]
		if !exists {
			return fmt.Errorf("missing required field: %s", key)
		}

		if err := ValidateSchema(actualValue, expectedValue); err != nil {
			return fmt.Errorf("field %s: %v", key, err)
		}
	}

	return nil
}

// validateBasicType validates basic types.
func validateBasicType(actual, expected interface{}) error {
	expType := reflect.TypeOf(expected)
	actType := reflect.TypeOf(actual)

	if expType != actType {
		return fmt.Errorf("type mismatch: expected %v, got %v", expType, actType)
	}

	return nil
}

// Schema interface for custom schema validation.
type Schema interface {
	Validate(interface{}) error
}

// StringSchema validates string fields.
type StringSchema struct {
	Required  bool
	MinLength int
	MaxLength int
	Pattern   string
}

func (s StringSchema) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}

	if s.Required && str == "" {
		return fmt.Errorf("required field is empty")
	}

	if s.MinLength > 0 && len(str) < s.MinLength {
		return fmt.Errorf("string too short: min %d, got %d", s.MinLength, len(str))
	}

	if s.MaxLength > 0 && len(str) > s.MaxLength {
		return fmt.Errorf("string too long: max %d, got %d", s.MaxLength, len(str))
	}

	return nil
}

// NumberSchema validates numeric fields.
type NumberSchema struct {
	Required bool
	Min      *float64
	Max      *float64
}

func (s NumberSchema) Validate(value interface{}) error {
	num, ok := value.(float64)
	if !ok {
		// Try to convert int to float64
		if intVal, ok := value.(int); ok {
			num = float64(intVal)
		} else {
			return fmt.Errorf("expected number, got %T", value)
		}
	}

	if s.Min != nil && num < *s.Min {
		return fmt.Errorf("number too small: min %f, got %f", *s.Min, num)
	}

	if s.Max != nil && num > *s.Max {
		return fmt.Errorf("number too large: max %f, got %f", *s.Max, num)
	}

	return nil
}

// ArraySchema validates array fields.
type ArraySchema struct {
	Required    bool
	MinItems    int
	MaxItems    int
	ItemsSchema Schema
}

func (s ArraySchema) Validate(value interface{}) error {
	arr, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("expected array, got %T", value)
	}

	if s.Required && len(arr) == 0 {
		return fmt.Errorf("required array is empty")
	}

	if s.MinItems > 0 && len(arr) < s.MinItems {
		return fmt.Errorf("array too small: min %d items, got %d", s.MinItems, len(arr))
	}

	if s.MaxItems > 0 && len(arr) > s.MaxItems {
		return fmt.Errorf("array too large: max %d items, got %d", s.MaxItems, len(arr))
	}

	if s.ItemsSchema != nil {
		for i, item := range arr {
			if err := s.ItemsSchema.Validate(item); err != nil {
				return fmt.Errorf("item %d: %v", i, err)
			}
		}
	}

	return nil
}

// ObjectSchema validates object fields.
type ObjectSchema struct {
	Required   bool
	Properties map[string]Schema
}

// BooleanSchema validates boolean fields.
type BooleanSchema struct {
	Required bool
}

func (s BooleanSchema) Validate(value interface{}) error {
	_, ok := value.(bool)
	if !ok {
		return fmt.Errorf("expected boolean, got %T", value)
	}
	return nil
}

func (s ObjectSchema) Validate(value interface{}) error {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected object, got %T", value)
	}

	for key, schema := range s.Properties {
		val, exists := obj[key]
		if !exists {
			// Check if field is required
			if strSchema, ok := schema.(StringSchema); ok && strSchema.Required {
				return fmt.Errorf("missing required field: %s", key)
			}
			continue
		}

		if err := schema.Validate(val); err != nil {
			return fmt.Errorf("field %s: %v", key, err)
		}
	}

	return nil
}

// Helper function to check if response contains expected fields.
func HasFields(fields ...string) Validation {
	return func(body []byte) error {
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return err
		}

		for _, field := range fields {
			if _, ok := data[field]; !ok {
				return fmt.Errorf("missing field: %s", field)
			}
		}
		return nil
	}
}

// Helper function to validate error response format.
func IsErrorResponse() Validation {
	return func(body []byte) error {
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return err
		}

		// Check for either 'error' field or 'success: false'
		if _, hasError := data["error"]; hasError {
			return nil
		}

		if success, ok := data["success"].(bool); ok && !success {
			return nil
		}

		return fmt.Errorf("expected error response format")
	}
}

// Helper function to validate success response format.
func IsSuccessResponse() Validation {
	return func(body []byte) error {
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return err
		}

		if success, ok := data["success"].(bool); !ok || !success {
			return fmt.Errorf("expected success: true")
		}

		return nil
	}
}
