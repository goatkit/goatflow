package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("NewOpenAPIValidator with valid spec", func(t *testing.T) {
		// Create a temporary OpenAPI spec
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "openapi.yaml")
		
		specContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      operationId: getTest
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
                required:
                  - message
`
		err := os.WriteFile(specPath, []byte(specContent), 0644)
		require.NoError(t, err)

		validator, err := NewOpenAPIValidator(specPath)
		assert.NoError(t, err)
		assert.NotNil(t, validator)
		assert.NotNil(t, validator.spec)
	})

	t.Run("NewOpenAPIValidator with non-existent file", func(t *testing.T) {
		validator, err := NewOpenAPIValidator("/non/existent/openapi.yaml")
		assert.Error(t, err)
		assert.Nil(t, validator)
		assert.Contains(t, err.Error(), "failed to read OpenAPI spec")
	})

	t.Run("NewOpenAPIValidator with invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "invalid.yaml")
		
		invalidContent := `
openapi: 3.0.0
info: [invalid yaml structure
`
		err := os.WriteFile(specPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		validator, err := NewOpenAPIValidator(specPath)
		assert.Error(t, err)
		assert.Nil(t, validator)
		assert.Contains(t, err.Error(), "failed to parse OpenAPI spec")
	})
}

func TestValidateResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a temporary OpenAPI spec for testing
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	
	specContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /health:
    get:
      operationId: getHealth
      responses:
        '200':
          description: Health check response
          content:
            application/json:
              schema:
                type: object
                properties:
                  status:
                    type: string
                  service:
                    type: string
                required:
                  - status
  /api/users:
    get:
      operationId: getUsers
      responses:
        '200':
          description: List of users
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id:
                      type: number
                    name:
                      type: string
  /api/error:
    get:
      operationId: getError
      responses:
        '500':
          description: Error response
          content:
            application/json:
              schema:
                type: object
                properties:
                  error:
                    type: string
`
	err := os.WriteFile(specPath, []byte(specContent), 0644)
	require.NoError(t, err)

	validator, err := NewOpenAPIValidator(specPath)
	require.NoError(t, err)

	t.Run("Valid response matches schema", func(t *testing.T) {
		router := gin.New()
		router.Use(validator.ValidateResponse())
		
		router.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "healthy",
				"service": "test",
			})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
	})

	t.Run("Array response validation", func(t *testing.T) {
		router := gin.New()
		router.Use(validator.ValidateResponse())
		
		router.GET("/api/users", func(c *gin.Context) {
			c.JSON(200, []gin.H{
				{"id": 1, "name": "User 1"},
				{"id": 2, "name": "User 2"},
			})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/users", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		
		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response, 2)
	})

	t.Run("Error response validation", func(t *testing.T) {
		router := gin.New()
		router.Use(validator.ValidateResponse())
		
		router.GET("/api/error", func(c *gin.Context) {
			c.JSON(500, gin.H{
				"error": "Internal server error",
			})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/error", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "Internal server error", response["error"])
	})
}

func TestResponseWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("responseWriter captures body and status", func(t *testing.T) {
		// Create a gin context with recorder
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:          make([]byte, 0),
		}

		// Write status code
		rw.WriteHeader(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, rw.statusCode)

		// Write body
		testData := []byte(`{"test": "data"}`)
		n, err := rw.Write(testData)
		assert.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.Equal(t, testData, rw.body)
	})

	t.Run("responseWriter accumulates multiple writes", func(t *testing.T) {
		// Create a gin context with recorder
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:          make([]byte, 0),
		}

		// Multiple writes
		part1 := []byte(`{"part1":`)
		part2 := []byte(`"data"}`)
		
		n1, err1 := rw.Write(part1)
		assert.NoError(t, err1)
		assert.Equal(t, len(part1), n1)
		
		n2, err2 := rw.Write(part2)
		assert.NoError(t, err2)
		assert.Equal(t, len(part2), n2)
		
		expectedBody := append(part1, part2...)
		assert.Equal(t, expectedBody, rw.body)
	})
}

