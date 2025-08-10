package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// OpenAPISpec represents a simplified OpenAPI specification
type OpenAPISpec struct {
	OpenAPI string                 `yaml:"openapi"`
	Info    map[string]interface{} `yaml:"info"`
	Paths   map[string]PathItem    `yaml:"paths"`
}

// PathItem represents an OpenAPI path item
type PathItem map[string]Operation

// Operation represents an OpenAPI operation
type Operation struct {
	OperationID string                 `yaml:"operationId"`
	Responses   map[string]Response    `yaml:"responses"`
	RequestBody *RequestBody          `yaml:"requestBody,omitempty"`
}

// Response represents an OpenAPI response
type Response struct {
	Description string               `yaml:"description"`
	Content     map[string]MediaType `yaml:"content,omitempty"`
}

// RequestBody represents an OpenAPI request body
type RequestBody struct {
	Required bool                 `yaml:"required"`
	Content  map[string]MediaType `yaml:"content"`
}

// MediaType represents an OpenAPI media type
type MediaType struct {
	Schema Schema `yaml:"schema"`
}

// Schema represents a simplified OpenAPI schema
type Schema struct {
	Type       string            `yaml:"type"`
	Properties map[string]Schema `yaml:"properties,omitempty"`
	Required   []string          `yaml:"required,omitempty"`
	Items      *Schema           `yaml:"items,omitempty"`
	Ref        string            `yaml:"$ref,omitempty"`
}

// OpenAPIValidator provides OpenAPI contract validation
type OpenAPIValidator struct {
	spec *OpenAPISpec
}

// NewOpenAPIValidator creates a new OpenAPI validator from the spec file
func NewOpenAPIValidator(specPath string) (*OpenAPIValidator, error) {
	// Read the OpenAPI spec file
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	// Parse YAML
	var spec OpenAPISpec
	if err := yaml.Unmarshal(specData, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	return &OpenAPIValidator{spec: &spec}, nil
}

// ValidateResponse validates that a response matches the OpenAPI spec
func (v *OpenAPIValidator) ValidateResponse() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Store the original writer
		originalWriter := c.Writer

		// Create a custom response writer to capture the response
		responseCapture := &responseWriter{
			ResponseWriter: originalWriter,
			body:          make([]byte, 0),
		}
		c.Writer = responseCapture

		// Process the request
		c.Next()

		// Validate the response after processing
		if err := v.validateResponse(c.Request.Method, c.Request.URL.Path, responseCapture.statusCode, responseCapture.body); err != nil {
			// Log the validation error (don't fail the request in production)
			fmt.Printf("OpenAPI validation warning for %s %s: %v\n", c.Request.Method, c.Request.URL.Path, err)
		}
	})
}

// responseWriter captures response data for validation
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
	body       []byte
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// validateResponse checks if the response matches the OpenAPI specification
func (v *OpenAPIValidator) validateResponse(method, path string, statusCode int, body []byte) error {
	// Find the path in the spec (simplified - doesn't handle path parameters)
	pathItem, exists := v.spec.Paths[path]
	if !exists {
		return fmt.Errorf("path %s not found in OpenAPI spec", path)
	}

	// Find the operation for this method
	operation, exists := pathItem[method]
	if !exists {
		return fmt.Errorf("method %s not found for path %s", method, path)
	}

	// Find the response for this status code
	statusStr := fmt.Sprintf("%d", statusCode)
	response, exists := operation.Responses[statusStr]
	if !exists {
		// Try default response
		if defaultResp, hasDefault := operation.Responses["default"]; hasDefault {
			response = defaultResp
		} else {
			return fmt.Errorf("status code %d not defined for %s %s", statusCode, method, path)
		}
	}

	// If there's no content defined, skip body validation
	if len(response.Content) == 0 {
		return nil
	}

	// Check if response has JSON content
	jsonContent, hasJSON := response.Content["application/json"]
	if !hasJSON {
		return nil // Skip validation if no JSON expected
	}

	// Basic JSON structure validation
	if len(body) > 0 {
		var responseData interface{}
		if err := json.Unmarshal(body, &responseData); err != nil {
			return fmt.Errorf("response is not valid JSON: %w", err)
		}

		// Additional schema validation could be added here
		// For now, we just validate that it's valid JSON matching the expected structure
		if err := v.validateJSONSchema(responseData, jsonContent.Schema); err != nil {
			return fmt.Errorf("response schema validation failed: %w", err)
		}
	}

	return nil
}

// validateJSONSchema performs basic schema validation
func (v *OpenAPIValidator) validateJSONSchema(data interface{}, schema Schema) error {
	switch schema.Type {
	case "object":
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected object, got %T", data)
		}

		// Check required fields
		for _, required := range schema.Required {
			if _, exists := dataMap[required]; !exists {
				return fmt.Errorf("required field '%s' is missing", required)
			}
		}

		// Validate properties (basic validation)
		for propName, propSchema := range schema.Properties {
			if value, exists := dataMap[propName]; exists {
				if err := v.validateJSONSchema(value, propSchema); err != nil {
					return fmt.Errorf("property '%s': %w", propName, err)
				}
			}
		}

	case "array":
		dataArray, ok := data.([]interface{})
		if !ok {
			return fmt.Errorf("expected array, got %T", data)
		}

		// Validate array items if schema is provided
		if schema.Items != nil {
			for i, item := range dataArray {
				if err := v.validateJSONSchema(item, *schema.Items); err != nil {
					return fmt.Errorf("array item %d: %w", i, err)
				}
			}
		}

	case "string":
		if _, ok := data.(string); !ok {
			return fmt.Errorf("expected string, got %T", data)
		}

	case "number":
		switch data.(type) {
		case float64, int, int64:
			// Valid number types
		default:
			return fmt.Errorf("expected number, got %T", data)
		}

	case "boolean":
		if _, ok := data.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", data)
		}
	}

	return nil
}

// LoadOpenAPIMiddleware creates the OpenAPI validation middleware
func LoadOpenAPIMiddleware() gin.HandlerFunc {
	// Find the OpenAPI spec file
	specPath := filepath.Join("api", "openapi.yaml")
	if _, err := os.Stat(specPath); err != nil {
		// If file doesn't exist, return a no-op middleware
		fmt.Printf("OpenAPI spec not found at %s, skipping validation\n", specPath)
		return gin.HandlerFunc(func(c *gin.Context) {
			c.Next()
		})
	}

	validator, err := NewOpenAPIValidator(specPath)
	if err != nil {
		fmt.Printf("Failed to load OpenAPI validator: %v\n", err)
		return gin.HandlerFunc(func(c *gin.Context) {
			c.Next()
		})
	}

	fmt.Println("OpenAPI validation middleware loaded")
	return validator.ValidateResponse()
}