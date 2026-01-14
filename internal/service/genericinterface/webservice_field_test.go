//go:build integration

package genericinterface

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// Note: getTestDB is defined in service_test.go

// TestWebserviceFieldService_Search tests the autocomplete search functionality.
func TestWebserviceFieldService_Search(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	// Create mock customer API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/customers/search":
			term := r.URL.Query().Get("SearchTerms")
			limit := r.URL.Query().Get("Limit")
			response := map[string]interface{}{
				"items": []map[string]interface{}{
					{"CustomerID": "C001", "Name": "Acme Corp matching " + term, "Email": "acme@example.com"},
					{"CustomerID": "C002", "Name": "Beta Inc matching " + term, "Email": "beta@example.com"},
					{"CustomerID": "C003", "Name": "Gamma Ltd matching " + term, "Email": "gamma@example.com"},
				},
				"total": 3,
				"limit": limit,
			}
			json.NewEncoder(w).Encode(response)

		case "/api/customers/C001":
			response := map[string]interface{}{
				"CustomerID": "C001",
				"Name":       "Acme Corporation",
				"Email":      "acme@example.com",
				"Phone":      "+1-555-0101",
			}
			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Create GI service and webservice config
	giService := NewService(testDB.DB)
	ctx := context.Background()

	wsConfig := &models.WebserviceConfig{
		Name:    "CustomerAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Name:         "Customer API",
			RemoteSystem: "CRM",
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host:           mockServer.URL,
						DefaultCommand: "GET",
						InvokerControllerMapping: map[string]models.ControllerMapping{
							"CustomerSearch": {
								Controller: "/api/customers/search",
								Command:    "GET",
							},
							"CustomerGet": {
								Controller: "/api/customers/:CustomerID",
								Command:    "GET",
							},
						},
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"CustomerSearch": {
						Type:        "Customer::Search",
						Description: "Search customers",
					},
					"CustomerGet": {
						Type:        "Customer::Get",
						Description: "Get customer by ID",
					},
				},
			},
		},
	}

	wsID, err := giService.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer giService.DeleteWebservice(ctx, wsID)

	// Create field service
	fieldService := NewWebserviceFieldServiceWithGI(giService)

	fieldConfig := FieldConfig{
		Webservice:               wsConfig.Name,
		InvokerSearch:            "CustomerSearch",
		InvokerGet:               "CustomerGet",
		StoredValue:              "CustomerID",
		DisplayedValues:          []string{"Name", "Email"},
		DisplayedValuesSeparator: " - ",
		AutocompleteMinLength:    2,
		Limit:                    10,
		CacheTTL:                 60,
	}

	t.Run("Search returns results", func(t *testing.T) {
		results, err := fieldService.Search(ctx, fieldConfig, "Acme")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}

		// Check first result - OTRS-compatible fields
		if results[0].StoredValue != "C001" {
			t.Errorf("Expected StoredValue 'C001', got %q", results[0].StoredValue)
		}
		if results[0].DisplayValue != "Acme Corp matching Acme - acme@example.com" {
			t.Errorf("Unexpected DisplayValue: %q", results[0].DisplayValue)
		}

		// Verify legacy aliases are also populated (for frontend compatibility)
		if results[0].Value != "C001" {
			t.Errorf("Expected Value (legacy) 'C001', got %q", results[0].Value)
		}
		if results[0].Label != "Acme Corp matching Acme - acme@example.com" {
			t.Errorf("Unexpected Label (legacy): %q", results[0].Label)
		}

		// Check that Data contains original response fields
		if results[0].Data["Email"] != "acme@example.com" {
			t.Errorf("Expected Data to contain Email field")
		}
	})

	t.Run("Search respects minimum length", func(t *testing.T) {
		results, err := fieldService.Search(ctx, fieldConfig, "A")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for short search term, got %d", len(results))
		}
	})

	t.Run("Search caches results", func(t *testing.T) {
		// First search
		results1, err := fieldService.Search(ctx, fieldConfig, "CacheTest")
		if err != nil {
			t.Fatalf("First search failed: %v", err)
		}

		// Second search - should use cache
		results2, err := fieldService.Search(ctx, fieldConfig, "CacheTest")
		if err != nil {
			t.Fatalf("Second search failed: %v", err)
		}

		if len(results1) != len(results2) {
			t.Error("Cache returned different results")
		}
	})

	t.Run("ClearCache works", func(t *testing.T) {
		// Populate cache
		_, err := fieldService.Search(ctx, fieldConfig, "ClearTest")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Clear cache for this webservice
		fieldService.ClearCache(wsConfig.Name)

		// Should not find in cache now (will fetch fresh)
		results, err := fieldService.Search(ctx, fieldConfig, "ClearTest")
		if err != nil {
			t.Fatalf("Search after clear failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("Search after clear should still return results")
		}
	})
}

