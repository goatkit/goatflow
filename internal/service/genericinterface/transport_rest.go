package genericinterface

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

// RESTTransport implements the Transport interface for HTTP REST APIs.
type RESTTransport struct {
	client *http.Client
}

// NewRESTTransport creates a new REST transport.
func NewRESTTransport() *RESTTransport {
	return &RESTTransport{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Type returns the transport type.
func (t *RESTTransport) Type() string {
	return "HTTP::REST"
}

// Execute performs an HTTP request to the remote REST API.
func (t *RESTTransport) Execute(ctx context.Context, config models.TransportHTTPConfig, request *Request) (*Response, error) {
	// Build URL
	baseURL := strings.TrimSuffix(config.Host, "/")
	path := t.buildPath(config, request)
	fullURL := baseURL + path

	// Determine HTTP method
	method := t.determineMethod(config, request)

	// Build request body
	var body io.Reader
	var bodyBytes []byte
	if method == "POST" || method == "PUT" || method == "PATCH" {
		var err error
		bodyBytes, err = json.Marshal(request.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(bodyBytes)
	} else if len(request.Data) > 0 {
		// For GET/DELETE, add data as query parameters
		u, err := url.Parse(fullURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL: %w", err)
		}
		q := u.Query()
		for k, v := range request.Data {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add additional headers from config
	for k, v := range config.AdditionalHeaders {
		req.Header.Set(k, v)
	}

	// Apply authentication
	if err := t.applyAuth(req, config.Authentication); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	// Set timeout from config if specified
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout + "s")
		if err == nil {
			t.client.Timeout = timeout
		}
	}

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Build response
	response := &Response{
		StatusCode: resp.StatusCode,
		RawData:    respBody,
		Headers:    make(map[string]string),
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
	}

	// Copy response headers
	for k := range resp.Header {
		response.Headers[k] = resp.Header.Get(k)
	}

	// Parse JSON response if possible
	if len(respBody) > 0 {
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var data map[string]interface{}
			if err := json.Unmarshal(respBody, &data); err == nil {
				response.Data = data
			} else {
				// Try parsing as array
				var arrayData []interface{}
				if err := json.Unmarshal(respBody, &arrayData); err == nil {
					response.Data = map[string]interface{}{"items": arrayData}
				}
			}
		}
	}

	if !response.Success {
		response.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return response, nil
}

// buildPath constructs the request path.
func (t *RESTTransport) buildPath(config models.TransportHTTPConfig, request *Request) string {
	// If request has explicit path, use it
	if request.Path != "" {
		return t.substitutePlaceholders(request.Path, request.Data)
	}

	// Look up controller mapping for the operation
	if config.InvokerControllerMapping != nil {
		if mapping, ok := config.InvokerControllerMapping[request.Operation]; ok {
			return t.substitutePlaceholders(mapping.Controller, request.Data)
		}
	}

	// Default to operation name as path
	return "/" + request.Operation
}

// substitutePlaceholders replaces :param placeholders in the path with actual values.
func (t *RESTTransport) substitutePlaceholders(path string, data map[string]interface{}) string {
	// Match :paramName patterns
	re := regexp.MustCompile(`:(\w+)`)
	result := re.ReplaceAllStringFunc(path, func(match string) string {
		paramName := strings.TrimPrefix(match, ":")
		if val, ok := data[paramName]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match
	})
	return result
}

// determineMethod determines the HTTP method to use.
func (t *RESTTransport) determineMethod(config models.TransportHTTPConfig, request *Request) string {
	// If request has explicit method, use it
	if request.Method != "" {
		return strings.ToUpper(request.Method)
	}

	// Look up method from controller mapping
	if config.InvokerControllerMapping != nil {
		if mapping, ok := config.InvokerControllerMapping[request.Operation]; ok {
			if mapping.Command != "" {
				return strings.ToUpper(mapping.Command)
			}
		}
	}

	// Use default command from config
	if config.DefaultCommand != "" {
		return strings.ToUpper(config.DefaultCommand)
	}

	// Default to GET
	return "GET"
}

// applyAuth applies authentication to the request.
func (t *RESTTransport) applyAuth(req *http.Request, auth models.AuthConfig) error {
	switch auth.AuthType {
	case "BasicAuth":
		if auth.BasicAuthUser != "" {
			credentials := base64.StdEncoding.EncodeToString(
				[]byte(auth.BasicAuthUser + ":" + auth.BasicAuthPassword),
			)
			req.Header.Set("Authorization", "Basic "+credentials)
		}

	case "APIKey":
		if auth.APIKey != "" {
			header := auth.APIKeyHeader
			if header == "" {
				header = "X-API-Key"
			}
			req.Header.Set(header, auth.APIKey)
		}

	case "Bearer":
		// OAuth2 Bearer token (already obtained)
		if auth.OAuth2ClientID != "" {
			// For OAuth2, the token should be pre-fetched and stored
			// This is a simplified implementation
		}

	case "OAuth2":
		// OAuth2 client credentials flow would be implemented here
		// For now, skip - full OAuth2 support is a larger feature

	case "JWT":
		// JWT authentication would be implemented here
		// Requires JWT library and key management

	case "":
		// No authentication
	}

	return nil
}

// ExecuteRaw performs a raw HTTP request with minimal processing.
// Useful for testing and debugging.
func (t *RESTTransport) ExecuteRaw(ctx context.Context, method, url string, headers map[string]string, body []byte) (*Response, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	response := &Response{
		StatusCode: resp.StatusCode,
		RawData:    respBody,
		Headers:    make(map[string]string),
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
	}

	for k := range resp.Header {
		response.Headers[k] = resp.Header.Get(k)
	}

	// Try to parse JSON
	if len(respBody) > 0 {
		var data map[string]interface{}
		if err := json.Unmarshal(respBody, &data); err == nil {
			response.Data = data
		}
	}

	return response, nil
}

// TestConnection tests connectivity to the remote host.
func (t *RESTTransport) TestConnection(ctx context.Context, config models.TransportHTTPConfig) error {
	// Build a simple HEAD request to the base URL
	req, err := http.NewRequestWithContext(ctx, "HEAD", config.Host, nil)
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	// Apply authentication
	if err := t.applyAuth(req, config.Authentication); err != nil {
		return fmt.Errorf("failed to apply authentication: %w", err)
	}

	// Use a short timeout for connection test
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	// Any response (even 404) means we connected successfully
	return nil
}
