package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/goatkit/goatflow/sdk/go/auth"
	"github.com/goatkit/goatflow/sdk/go/errors"
	"github.com/goatkit/goatflow/sdk/go/types"
)

// Client represents the GoatFlow API client
type Client struct {
	httpClient *resty.Client
	baseURL    string
	auth       auth.Authenticator
	userAgent  string
	timeout    time.Duration

	// Service clients
	Tickets   *TicketsService
	Users     *UsersService
	Queues    *QueuesService
	Dashboard *DashboardService
	LDAP      *LDAPService
	Webhooks  *WebhooksService
	Notes     *NotesService
	Auth      *AuthService
}

// Config represents client configuration
type Config struct {
	BaseURL    string
	Auth       auth.Authenticator
	UserAgent  string
	Timeout    time.Duration
	RetryCount int
	Debug      bool
}

// NewClient creates a new GoatFlow API client
func NewClient(config *Config) *Client {
	if config.UserAgent == "" {
		config.UserAgent = "goatflow-go-sdk/1.0.0"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}

	httpClient := resty.New().
		SetBaseURL(config.BaseURL).
		SetTimeout(config.Timeout).
		SetRetryCount(config.RetryCount).
		SetHeader("User-Agent", config.UserAgent).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json")

	if config.Debug {
		httpClient.SetDebug(true)
	}

	client := &Client{
		httpClient: httpClient,
		baseURL:    config.BaseURL,
		auth:       config.Auth,
		userAgent:  config.UserAgent,
		timeout:    config.Timeout,
	}

	// Initialize service clients
	client.Tickets = &TicketsService{client: client}
	client.Users = &UsersService{client: client}
	client.Queues = &QueuesService{client: client}
	client.Dashboard = &DashboardService{client: client}
	client.LDAP = &LDAPService{client: client}
	client.Webhooks = &WebhooksService{client: client}
	client.Notes = &NotesService{client: client}
	client.Auth = &AuthService{client: client}

	// Set up authentication middleware
	httpClient.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		return client.setAuth(req)
	})

	// Set up error handling middleware
	httpClient.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		return client.handleError(resp)
	})

	return client
}

// NewClientWithAPIKey creates a new client with API key authentication
func NewClientWithAPIKey(baseURL, apiKey string) *Client {
	return NewClient(&Config{
		BaseURL: baseURL,
		Auth:    auth.NewAPIKeyAuth(apiKey),
	})
}

// NewClientWithJWT creates a new client with JWT authentication
func NewClientWithJWT(baseURL, token, refreshToken string, expiresAt time.Time) *Client {
	return NewClient(&Config{
		BaseURL: baseURL,
		Auth:    auth.NewJWTAuth(token, refreshToken, expiresAt),
	})
}

// setAuth sets authentication headers on requests
func (c *Client) setAuth(req *resty.Request) error {
	if c.auth == nil {
		return nil
	}

	// Check if token is expired and refresh if possible
	if c.auth.IsExpired() {
		if err := c.auth.Refresh(); err != nil {
			return fmt.Errorf("failed to refresh authentication: %w", err)
		}
	}

	authHeader := c.auth.GetAuthHeader()
	if authHeader != "" {
		switch c.auth.Type() {
		case auth.AuthMethodAPIKey:
			req.SetHeader("X-API-Key", authHeader)
		case auth.AuthMethodJWT, auth.AuthMethodOAuth2:
			req.SetHeader("Authorization", authHeader)
		}
	}

	return nil
}

// handleError processes API error responses
func (c *Client) handleError(resp *resty.Response) error {
	if resp.IsSuccess() {
		return nil
	}

	// Try to parse error response
	var apiResp types.APIResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err == nil {
		if !apiResp.Success && apiResp.Error != "" {
			return errors.NewAPIError(resp.StatusCode(), apiResp.Error, "", apiResp.Message)
		}
	}

	// Try to parse error details
	var errorResp types.ErrorResponse
	if err := json.Unmarshal(resp.Body(), &errorResp); err == nil {
		return errors.NewAPIError(resp.StatusCode(), errorResp.Message, "", errorResp.Error)
	}

	// Fallback to status code based errors
	switch resp.StatusCode() {
	case 401:
		return errors.ErrUnauthorized
	case 403:
		return errors.ErrForbidden
	case 404:
		return errors.ErrNotFound
	case 429:
		return errors.ErrRateLimited
	case 500:
		return errors.ErrInternalServer
	default:
		return errors.NewAPIError(resp.StatusCode(), "Unknown error", "", string(resp.Body()))
	}
}

// SetAuth updates the client's authentication
func (c *Client) SetAuth(authenticator auth.Authenticator) {
	c.auth = authenticator
}

// SetTimeout updates the client's timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.httpClient.SetTimeout(timeout)
}

// SetRetryCount updates the client's retry count
func (c *Client) SetRetryCount(count int) {
	c.httpClient.SetRetryCount(count)
}

// SetDebug enables or disables debug mode
func (c *Client) SetDebug(debug bool) {
	c.httpClient.SetDebug(debug)
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetResult(result).
		Get(path)

	if err != nil {
		return &errors.NetworkError{
			Operation: "GET",
			URL:       c.baseURL + path,
			Err:       err,
		}
	}

	return c.handleResponse(resp, result)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	req := c.httpClient.R().SetContext(ctx)

	if body != nil {
		req.SetBody(body)
	}

	if result != nil {
		req.SetResult(result)
	}

	resp, err := req.Post(path)
	if err != nil {
		return &errors.NetworkError{
			Operation: "POST",
			URL:       c.baseURL + path,
			Err:       err,
		}
	}

	return c.handleResponse(resp, result)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	req := c.httpClient.R().SetContext(ctx)

	if body != nil {
		req.SetBody(body)
	}

	if result != nil {
		req.SetResult(result)
	}

	resp, err := req.Put(path)
	if err != nil {
		return &errors.NetworkError{
			Operation: "PUT",
			URL:       c.baseURL + path,
			Err:       err,
		}
	}

	return c.handleResponse(resp, result)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string, result interface{}) error {
	req := c.httpClient.R().SetContext(ctx)

	if result != nil {
		req.SetResult(result)
	}

	resp, err := req.Delete(path)
	if err != nil {
		return &errors.NetworkError{
			Operation: "DELETE",
			URL:       c.baseURL + path,
			Err:       err,
		}
	}

	return c.handleResponse(resp, result)
}

// handleResponse processes successful responses
func (c *Client) handleResponse(resp *resty.Response, result interface{}) error {
	if !resp.IsSuccess() {
		return nil // Error already handled by middleware
	}

	// If result is provided, it's already unmarshaled by resty
	if result != nil {
		return nil
	}

	// Try to parse as standard API response
	var apiResp types.APIResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err == nil {
		if !apiResp.Success && apiResp.Error != "" {
			return errors.NewAPIError(resp.StatusCode(), apiResp.Error, "", apiResp.Message)
		}
	}

	return nil
}

// Ping checks if the API is reachable
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.httpClient.R().
		SetContext(ctx).
		Get("/api/v1/health")

	if err != nil {
		return &errors.NetworkError{
			Operation: "PING",
			URL:       c.baseURL + "/api/v1/health",
			Err:       err,
		}
	}

	if !resp.IsSuccess() {
		return errors.NewAPIError(resp.StatusCode(), "Health check failed", "", string(resp.Body()))
	}

	return nil
}