// TestWebserviceFieldService_GetDisplayValue tests display value retrieval.
func TestWebserviceFieldService_GetDisplayValue(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/products/P123" {
			response := map[string]interface{}{
				"ProductID": "P123",
				"Name":      "Widget Pro",
				"SKU":       "WGT-PRO-001",
				"Price":     99.99,
			}
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	giService := NewService(testDB.DB)
	ctx := context.Background()

	wsConfig := &models.WebserviceConfig{
		Name:    "ProductAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host: mockServer.URL,
						InvokerControllerMapping: map[string]models.ControllerMapping{
							"ProductGet": {
								Controller: "/api/products/:ProductID",
								Command:    "GET",
							},
						},
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"ProductGet": {Type: "Product::Get"},
				},
			},
		},
	}

	wsID, err := giService.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer giService.DeleteWebservice(ctx, wsID)

	fieldService := NewWebserviceFieldServiceWithGI(giService)

	fieldConfig := FieldConfig{
		Webservice:               wsConfig.Name,
		InvokerGet:               "ProductGet",
		StoredValue:              "ProductID",
		DisplayedValues:          []string{"Name", "SKU"},
		DisplayedValuesSeparator: " | ",
	}

	t.Run("GetDisplayValue returns formatted string", func(t *testing.T) {
		display, err := fieldService.GetDisplayValue(ctx, fieldConfig, "P123")
		if err != nil {
			t.Fatalf("GetDisplayValue failed: %v", err)
		}

		expected := "Widget Pro | WGT-PRO-001"
		if display != expected {
			t.Errorf("Expected display %q, got %q", expected, display)
		}
	})

	t.Run("GetDisplayValue returns stored value on empty", func(t *testing.T) {
		display, err := fieldService.GetDisplayValue(ctx, fieldConfig, "")
		if err != nil {
			t.Fatalf("GetDisplayValue failed: %v", err)
		}

		if display != "" {
			t.Errorf("Expected empty string, got %q", display)
		}
	})

	t.Run("GetDisplayValue fallback when no InvokerGet", func(t *testing.T) {
		configNoGet := FieldConfig{
			Webservice:      wsConfig.Name,
			InvokerGet:      "", // No get invoker
			StoredValue:     "ProductID",
			DisplayedValues: []string{"Name"},
		}

		display, err := fieldService.GetDisplayValue(ctx, configNoGet, "P999")
		if err != nil {
			t.Fatalf("GetDisplayValue failed: %v", err)
		}

		// Should return stored value as fallback
		if display != "P999" {
			t.Errorf("Expected fallback to stored value 'P999', got %q", display)
		}
	})
}

// TestWebserviceFieldService_GetMultipleDisplayValues tests multiselect display values.
func TestWebserviceFieldService_GetMultipleDisplayValues(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		responses := map[string]map[string]interface{}{
			"/api/tags/T1": {"TagID": "T1", "Name": "Priority", "Color": "red"},
			"/api/tags/T2": {"TagID": "T2", "Name": "Urgent", "Color": "orange"},
			"/api/tags/T3": {"TagID": "T3", "Name": "Review", "Color": "blue"},
		}

		if resp, ok := responses[r.URL.Path]; ok {
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	giService := NewService(testDB.DB)
	ctx := context.Background()

	wsConfig := &models.WebserviceConfig{
		Name:    "TagAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host: mockServer.URL,
						InvokerControllerMapping: map[string]models.ControllerMapping{
							"TagGet": {
								Controller: "/api/tags/:TagID",
								Command:    "GET",
							},
						},
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"TagGet": {Type: "Tag::Get"},
				},
			},
		},
	}

	wsID, err := giService.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer giService.DeleteWebservice(ctx, wsID)

	fieldService := NewWebserviceFieldServiceWithGI(giService)

	fieldConfig := FieldConfig{
		Webservice:               wsConfig.Name,
		InvokerGet:               "TagGet",
		StoredValue:              "TagID",
		DisplayedValues:          []string{"Name"},
		DisplayedValuesSeparator: "",
	}

	t.Run("GetMultipleDisplayValues returns all values", func(t *testing.T) {
		values, err := fieldService.GetMultipleDisplayValues(ctx, fieldConfig, []string{"T1", "T2", "T3"})
		if err != nil {
			t.Fatalf("GetMultipleDisplayValues failed: %v", err)
		}

		if len(values) != 3 {
			t.Errorf("Expected 3 values, got %d", len(values))
		}

		if values["T1"] != "Priority" {
			t.Errorf("Expected T1='Priority', got %q", values["T1"])
		}
		if values["T2"] != "Urgent" {
			t.Errorf("Expected T2='Urgent', got %q", values["T2"])
		}
		if values["T3"] != "Review" {
			t.Errorf("Expected T3='Review', got %q", values["T3"])
		}
	})

	t.Run("GetMultipleDisplayValues handles missing values", func(t *testing.T) {
		values, err := fieldService.GetMultipleDisplayValues(ctx, fieldConfig, []string{"T1", "MISSING"})
		if err != nil {
			t.Fatalf("GetMultipleDisplayValues failed: %v", err)
		}

		if values["T1"] != "Priority" {
			t.Errorf("Expected T1='Priority', got %q", values["T1"])
		}
		// Missing value should fall back to stored value
		if values["MISSING"] != "MISSING" {
			t.Errorf("Expected MISSING='MISSING' (fallback), got %q", values["MISSING"])
		}
	})
}

