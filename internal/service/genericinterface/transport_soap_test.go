//go:build integration

package genericinterface

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

// mockSOAPServer creates a test SOAP server that handles various operations.
func mockSOAPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify SOAP request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/xml") {
			t.Errorf("Expected Content-Type text/xml, got %s", contentType)
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Determine operation from SOAPAction header or body
		soapAction := r.Header.Get("SOAPAction")

		w.Header().Set("Content-Type", "text/xml; charset=utf-8")

		// Route based on SOAPAction
		switch {
		case strings.Contains(soapAction, "SearchCustomers") || strings.Contains(string(body), "SearchCustomers"):
			handleSearchCustomers(w, body)
		case strings.Contains(soapAction, "GetCustomer") || strings.Contains(string(body), "GetCustomer"):
			handleGetCustomer(w, body)
		case strings.Contains(soapAction, "CreateOrder") || strings.Contains(string(body), "CreateOrder"):
			handleCreateOrder(w, body)
		case strings.Contains(soapAction, "FaultTest") || strings.Contains(string(body), "FaultTest"):
			handleFaultTest(w)
		case strings.Contains(soapAction, "AuthRequired") || strings.Contains(string(body), "AuthRequired"):
			handleAuthRequired(w, r)
		default:
			handleUnknownOperation(w)
		}
	}))
}

func handleSearchCustomers(w http.ResponseWriter, body []byte) {
	// Extract search term from request
	searchTerm := extractXMLElement(string(body), "SearchTerms")

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <SearchCustomersResponse xmlns="http://example.com/customers">
      <Customers>
        <Customer>
          <ID>C001</ID>
          <Name>Acme Corp %s</Name>
          <Email>acme@example.com</Email>
          <Country>USA</Country>
        </Customer>
        <Customer>
          <ID>C002</ID>
          <Name>Beta Inc %s</Name>
          <Email>beta@example.com</Email>
          <Country>Germany</Country>
        </Customer>
        <Customer>
          <ID>C003</ID>
          <Name>Gamma Ltd %s</Name>
          <Email>gamma@example.com</Email>
          <Country>UK</Country>
        </Customer>
      </Customers>
      <TotalCount>3</TotalCount>
    </SearchCustomersResponse>
  </soap:Body>
</soap:Envelope>`, searchTerm, searchTerm, searchTerm)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func handleGetCustomer(w http.ResponseWriter, body []byte) {
	customerID := extractXMLElement(string(body), "CustomerID")
	if customerID == "" {
		customerID = extractXMLElement(string(body), "ID")
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetCustomerResponse xmlns="http://example.com/customers">
      <Customer>
        <ID>%s</ID>
        <Name>Customer %s Details</Name>
        <Email>customer%s@example.com</Email>
        <Phone>+1-555-0100</Phone>
        <Address>123 Main St</Address>
        <Country>USA</Country>
        <Active>true</Active>
      </Customer>
    </GetCustomerResponse>
  </soap:Body>
</soap:Envelope>`, customerID, customerID, customerID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func handleCreateOrder(w http.ResponseWriter, body []byte) {
	response := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <CreateOrderResponse xmlns="http://example.com/orders">
      <OrderID>ORD-12345</OrderID>
      <Status>Created</Status>
      <Timestamp>2026-01-14T12:00:00Z</Timestamp>
    </CreateOrderResponse>
  </soap:Body>
</soap:Envelope>`

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func handleFaultTest(w http.ResponseWriter) {
	response := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Server</faultcode>
      <faultstring>Internal service error: Database connection failed</faultstring>
      <detail>Connection to database server timed out after 30 seconds</detail>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`

	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(response))
}

