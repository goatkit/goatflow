//go:build integration

package genericinterface

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// testDB wraps a database connection for tests.
type testDB struct {
	DB *sql.DB
}

func (t *testDB) Close() error {
	return t.DB.Close()
}

// getTestDB returns a test database connection.
func getTestDB() (*testDB, error) {
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if driver == "" {
		driver = "postgres"
	}

	host := firstNonEmpty(os.Getenv("TEST_DB_HOST"), os.Getenv("DB_HOST"), "localhost")
	user := firstNonEmpty(os.Getenv("TEST_DB_USER"), os.Getenv("DB_USER"), "gotrs")
	password := firstNonEmpty(os.Getenv("TEST_DB_PASSWORD"), os.Getenv("DB_PASSWORD"), "gotrs")
	dbName := firstNonEmpty(os.Getenv("TEST_DB_NAME"), os.Getenv("DB_NAME"), "gotrs")
	port := firstNonEmpty(os.Getenv("TEST_DB_PORT"), os.Getenv("DB_PORT"), "5432")

	var db *sql.DB
	var err error

	switch driver {
	case "postgres", "pgsql", "pg":
		sslMode := firstNonEmpty(os.Getenv("TEST_DB_SSLMODE"), os.Getenv("DB_SSL_MODE"), "disable")
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbName, sslMode)
		db, err = sql.Open("postgres", connStr)
	case "mysql", "mariadb":
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC", user, password, host, port, dbName)
		db, err = sql.Open("mysql", connStr)
	default:
		return nil, fmt.Errorf("unsupported TEST_DB_DRIVER %q", driver)
	}

	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return &testDB{DB: db}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// TestRESTTransport_Execute tests the REST transport against a mock server.