// TestWebserviceFieldService_DifferentResponseFormats tests parsing various response structures.
func TestWebserviceFieldService_DifferentResponseFormats(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	testCases := []struct {
		name     string
		response interface{}
		expected int
	}{
		{
			name: "items array",
			response: map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "1", "name": "Item 1"},
					{"id": "2", "name": "Item 2"},
				},
			},
			expected: 2,
		},
		{
			name: "Items array (capitalized)",
			response: map[string]interface{}{
				"Items": []map[string]interface{}{
					{"id": "1", "name": "Item 1"},
				},
			},
			expected: 1,
		},
		{
			name: "results array",
			response: map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": "1", "name": "Result 1"},
					{"id": "2", "name": "Result 2"},
					{"id": "3", "name": "Result 3"},
				},
			},
			expected: 3,
		},
		{
			name: "data array",
			response: map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "1", "name": "Data 1"},
				},
			},
			expected: 1,
		},
		{
			name: "single object",
			response: map[string]interface{}{
				"id":   "1",
				"name": "Single Item",
			},
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tc.response)
			}))
			defer mockServer.Close()

			giService := NewService(testDB.DB)
			ctx := context.Background()

			wsConfig := &models.WebserviceConfig{
				Name:    "FormatTestAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
				ValidID: 1,
				Config: &models.WebserviceConfigData{
					Requester: models.RequesterConfig{
						Transport: models.TransportConfig{
							Type: "HTTP::REST",
							Config: models.TransportHTTPConfig{
								Host: mockServer.URL,
								InvokerControllerMapping: map[string]models.ControllerMapping{
									"Search": {Controller: "/search", Command: "GET"},
								},
							},
						},
						Invoker: map[string]models.InvokerConfig{
							"Search": {Type: "Test::Search"},
						},
					},
				},
			}

			wsID, err := giService.CreateWebservice(ctx, wsConfig, 1)
			if err != nil {
				t.Fatalf("Failed to create webservice: %v", err)
			}
			defer giService.DeleteWebservice(ctx, wsID)

			fieldService := NewWebserviceFieldServiceWithGI(giService)
			fieldConfig := FieldConfig{
				Webservice:            wsConfig.Name,
				InvokerSearch:         "Search",
				StoredValue:           "id",
				DisplayedValues:       []string{"name"},
				AutocompleteMinLength: 1,
			}

			results, err := fieldService.Search(ctx, fieldConfig, "test")
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) != tc.expected {
				t.Errorf("Expected %d results, got %d", tc.expected, len(results))
			}
		})
	}
}

// TestParseFieldConfigFromMap tests configuration parsing from dynamic field config.
func TestParseFieldConfigFromMap(t *testing.T) {
	t.Run("Full configuration", func(t *testing.T) {
		configMap := map[string]interface{}{
			"Webservice":               "CustomerAPI",
			"InvokerSearch":            "Search",
			"InvokerGet":               "Get",
			"StoredValue":              "ID",
			"DisplayedValues":          "Name, Email, Phone",
			"DisplayedValuesSeparator": " | ",
			"SearchKeys":               "Name, Email",
			"AutocompleteMinLength":    5,
			"Limit":                    25,
			"CacheTTL":                 120,
		}

		config := ParseFieldConfigFromMap(configMap)

		if config.Webservice != "CustomerAPI" {
			t.Errorf("Expected Webservice='CustomerAPI', got %q", config.Webservice)
		}
		if config.InvokerSearch != "Search" {
			t.Errorf("Expected InvokerSearch='Search', got %q", config.InvokerSearch)
		}
		if config.InvokerGet != "Get" {
			t.Errorf("Expected InvokerGet='Get', got %q", config.InvokerGet)
		}
		if config.StoredValue != "ID" {
			t.Errorf("Expected StoredValue='ID', got %q", config.StoredValue)
		}
		if len(config.DisplayedValues) != 3 {
			t.Errorf("Expected 3 DisplayedValues, got %d", len(config.DisplayedValues))
		}
		if config.DisplayedValues[0] != "Name" {
			t.Errorf("Expected first DisplayedValue='Name', got %q", config.DisplayedValues[0])
		}
		if config.DisplayedValuesSeparator != " | " {
			t.Errorf("Expected separator=' | ', got %q", config.DisplayedValuesSeparator)
		}
		if len(config.SearchKeys) != 2 {
			t.Errorf("Expected 2 SearchKeys, got %d", len(config.SearchKeys))
		}
		if config.AutocompleteMinLength != 5 {
			t.Errorf("Expected AutocompleteMinLength=5, got %d", config.AutocompleteMinLength)
		}
		if config.Limit != 25 {
			t.Errorf("Expected Limit=25, got %d", config.Limit)
		}
		if config.CacheTTL != 120 {
			t.Errorf("Expected CacheTTL=120, got %d", config.CacheTTL)
		}
	})

	t.Run("Default values", func(t *testing.T) {
		configMap := map[string]interface{}{}

		config := ParseFieldConfigFromMap(configMap)

		if config.AutocompleteMinLength != 3 {
			t.Errorf("Expected default AutocompleteMinLength=3, got %d", config.AutocompleteMinLength)
		}
		if config.Limit != 20 {
			t.Errorf("Expected default Limit=20, got %d", config.Limit)
		}
		if config.CacheTTL != 60 {
			t.Errorf("Expected default CacheTTL=60, got %d", config.CacheTTL)
		}
		if config.DisplayedValuesSeparator != " - " {
			t.Errorf("Expected default separator=' - ', got %q", config.DisplayedValuesSeparator)
		}
	})
}