func handleAuthRequired(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	if !ok || user != "soapuser" || pass != "soappass" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Client</faultcode>
      <faultstring>Authentication required</faultstring>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`))
		return
	}

	response := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <AuthRequiredResponse xmlns="http://example.com/auth">
      <Status>Authenticated</Status>
      <User>soapuser</User>
    </AuthRequiredResponse>
  </soap:Body>
</soap:Envelope>`

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func handleUnknownOperation(w http.ResponseWriter) {
	response := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Client</faultcode>
      <faultstring>Unknown operation</faultstring>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`

	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(response))
}

// extractXMLElement extracts the text content of an XML element by name.
func extractXMLElement(xmlStr, elementName string) string {
	startTag := "<" + elementName + ">"
	endTag := "</" + elementName + ">"

	startIdx := strings.Index(xmlStr, startTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startTag)

	endIdx := strings.Index(xmlStr[startIdx:], endTag)
	if endIdx == -1 {
		return ""
	}

	return strings.TrimSpace(xmlStr[startIdx : startIdx+endIdx])
}

// TestSOAPTransport_Execute tests the SOAP transport against a mock server.
func TestSOAPTransport_Execute(t *testing.T) {
	mockServer := mockSOAPServer(t)
	defer mockServer.Close()

	transport := NewSOAPTransport()
	ctx := context.Background()

	t.Run("Search operation", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/customers",
		}
		request := &Request{
			Operation: "SearchCustomers",
			Data: map[string]interface{}{
				"SearchTerms": "Acme",
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

		// Verify response contains customer data
		if response.Data == nil {
			t.Fatal("Expected data in response")
		}
	})

	t.Run("Get single record", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/customers",
		}
		request := &Request{
			Operation: "GetCustomer",
			Data: map[string]interface{}{
				"CustomerID": "C001",
			},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
	})

	t.Run("Create operation (POST with data)", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/orders",
		}
		request := &Request{
			Operation: "CreateOrder",
			Data: map[string]interface{}{
				"CustomerID": "C001",
				"ProductID":  "P100",
				"Quantity":   5,
			},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
	})

	t.Run("SOAP Fault handling", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/test",
		}
		request := &Request{
			Operation: "FaultTest",
			Data:      map[string]interface{}{},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if response.Success {
			t.Error("Expected failure for fault response")
		}
		if !strings.Contains(response.Error, "SOAP Fault") {
			t.Errorf("Expected SOAP Fault error, got: %s", response.Error)
		}
	})

	t.Run("Basic authentication", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/auth",
			Authentication: models.AuthConfig{
				AuthType:          "BasicAuth",
				BasicAuthUser:     "soapuser",
				BasicAuthPassword: "soappass",
			},
		}
		request := &Request{
			Operation: "AuthRequired",
			Data:      map[string]interface{}{},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
	})

	t.Run("Auth failure", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/auth",
			Authentication: models.AuthConfig{
				AuthType:          "BasicAuth",
				BasicAuthUser:     "wronguser",
				BasicAuthPassword: "wrongpass",
			},
		}
		request := &Request{
			Operation: "AuthRequired",
			Data:      map[string]interface{}{},
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

	t.Run("Unknown operation", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/unknown",
		}
		request := &Request{
			Operation: "UnknownOperation",
			Data:      map[string]interface{}{},
		}

		response, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if response.Success {
			t.Error("Expected failure for unknown operation")
		}
	})
}

// TestSOAPTransport_SOAPAction tests SOAPAction header handling.
func TestSOAPTransport_SOAPAction(t *testing.T) {
	var receivedSOAPAction string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSOAPAction = r.Header.Get("SOAPAction")
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Response><Status>OK</Status></Response></soap:Body></soap:Envelope>`))
	}))
	defer mockServer.Close()

	transport := NewSOAPTransport()
	ctx := context.Background()

	t.Run("Explicit SOAPAction", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:       mockServer.URL,
			SOAPAction: "http://example.com/MyAction",
		}
		request := &Request{
			Operation: "TestOp",
			Data:      map[string]interface{}{},
		}

		_, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if receivedSOAPAction != "http://example.com/MyAction" {
			t.Errorf("Expected SOAPAction 'http://example.com/MyAction', got %q", receivedSOAPAction)
		}
	})

	t.Run("SOAPAction from namespace + operation", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host:      mockServer.URL,
			NameSpace: "http://example.com/service/",
		}
		request := &Request{
			Operation: "DoSomething",
			Data:      map[string]interface{}{},
		}

		_, err := transport.Execute(ctx, config, request)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if receivedSOAPAction != "http://example.com/service/DoSomething" {
			t.Errorf("Expected SOAPAction 'http://example.com/service/DoSomething', got %q", receivedSOAPAction)
		}
	})
}

// TestSOAPTransport_BuildSOAPRequest tests SOAP envelope construction.
func TestSOAPTransport_BuildSOAPRequest(t *testing.T) {
	transport := NewSOAPTransport()

	config := models.TransportHTTPConfig{
		Host:      "http://example.com",
		NameSpace: "http://example.com/customers",
	}
	request := &Request{
		Operation: "SearchCustomers",
		Data: map[string]interface{}{
			"SearchTerms": "Test & <Special>",
			"Limit":       10,
		},
	}

	xmlData, err := transport.BuildSOAPRequest(config, request)
	if err != nil {
		t.Fatalf("BuildSOAPRequest failed: %v", err)
	}

	xmlStr := string(xmlData)

	// Verify envelope structure
	if !strings.Contains(xmlStr, "soap:Envelope") {
		t.Error("Missing soap:Envelope")
	}
	if !strings.Contains(xmlStr, "soap:Body") {
		t.Error("Missing soap:Body")
	}
	if !strings.Contains(xmlStr, "SearchCustomers") {
		t.Error("Missing operation element")
	}
	if !strings.Contains(xmlStr, `xmlns="http://example.com/customers"`) {
		t.Error("Missing namespace")
	}

	// Verify XML escaping
	if !strings.Contains(xmlStr, "Test &amp; &lt;Special&gt;") {
		t.Errorf("Special characters not escaped properly in: %s", xmlStr)
	}
}