func TestValidateJSONSchema(t *testing.T) {
	validator := &OpenAPIValidator{}

	t.Run("Object schema validation", func(t *testing.T) {
		schema := Schema{
			Type: "object",
			Properties: map[string]Schema{
				"name": {Type: "string"},
				"age":  {Type: "number"},
			},
			Required: []string{"name"},
		}

		// Valid object
		validData := map[string]interface{}{
			"name": "John",
			"age":  30.0,
		}
		err := validator.validateJSONSchema(validData, schema)
		assert.NoError(t, err)

		// Missing required field
		invalidData := map[string]interface{}{
			"age": 30.0,
		}
		err = validator.validateJSONSchema(invalidData, schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required field 'name' is missing")

		// Wrong type
		wrongTypeData := "not an object"
		err = validator.validateJSONSchema(wrongTypeData, schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected object")
	})

	t.Run("Array schema validation", func(t *testing.T) {
		itemSchema := Schema{
			Type: "object",
			Properties: map[string]Schema{
				"id": {Type: "number"},
			},
		}
		schema := Schema{
			Type:  "array",
			Items: &itemSchema,
		}

		// Valid array
		validData := []interface{}{
			map[string]interface{}{"id": 1.0},
			map[string]interface{}{"id": 2.0},
		}
		err := validator.validateJSONSchema(validData, schema)
		assert.NoError(t, err)

		// Wrong type
		wrongTypeData := "not an array"
		err = validator.validateJSONSchema(wrongTypeData, schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected array")

		// Invalid item in array
		invalidItemData := []interface{}{
			map[string]interface{}{"id": "not a number"},
		}
		err = validator.validateJSONSchema(invalidItemData, schema)
		assert.Error(t, err)
	})

	t.Run("String schema validation", func(t *testing.T) {
		schema := Schema{Type: "string"}

		// Valid string
		err := validator.validateJSONSchema("test string", schema)
		assert.NoError(t, err)

		// Wrong type
		err = validator.validateJSONSchema(123, schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected string")
	})

	t.Run("Number schema validation", func(t *testing.T) {
		schema := Schema{Type: "number"}

		// Valid numbers
		assert.NoError(t, validator.validateJSONSchema(42.0, schema))
		assert.NoError(t, validator.validateJSONSchema(42, schema))
		assert.NoError(t, validator.validateJSONSchema(int64(42), schema))

		// Wrong type
		err := validator.validateJSONSchema("not a number", schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected number")
	})

	t.Run("Boolean schema validation", func(t *testing.T) {
		schema := Schema{Type: "boolean"}

		// Valid boolean
		assert.NoError(t, validator.validateJSONSchema(true, schema))
		assert.NoError(t, validator.validateJSONSchema(false, schema))

		// Wrong type
		err := validator.validateJSONSchema("true", schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected boolean")
	})

	t.Run("Nested object validation", func(t *testing.T) {
		schema := Schema{
			Type: "object",
			Properties: map[string]Schema{
				"user": {
					Type: "object",
					Properties: map[string]Schema{
						"name":  {Type: "string"},
						"email": {Type: "string"},
					},
					Required: []string{"name"},
				},
			},
		}

		// Valid nested object
		validData := map[string]interface{}{
			"user": map[string]interface{}{
				"name":  "John",
				"email": "john@example.com",
			},
		}
		err := validator.validateJSONSchema(validData, schema)
		assert.NoError(t, err)

		// Invalid nested object (missing required field)
		invalidData := map[string]interface{}{
			"user": map[string]interface{}{
				"email": "john@example.com",
			},
		}
		err = validator.validateJSONSchema(invalidData, schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required field 'name' is missing")
	})
}

func TestLoadOpenAPIMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns no-op middleware when spec file doesn't exist", func(t *testing.T) {
		// Ensure the spec file doesn't exist
		specPath := filepath.Join("api", "openapi.yaml")
		os.Remove(specPath)

		middleware := LoadOpenAPIMiddleware()
		assert.NotNil(t, middleware)

		// Test that middleware doesn't break the request
		router := gin.New()
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("Loads validator when spec file exists", func(t *testing.T) {
		// Create api directory if it doesn't exist
		apiDir := "api"
		os.MkdirAll(apiDir, 0755)
		defer os.RemoveAll(apiDir)

		// Create a valid spec file
		specPath := filepath.Join(apiDir, "openapi.yaml")
		specContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
`
		err := os.WriteFile(specPath, []byte(specContent), 0644)
		require.NoError(t, err)

		middleware := LoadOpenAPIMiddleware()
		assert.NotNil(t, middleware)

		// Test that middleware works
		router := gin.New()
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("Returns no-op middleware on invalid spec", func(t *testing.T) {
		// Create api directory if it doesn't exist
		apiDir := "api"
		os.MkdirAll(apiDir, 0755)
		defer os.RemoveAll(apiDir)

		// Create an invalid spec file
		specPath := filepath.Join(apiDir, "openapi.yaml")
		invalidContent := `invalid: [yaml content`
		err := os.WriteFile(specPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		middleware := LoadOpenAPIMiddleware()
		assert.NotNil(t, middleware)

		// Test that middleware still allows requests
		router := gin.New()
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
	})
}

func TestValidateResponseIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test OpenAPI spec
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	
	specContent := `
openapi: 3.0.0
info:
  title: Integration Test API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: User found
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: number
                  name:
                    type: string
                  email:
                    type: string
                required:
                  - id
                  - name
        '404':
          description: User not found
        default:
          description: Error response
          content:
            application/json:
              schema:
                type: object
                properties:
                  error:
                    type: string
`
	err := os.WriteFile(specPath, []byte(specContent), 0644)
	require.NoError(t, err)

	validator, err := NewOpenAPIValidator(specPath)
	require.NoError(t, err)

	t.Run("Validates parameterized path", func(t *testing.T) {
		// Note: This is a simplified test. In real implementation,
		// you would need to handle path parameters properly
		router := gin.New()
		router.Use(validator.ValidateResponse())
		
		router.GET("/users/:id", func(c *gin.Context) {
			id := c.Param("id")
			if id == "1" {
				c.JSON(200, gin.H{
					"id":    1,
					"name":  "John Doe",
					"email": "john@example.com",
				})
			} else {
				c.Status(404)
			}
		})

		// Test successful response
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/users/1", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		// Test 404 response
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/users/999", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 404, w.Code)
	})
}

func TestValidateResponseMethod(t *testing.T) {
	validator := &OpenAPIValidator{
		spec: &OpenAPISpec{
			Paths: map[string]PathItem{
				"/test": {
					"get": Operation{
						Responses: map[string]Response{
							"200": {
								Description: "Success",
								Content: map[string]MediaType{
									"application/json": {
										Schema: Schema{
											Type: "object",
											Properties: map[string]Schema{
												"message": {Type: "string"},
											},
											Required: []string{"message"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("Valid response passes validation", func(t *testing.T) {
		body := []byte(`{"message": "test"}`)
		err := validator.validateResponse("get", "/test", 200, body)
		assert.NoError(t, err)
	})

	t.Run("Missing required field fails validation", func(t *testing.T) {
		body := []byte(`{"other": "field"}`)
		err := validator.validateResponse("get", "/test", 200, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required field 'message' is missing")
	})

	t.Run("Invalid JSON fails validation", func(t *testing.T) {
		body := []byte(`{invalid json}`)
		err := validator.validateResponse("get", "/test", 200, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})

	t.Run("Path not found in spec", func(t *testing.T) {
		body := []byte(`{"message": "test"}`)
		err := validator.validateResponse("get", "/unknown", 200, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path /unknown not found")
	})

	t.Run("Method not found for path", func(t *testing.T) {
		body := []byte(`{"message": "test"}`)
		err := validator.validateResponse("post", "/test", 200, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "method post not found")
	})

	t.Run("Status code not defined uses default", func(t *testing.T) {
		validator.spec.Paths["/test"]["get"].Responses["default"] = Response{
			Description: "Default response",
			Content: map[string]MediaType{
				"application/json": {
					Schema: Schema{Type: "object"},
				},
			},
		}

		body := []byte(`{}`)
		err := validator.validateResponse("get", "/test", 404, body)
		assert.NoError(t, err)
	})

	t.Run("No content defined skips validation", func(t *testing.T) {
		validator.spec.Paths["/test"]["get"].Responses["204"] = Response{
			Description: "No content",
		}

		err := validator.validateResponse("get", "/test", 204, nil)
		assert.NoError(t, err)
	})
}

func BenchmarkValidateResponse(b *testing.B) {
	gin.SetMode(gin.TestMode)

	// Create a simple validator
	validator := &OpenAPIValidator{
		spec: &OpenAPISpec{
			Paths: map[string]PathItem{
				"/test": {
					"get": Operation{
						Responses: map[string]Response{
							"200": {
								Description: "Success",
								Content: map[string]MediaType{
									"application/json": {
										Schema: Schema{
											Type: "object",
											Properties: map[string]Schema{
												"message": {Type: "string"},
												"status":  {Type: "string"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	router := gin.New()
	router.Use(validator.ValidateResponse())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "test",
			"status":  "ok",
		})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkValidateJSONSchema(b *testing.B) {
	validator := &OpenAPIValidator{}
	schema := Schema{
		Type: "object",
		Properties: map[string]Schema{
			"id":     {Type: "number"},
			"name":   {Type: "string"},
			"active": {Type: "boolean"},
		},
		Required: []string{"id", "name"},
	}

	data := map[string]interface{}{
		"id":     1.0,
		"name":   "Test",
		"active": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.validateJSONSchema(data, schema)
	}
}

func TestResponseCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Middleware captures response for validation", func(t *testing.T) {
		// Create a simple validator
		validator := &OpenAPIValidator{
			spec: &OpenAPISpec{
				Paths: map[string]PathItem{
					"/capture": {
						"get": Operation{
							Responses: map[string]Response{
								"200": {
									Description: "Success",
									Content: map[string]MediaType{
										"application/json": {
											Schema: Schema{
												Type: "object",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		router := gin.New()
		router.Use(validator.ValidateResponse())
		
		var capturedBody []byte
		router.GET("/capture", func(c *gin.Context) {
			responseData := gin.H{"captured": true, "timestamp": "2024-01-01"}
			capturedBody, _ = json.Marshal(responseData)
			c.JSON(200, responseData)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/capture", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		
		// Verify the response body was properly written
		assert.JSONEq(t, string(capturedBody), w.Body.String())
	})

	t.Run("Large response handling", func(t *testing.T) {
		validator := &OpenAPIValidator{
			spec: &OpenAPISpec{
				Paths: map[string]PathItem{
					"/large": {
						"get": Operation{
							Responses: map[string]Response{
								"200": {
									Description: "Large response",
									Content: map[string]MediaType{
										"application/json": {
											Schema: Schema{
												Type: "array",
												Items: &Schema{
													Type: "object",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		router := gin.New()
		router.Use(validator.ValidateResponse())
		
		router.GET("/large", func(c *gin.Context) {
			// Create a large response
			items := make([]gin.H, 1000)
			for i := range items {
				items[i] = gin.H{"id": i, "data": "test data string"}
			}
			c.JSON(200, items)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/large", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		
		// Verify response is valid JSON
		var response []interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response, 1000)
	})
}

func TestConcurrentValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator := &OpenAPIValidator{
		spec: &OpenAPISpec{
			Paths: map[string]PathItem{
				"/concurrent": {
					"get": Operation{
						Responses: map[string]Response{
							"200": {
								Description: "Success",
								Content: map[string]MediaType{
									"application/json": {
										Schema: Schema{
											Type: "object",
											Properties: map[string]Schema{
												"id": {Type: "number"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	router := gin.New()
	router.Use(validator.ValidateResponse())
	
	var counter int32
	router.GET("/concurrent", func(c *gin.Context) {
		newValue := atomic.AddInt32(&counter, 1)
		c.JSON(200, gin.H{"id": newValue})
	})

	// Run concurrent requests
	numRequests := 100
	done := make(chan bool, numRequests)
	
	for i := 0; i < numRequests; i++ {
		go func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/concurrent", nil)
			router.ServeHTTP(w, req)
			
			assert.Equal(t, 200, w.Code)
			done <- true
		}()
	}
	
	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
	
	assert.Equal(t, int32(numRequests), atomic.LoadInt32(&counter))
}

func TestEmptyResponseHandling(t *testing.T) {
	validator := &OpenAPIValidator{
		spec: &OpenAPISpec{
			Paths: map[string]PathItem{
				"/empty": {
					"delete": Operation{
						Responses: map[string]Response{
							"204": {
								Description: "No Content",
							},
						},
					},
				},
			},
		},
	}

	t.Run("Empty response with no content definition", func(t *testing.T) {
		err := validator.validateResponse("delete", "/empty", 204, nil)
		assert.NoError(t, err)
	})

	t.Run("Empty response with empty body", func(t *testing.T) {
		err := validator.validateResponse("delete", "/empty", 204, []byte{})
		assert.NoError(t, err)
	})
}

func TestStreamingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator := &OpenAPIValidator{
		spec: &OpenAPISpec{
			Paths: map[string]PathItem{
				"/stream": {
					"get": Operation{
						Responses: map[string]Response{
							"200": {
								Description: "Stream response",
								Content: map[string]MediaType{
									"text/plain": {
										Schema: Schema{
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	router := gin.New()
	router.Use(validator.ValidateResponse())
	
	router.GET("/stream", func(c *gin.Context) {
		c.String(200, "This is a plain text response")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/stream", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "This is a plain text response", w.Body.String())
}

func TestMalformedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator := &OpenAPIValidator{
		spec: &OpenAPISpec{
			Paths: map[string]PathItem{},
		},
	}

	router := gin.New()
	router.Use(validator.ValidateResponse())
	
	router.POST("/malformed", func(c *gin.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
			c.JSON(400, gin.H{"error": "Invalid JSON"})
			return
		}
		c.JSON(200, data)
	})

	// Send malformed JSON
	malformedJSON := `{"key": invalid}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/malformed", bytes.NewReader([]byte(malformedJSON)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid JSON", response["error"])
}

func TestHeaderValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test that the middleware doesn't interfere with header handling
	validator := &OpenAPIValidator{
		spec: &OpenAPISpec{
			Paths: map[string]PathItem{
				"/headers": {
					"get": Operation{
						Responses: map[string]Response{
							"200": {
								Description: "Success with headers",
								Content: map[string]MediaType{
									"application/json": {
										Schema: Schema{
											Type: "object",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	router := gin.New()
	router.Use(validator.ValidateResponse())
	
	router.GET("/headers", func(c *gin.Context) {
		c.Header("X-Custom-Header", "test-value")
		c.Header("X-Request-ID", "123456")
		c.JSON(200, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/headers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "test-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "123456", w.Header().Get("X-Request-ID"))
}

func TestPartialWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a gin context with recorder
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	rw := &responseWriter{
		ResponseWriter: c.Writer,
		body:          make([]byte, 0),
	}

	// Test partial writes that simulate streaming
	chunks := []string{
		`{"stream":`,
		`"chunk1",`,
		`"data":`,
		`"chunk2"}`,
	}

	for _, chunk := range chunks {
		n, err := rw.Write([]byte(chunk))
		assert.NoError(t, err)
		assert.Equal(t, len(chunk), n)
	}

	expectedBody := `{"stream":"chunk1","data":"chunk2"}`
	assert.Equal(t, expectedBody, string(rw.body))
}