// TestWebserviceFieldService_ErrorHandling tests error scenarios.
func TestWebserviceFieldService_ErrorHandling(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	giService := NewService(testDB.DB)
	fieldService := NewWebserviceFieldServiceWithGI(giService)
	ctx := context.Background()

	t.Run("Search with non-existent webservice", func(t *testing.T) {
		config := FieldConfig{
			Webservice:            "NonExistentWebservice",
			InvokerSearch:         "Search",
			AutocompleteMinLength: 1,
		}

		_, err := fieldService.Search(ctx, config, "test")
		if err == nil {
			t.Error("Expected error for non-existent webservice")
		}
	})

	t.Run("Search with server error", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer mockServer.Close()

		wsConfig := &models.WebserviceConfig{
			Name:    "ErrorAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			ValidID: 1,
			Config: &models.WebserviceConfigData{
				Requester: models.RequesterConfig{
					Transport: models.TransportConfig{
						Type: "HTTP::REST",
						Config: models.TransportHTTPConfig{
							Host: mockServer.URL,
							InvokerControllerMapping: map[string]models.ControllerMapping{
								"Search": {Controller: "/search", Command: "GET"},
							},
						},
					},
					Invoker: map[string]models.InvokerConfig{
						"Search": {Type: "Test::Search"},
					},
				},
			},
		}

		wsID, err := giService.CreateWebservice(ctx, wsConfig, 1)
		if err != nil {
			t.Fatalf("Failed to create webservice: %v", err)
		}
		defer giService.DeleteWebservice(ctx, wsID)

		config := FieldConfig{
			Webservice:            wsConfig.Name,
			InvokerSearch:         "Search",
			AutocompleteMinLength: 1,
		}

		_, err = fieldService.Search(ctx, config, "test")
		if err == nil {
			t.Error("Expected error for server error response")
		}
	})
}

// TestWebserviceFieldService_Limit tests result limiting.
func TestWebserviceFieldService_Limit(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	// Create mock that returns many results
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		items := make([]map[string]interface{}, 50)
		for i := 0; i < 50; i++ {
			items[i] = map[string]interface{}{
				"id":   fmt.Sprintf("ID%d", i),
				"name": fmt.Sprintf("Item %d", i),
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
	}))
	defer mockServer.Close()

	giService := NewService(testDB.DB)
	ctx := context.Background()

	wsConfig := &models.WebserviceConfig{
		Name:    "LimitAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host: mockServer.URL,
						InvokerControllerMapping: map[string]models.ControllerMapping{
							"Search": {Controller: "/search", Command: "GET"},
						},
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"Search": {Type: "Test::Search"},
				},
			},
		},
	}

	wsID, err := giService.CreateWebservice(ctx, wsConfig, 1)
	if err != nil {
		t.Fatalf("Failed to create webservice: %v", err)
	}
	defer giService.DeleteWebservice(ctx, wsID)

	fieldService := NewWebserviceFieldServiceWithGI(giService)

	config := FieldConfig{
		Webservice:            wsConfig.Name,
		InvokerSearch:         "Search",
		StoredValue:           "id",
		DisplayedValues:       []string{"name"},
		AutocompleteMinLength: 1,
		Limit:                 10,
	}

	results, err := fieldService.Search(ctx, config, "test")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 10 {
		t.Errorf("Expected 10 results (limit), got %d", len(results))
	}
}