// TestSOAPTransport_TestConnection tests connection testing.
func TestSOAPTransport_TestConnection(t *testing.T) {
	transport := NewSOAPTransport()
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

	t.Run("Failed connection", func(t *testing.T) {
		config := models.TransportHTTPConfig{
			Host: "http://localhost:59998", // Unlikely to be in use
		}

		err := transport.TestConnection(ctx, config)
		if err == nil {
			t.Error("Expected error for unreachable host")
		}
	})

	t.Run("With endpoint path", func(t *testing.T) {
		var receivedPath string
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer mockServer.Close()

		config := models.TransportHTTPConfig{
			Host:     mockServer.URL,
			Endpoint: "/soap/service",
		}

		err := transport.TestConnection(ctx, config)
		if err != nil {
			t.Errorf("TestConnection failed: %v", err)
		}
		if receivedPath != "/soap/service" {
			t.Errorf("Expected path '/soap/service', got %q", receivedPath)
		}
	})
}

// TestGenericInterfaceService_InvokeSOAP tests full SOAP invocation through the service.
func TestGenericInterfaceService_InvokeSOAP(t *testing.T) {
	testDB, err := getTestDB()
	if err != nil {
		t.Skipf("Skipping test - no test database available: %v", err)
	}
	defer testDB.Close()

	// Create mock SOAP server
	mockServer := mockSOAPServer(t)
	defer mockServer.Close()

	// Create GenericInterface service
	service := NewService(testDB.DB)
	ctx := context.Background()

	// Create SOAP webservice configuration
	wsConfig := &models.WebserviceConfig{
		Name:    "TestSOAPCustomerAPI_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Name:         "SOAP Customer API",
			Description:  "Test SOAP API for integration tests",
			RemoteSystem: "MockSOAPServer",
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::SOAP",
					Config: models.TransportHTTPConfig{
						Host:      mockServer.URL,
						NameSpace: "http://example.com/customers",
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"SearchCustomers": {
						Type:        "Customer::Search",
						Description: "Search customers via SOAP",
					},
					"GetCustomer": {
						Type:        "Customer::Get",
						Description: "Get customer by ID via SOAP",
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

	t.Run("Invoke SOAP Search", func(t *testing.T) {
		response, err := service.Invoke(ctx, wsConfig.Name, "SearchCustomers", map[string]interface{}{
			"SearchTerms": "Acme",
		})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
	})

	t.Run("Invoke SOAP Get", func(t *testing.T) {
		response, err := service.Invoke(ctx, wsConfig.Name, "GetCustomer", map[string]interface{}{
			"CustomerID": "C001",
		})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}
		if !response.Success {
			t.Errorf("Expected success, got error: %s", response.Error)
		}
	})
}

// TestSOAPEnvelopeParsing tests parsing of various SOAP response formats.
func TestSOAPEnvelopeParsing(t *testing.T) {
	transport := NewSOAPTransport()

	testCases := []struct {
		name        string
		response    string
		expectFault bool
		expectData  bool
	}{
		{
			name: "Simple response",
			response: `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <Response>
      <ID>123</ID>
      <Name>Test</Name>
    </Response>
  </soap:Body>
</soap:Envelope>`,
			expectFault: false,
			expectData:  true,
		},
		{
			name: "SOAP Fault",
			response: `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Server</faultcode>
      <faultstring>Server error</faultstring>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`,
			expectFault: true,
			expectData:  false,
		},
		{
			name: "Namespaced response",
			response: `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <ns:GetResponse xmlns:ns="http://example.com/">
      <ns:Result>Success</ns:Result>
    </ns:GetResponse>
  </soap:Body>
</soap:Envelope>`,
			expectFault: false,
			expectData:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, fault, err := transport.parseSOAPResponse([]byte(tc.response), models.TransportHTTPConfig{})
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if tc.expectFault && fault == nil {
				t.Error("Expected fault, got nil")
			}
			if !tc.expectFault && fault != nil {
				t.Errorf("Unexpected fault: %v", fault)
			}
			if tc.expectData && data == nil {
				t.Error("Expected data, got nil")
			}
		})
	}
}