func TestRESTTransport_Execute(t *testing.T) {
	// Create mock REST API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/search":
			// Search endpoint - expects GET with query params
			if r.Method != "GET" {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			term := r.URL.Query().Get("SearchTerms")
			response := map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "1", "name": "Result 1 for " + term, "code": "R1"},
					{"id": "2", "name": "Result 2 for " + term, "code": "R2"},
					{"id": "3", "name": "Result 3 for " + term, "code": "R3"},
				},
			}
			json.NewEncoder(w).Encode(response)

		case "/api/items/123":
			// Get single item endpoint
			if r.Method != "GET" {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			response := map[string]interface{}{
				"id":   "123",
				"name": "Test Item",
				"code": "TI123",
			}
			json.NewEncoder(w).Encode(response)

		case "/api/items":
			// Create item endpoint
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			response := map[string]interface{}{
				"id":      "999",
				"name":    body["name"],
				"created": true,
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)

		case "/api/auth-test":
			// Endpoint that requires basic auth
			user, pass, ok := r.BasicAuth()
			if !ok || user != "testuser" || pass != "testpass" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})

		case "/api/apikey-test":
			// Endpoint that requires API key
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "test-api-key-12345" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid api key"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer mockServer.Close()

	transport := NewRESTTransport()
	ctx := context.Background()

	t.Run("GET request with query params", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:           mockServer.URL,
			DefaultCommand: "GET",
		}
		request := &Request{
			Operation: "search",
			Path:      "/api/search",
			Data: map[string]interface{}{
				"SearchTerms": "test",
			},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
		if response.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", response.StatusCode)
		}

		items, ok := response.Data["items"].([]interface{})
		if !ok {
			t.Fatalf("Expected items array in response")
		}
		if len(items) != 3 {
			t.Errorf("Expected 3 items, got %d", len(items))
		}
	})

	t.Run("GET request with path parameter", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:           mockServer.URL,
			DefaultCommand: "GET",
		}
		request := &Request{
			Operation: "get",
			Path:      "/api/items/:id",
			Data: map[string]interface{}{
				"id": "123",
			},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
		if response.Data["id"] != "123" {
			t.Errorf("Expected id=123, got %v", response.Data["id"])
		}
		if response.Data["name"] != "Test Item" {
			t.Errorf("Expected name='Test Item', got %v", response.Data["name"])
		}
	})

	t.Run("POST request with JSON body", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:           mockServer.URL,
			DefaultCommand: "POST",
		}
		request := &Request{
			Operation: "create",
			Path:      "/api/items",
			Method:    "POST",
			Data: map[string]interface{}{
				"name": "New Item",
			},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
		if response.StatusCode != 201 {
			t.Errorf("Expected status 201, got %d", response.StatusCode)
		}
		if response.Data["created"] != true {
			t.Errorf("Expected created=true, got %v", response.Data["created"])
		}
	})

	t.Run("Basic authentication", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:           mockServer.URL,
			DefaultCommand: "GET",
			Authentication: models.AuthConfig{
				AuthType:          "BasicAuth",
				BasicAuthUser:     "testuser",
				BasicAuthPassword: "testpass",
			},
		}
		request := &Request{
			Operation: "authtest",
			Path:      "/api/auth-test",
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
		if response.Data["status"] != "authenticated" {
			t.Errorf("Expected status='authenticated', got %v", response.Data["status"])
		}
	})

	t.Run("Basic auth failure", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:           mockServer.URL,
			DefaultCommand: "GET",
			Authentication: models.AuthConfig{
				AuthType:          "BasicAuth",
				BasicAuthUser:     "wronguser",
				BasicAuthPassword: "wrongpass",
			},
		}
		request := &Request{
			Operation: "authtest",
			Path:      "/api/auth-test",
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if response.Success {
			t.Error("Expected failure for wrong credentials")
		}
		if response.StatusCode != 401 {
			t.Errorf("Expected status 401, got %d", response.StatusCode)
		}
	})

	t.Run("API key authentication", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:           mockServer.URL,
			DefaultCommand: "GET",
			Authentication: models.AuthConfig{
				AuthType: "APIKey",
				APIKey:   "test-api-key-12345",
			},
		}
		request := &Request{
			Operation: "apikeytest",
			Path:      "/api/apikey-test",
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
		if response.Data["status"] != "authenticated" {
			t.Errorf("Expected status='authenticated', got %v", response.Data["status"])
		}
	})

	t.Run("404 not found", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:           mockServer.URL,
			DefaultCommand: "GET",
		}
		request := &Request{
			Operation: "nonexistent",
			Path:      "/api/does-not-exist",
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if response.Success {
			t.Error("Expected failure for 404")
		}
		if response.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d", response.StatusCode)
		}
	})
}

// TestRESTTransport_InvokerControllerMapping tests controller mapping.
func TestRESTTransport_InvokerControllerMapping(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"path":   r.URL.Path,
			"method": r.Method,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	transport := NewRESTTransport()
	ctx := context.Background()

	config := models.TransportHTTPConfig{
		Host:           mockServer.URL,
		DefaultCommand: "GET",
		InvokerControllerMapping: map[string]models.ControllerMapping{
			"SearchItems": {
				Controller: "/api/items/search",
				Command:    "GET",
			},
			"CreateItem": {
				Controller: "/api/items",
				Command:    "POST",
			},
			"GetItem": {
				Controller: "/api/items/:id",
				Command:    "GET",
			},
		},
	}

	t.Run("Mapped GET endpoint", func(t *testing.T) {
		request := &Request{
			Operation: "SearchItems",
			Data:      map[string]interface{}{},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if response.Data["path"] != "/api/items/search" {
			t.Errorf("Expected path '/api/items/search', got %v", response.Data["path"])
		}
		if response.Data["method"] != "GET" {
			t.Errorf("Expected method 'GET', got %v", response.Data["method"])
		}
	})

	t.Run("Mapped POST endpoint", func(t *testing.T) {
		request := &Request{
			Operation: "CreateItem",
			Data:      map[string]interface{}{"name": "test"},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if response.Data["path"] != "/api/items" {
			t.Errorf("Expected path '/api/items', got %v", response.Data["path"])
		}
		if response.Data["method"] != "POST" {
			t.Errorf("Expected method 'POST', got %v", response.Data["method"])
		}
	})

	t.Run("Mapped endpoint with path param substitution", func(t *testing.T) {
		request := &Request{
			Operation: "GetItem",
			Data:      map[string]interface{}{"id": "456"},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if response.Data["path"] != "/api/items/456" {
			t.Errorf("Expected path '/api/items/456', got %v", response.Data["path"])
		}
	})
}

// TestRESTTransport_TestConnection tests connection testing functionality.
func TestRESTTransport_TestConnection(t *testing.T) {
	transport := NewRESTTransport()
	ctx := context.Background()

	t.Run("Successful connection", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer mockServer.Close()

		config := models.TransportHTTPConfig{
			Host: mockServer.URL,
		}

		err := transport.TestConnection(ctx, config)
		if err != nil {
			t.Errorf("TestConnection failed: %v", err)
		}
	})

	t.Run("Failed connection - unreachable", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host: "http://localhost:59999", // Unlikely to be in use
		}

		err := transport.TestConnection(ctx, config)
		if err == nil {
			t.Error("Expected error for unreachable host")
		}
	})
}

// TestGenericInterfaceService_Invoke tests the full invoke flow with database.
func TestGenericInterfaceService_Invoke(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	// Create mock REST API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/search":
			term := r.URL.Query().Get("SearchTerms")
			response := map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "1", "name": "Customer " + term, "email": "customer1@example.com"},
					{"id": "2", "name": "Vendor " + term, "email": "vendor1@example.com"},
				},
			}
			json.NewEncoder(w).Encode(response)

		case "/api/get/42":
			response := map[string]interface{}{
				"id":    "42",
				"name":  "Specific Customer",
				"email": "specific@example.com",
			}
			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Create GenericInterface service
	service := NewService(testDB.DB)
	ctx := context.Background()

	// Create test webservice configuration
	wsConfig := &models.WebserviceConfig{
		Name:    "TestCustomerAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Name:         "Test Customer API",
			Description:  "Test API for integration tests",
			RemoteSystem: "MockServer",
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host:           mockServer.URL,
						DefaultCommand: "GET",
						InvokerControllerMapping: map[string]models.ControllerMapping{
							"Search": {
								Controller: "/api/search",
								Command:    "GET",
							},
							"Get": {
								Controller: "/api/get/:id",
								Command:    "GET",
							},
						},
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"Search": {
						Type:        "Test::Search",
						Description: "Search customers",
					},
					"Get": {
						Type:        "Test::Get",
						Description: "Get customer by ID",
					},
				},
			},
		},
	}

	// Create webservice in database
	wsID, err := service.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer service.DeleteWebservice(ctx, wsID)

	t.Run("Invoke Search invoker", func(t *testing.T) {
		response, err := service.Invoke(ctx, wsConfig.Name, "Search", map[string]interface{}{
			"SearchTerms": "Acme",
		})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}

		items, ok := response.Data["items"].([]interface{})
		if !ok {
			t.Fatal("Expected items array")
		}
		if len(items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(items))
		}
	})

	t.Run("Invoke Get invoker", func(t *testing.T) {
		response, err := service.Invoke(ctx, wsConfig.Name, "Get", map[string]interface{}{
			"id": "42",
		})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
		if response.Data["id"] != "42" {
			t.Errorf("Expected id=42, got %v", response.Data["id"])
		}
		if response.Data["name"] != "Specific Customer" {
			t.Errorf("Expected name='Specific Customer', got %v", response.Data["name"])
		}
	})

	t.Run("Invoke non-existent invoker", func(t *testing.T) {
		_, err := service.Invoke(ctx, wsConfig.Name, "NonExistent", map[string]interface{}{})
		if err == nil {
			t.Error("Expected error for non-existent invoker")
		}
	})

	t.Run("Invoke on non-existent webservice", func(t *testing.T) {
		_, err := service.Invoke(ctx, "NonExistentWebservice", "Search", map[string]interface{}{})
		if err == nil {
			t.Error("Expected error for non-existent webservice")
		}
	})

	t.Run("Invoke on invalid webservice", func(t *testing.T) {
		// Create invalid webservice
		invalidWS := &models.WebserviceConfig{
			Name:    "InvalidWebservice_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			ValidID: 2, // Invalid
			Config:  &models.WebserviceConfigData{},
		}
		invalidID, err := service.CreateWebservice(ctx, invalidWS, 1)
		if err != nil {
			t.Fatalf("Failed to create invalid webservice: %v", err)
		}
		defer service.DeleteWebservice(ctx, invalidID)

		_, err = service.Invoke(ctx, invalidWS.Name, "Search", map[string]interface{}{})
		if err == nil {
			t.Error("Expected error for invalid webservice")
		}
	})
}

// TestGenericInterfaceService_InvokeWithController tests direct controller invocation.
func TestGenericInterfaceService_InvokeWithController(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	// Create mock REST API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"path":   r.URL.Path,
			"method": r.Method,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	service := NewService(testDB.DB)
	ctx := context.Background()

	wsConfig := &models.WebserviceConfig{
		Name:    "TestControllerAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host: mockServer.URL,
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"Generic": {Type: "Generic"},
				},
			},
		},
	}

	wsID, err := service.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer service.DeleteWebservice(ctx, wsID)

	t.Run("Custom controller and method", func(t *testing.T) {
		response, err := service.InvokeWithController(ctx, wsConfig.Name, "Generic", "/custom/path", "PUT", map[string]interface{}{
			"data": "test",
		})
		if err != nil {
			t.Fatalf("InvokeWithController failed: %v", err)
		}
		if response.Data["path"] != "/custom/path" {
			t.Errorf("Expected path '/custom/path', got %v", response.Data["path"])
		}
		if response.Data["method"] != "PUT" {
			t.Errorf("Expected method 'PUT', got %v", response.Data["method"])
		}
	})
}

// TestGenericInterfaceService_Caching tests webservice config caching.
func TestGenericInterfaceService_Caching(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer mockServer.Close()

	service := NewService(testDB.DB)
	ctx := context.Background()

	wsConfig := &models.WebserviceConfig{
		Name:    "TestCacheAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host: mockServer.URL,
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"Test": {Type: "Test"},
				},
			},
		},
	}

	wsID, err := service.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer service.DeleteWebservice(ctx, wsID)

	// First call populates cache
	ws1, err := service.GetWebservice(ctx, wsConfig.Name)
	if err != nil {
		t.Fatalf("GetWebservice failed: %v", err)
	}

	// Second call should use cache
	ws2, err := service.GetWebservice(ctx, wsConfig.Name)
	if err != nil {
		t.Fatalf("GetWebservice failed: %v", err)
	}

	if ws1.ID != ws2.ID {
		t.Error("Expected same webservice from cache")
	}

	// Get by ID should also be cached
	ws3, err := service.GetWebserviceByID(ctx, wsID)
	if err != nil {
		t.Fatalf("GetWebserviceByID failed: %v", err)
	}
	if ws3.Name != wsConfig.Name {
		t.Errorf("Expected name %q, got %q", wsConfig.Name, ws3.Name)
	}
}

// TestGenericInterfaceService_DataMapping tests data transformation.
func TestGenericInterfaceService_DataMapping(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	var receivedData map[string]interface{}
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewDecoder(r.Body).Decode(&receivedData)
		// Return response with different key names
		response := map[string]interface{}{
			"customer_id":   "123",
			"customer_name": "Test Customer",
			"customer_code": "TC001",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	service := NewService(testDB.DB)
	ctx := context.Background()

	wsConfig := &models.WebserviceConfig{
		Name:    "TestMappingAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host:           mockServer.URL,
						DefaultCommand: "POST",
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"CreateCustomer": {
						Type: "Customer::Create",
						MappingOutbound: models.MappingConfig{
							Type: "Simple",
							Config: map[string]interface{}{
								"KeyMapDefault": map[string]interface{}{
									"MapTo": "1",
								},
								"KeyMap": map[string]interface{}{
									"Name": "CustomerName",
									"Code": "CustomerCode",
								},
							},
						},
						MappingInbound: models.MappingConfig{
							Type: "Simple",
							Config: map[string]interface{}{
								"KeyMap": map[string]interface{}{
									"customer_id":   "ID",
									"customer_name": "Name",
									"customer_code": "Code",
								},
							},
						},
					},
				},
			},
		},
	}

	wsID, err := service.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer service.DeleteWebservice(ctx, wsID)

	t.Run("Outbound mapping transforms request data", func(t *testing.T) {
		_, err := service.Invoke(ctx, wsConfig.Name, "CreateCustomer", map[string]interface{}{
			"Name": "Acme Corp",
			"Code": "ACM001",
		})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}

		// Check that outbound mapping transformed the keys
		if receivedData["CustomerName"] != "Acme Corp" {
			t.Errorf("Expected CustomerName='Acme Corp', got %v", receivedData["CustomerName"])
		}
		if receivedData["CustomerCode"] != "ACM001" {
			t.Errorf("Expected CustomerCode='ACM001', got %v", receivedData["CustomerCode"])
		}
	})

	t.Run("Inbound mapping transforms response data", func(t *testing.T) {
		response, err := service.Invoke(ctx, wsConfig.Name, "CreateCustomer", map[string]interface{}{
			"Name": "Test",
		})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}

		// Check that inbound mapping transformed response keys
		if response.Data["ID"] != "123" {
			t.Errorf("Expected ID='123', got %v", response.Data["ID"])
		}
		if response.Data["Name"] != "Test Customer" {
			t.Errorf("Expected Name='Test Customer', got %v", response.Data["Name"])
		}
		if response.Data["Code"] != "TC001" {
			t.Errorf("Expected Code='TC001', got %v", response.Data["Code"])
		}
	})
}

// TestRESTTransport_AdditionalHeaders tests custom header support.
func TestRESTTransport_AdditionalHeaders(t *testing.T) {
	var receivedHeaders http.Header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer mockServer.Close()

	transport := NewRESTTransport()
	ctx := context.Background()

	config := models.TransportHTTPConfig{
		Host:           mockServer.URL,
		DefaultCommand: "GET",
		AdditionalHeaders: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Request-ID":    "12345",
		},
	}
	request := &Request{
		Operation: "test",
		Path:      "/api/test",
	}

	_, err := transport.Execute(ctx, config, request)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Expected X-Custom-Header='custom-value', got %q", receivedHeaders.Get("X-Custom-Header"))
	}
	if receivedHeaders.Get("X-Request-ID") != "12345" {
		t.Errorf("Expected X-Request-ID='12345', got %q", receivedHeaders.Get("X-Request-ID"))
	}
}

// TestRESTTransport_ExecuteRaw tests raw HTTP execution.
func TestRESTTransport_ExecuteRaw(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Response-Header", "response-value")
		json.NewEncoder(w).Encode(map[string]string{"raw": "response"})
	}))
	defer mockServer.Close()

	transport := NewRESTTransport()
	ctx := context.Background()

	response, err := transport.ExecuteRaw(ctx, "GET", mockServer.URL+"/test", map[string]string{
		"X-Test": "header",
	}, nil)
	if err != nil {
		t.Fatalf("ExecuteRaw failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected success")
	}
	if response.Data["raw"] != "response" {
		t.Errorf("Expected raw='response', got %v", response.Data["raw"])
	}
	if response.Headers["X-Response-Header"] != "response-value" {
		t.Errorf("Expected X-Response-Header='response-value', got %q", response.Headers["X-Response-Header"])
	}
}
