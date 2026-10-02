// Package client is the GoatFlow REST API client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/goatkit/goatflow/sdk/go/auth"
	"github.com/goatkit/goatflow/sdk/go/errors"
	"github.com/goatkit/goatflow/sdk/go/types"
)

// Client is a GoatFlow API client. It is safe for concurrent use; SetAuth
// must not be called while requests are in flight.
type Client struct {
	httpClient *resty.Client
	baseURL    string
	auth       auth.Authenticator

	Tickets    *TicketsService
	Articles   *ArticlesService
	Users      *UsersService
	Queues     *QueuesService
	Statistics *StatisticsService
	Search     *SearchService
	Webhooks   *WebhooksService
	Auth       *AuthService
}

// Config configures a Client.
type Config struct {
	// BaseURL is the GoatFlow server root, e.g. "https://goatflow.example.com".
	BaseURL string
	// Auth supplies credentials; nil sends no Authorization header (only
	// POST /api/v1/auth/login and /health work without one).
	Auth      auth.Authenticator
	UserAgent string
	// Timeout per request; default 30s.
	Timeout time.Duration
	// RetryCount is how often a request is retried after a transport error
	// (no HTTP response). Default 0: retrying a POST whose response was lost
	// can create the resource twice.
	RetryCount int
	Debug      bool
}

// NewClient creates a client.
func NewClient(config *Config) *Client {
	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = "goatflow-go-sdk/1.0.0"
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")

	httpClient := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(timeout).
		SetRetryCount(config.RetryCount).
		SetHeader("User-Agent", userAgent).
		SetHeader("Accept", "application/json").
		SetDebug(config.Debug).
		// Unauthenticated API calls may be answered with a redirect to the
		// login page; surface the 3xx instead of decoding the HTML page.
		SetRedirectPolicy(resty.RedirectPolicyFunc(func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}))

	c := &Client{httpClient: httpClient, baseURL: baseURL, auth: config.Auth}
	c.Tickets = &TicketsService{client: c}
	c.Articles = &ArticlesService{client: c}
	c.Users = &UsersService{client: c}
	c.Queues = &QueuesService{client: c}
	c.Statistics = &StatisticsService{client: c}
	c.Search = &SearchService{client: c}
	c.Webhooks = &WebhooksService{client: c}
	c.Auth = &AuthService{client: c}
	return c
}

// NewClientWithAPIKey creates a client that authenticates with a GoatFlow
// API token (gf_...).
func NewClientWithAPIKey(baseURL, apiToken string) *Client {
	return NewClient(&Config{BaseURL: baseURL, Auth: auth.NewAPIKeyAuth(apiToken)})
}

// NewClientWithJWT creates a client that authenticates with a JWT access
// token from POST /api/v1/auth/login. When expiresAt is less than a minute
// ahead, the client renews the pair through POST /api/v1/auth/refresh using
// refreshToken (rotated on every refresh). A zero expiresAt disables renewal.
func NewClientWithJWT(baseURL, token, refreshToken string, expiresAt time.Time) *Client {
	c := NewClient(&Config{BaseURL: baseURL})
	c.auth = auth.NewJWTAuth(token, refreshToken, expiresAt, c.Auth.RefreshFunc())
	return c
}

// Login logs in with an agent's login and password and switches the client
// to the returned access token, renewed automatically through
// POST /api/v1/auth/refresh. Like SetAuth it must not be called while
// requests are in flight.
func (c *Client) Login(ctx context.Context, login, password string) (*types.TokenPair, error) {
	pair, err := c.Auth.Login(ctx, login, password)
	if err != nil {
		return nil, err
	}
	c.auth = auth.NewJWTAuth(pair.AccessToken, pair.RefreshToken, pair.ExpiresAt(time.Now()), c.Auth.RefreshFunc())
	return pair, nil
}

// SetAuth replaces the client's credentials.
func (c *Client) SetAuth(authenticator auth.Authenticator) {
	c.auth = authenticator
}

// Get sends a GET request and decodes the response into result (nil to
// discard). Like all methods it unwraps the {"success": true, "data": ...}
// envelope and returns *errors.APIError for error responses.
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	_, err := c.do(ctx, http.MethodGet, path, nil, nil, result)
	return err
}

// Post sends a POST request with a JSON body (nil for none).
func (c *Client) Post(ctx context.Context, path string, body, result interface{}) error {
	_, err := c.do(ctx, http.MethodPost, path, nil, body, result)
	return err
}

// Put sends a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body, result interface{}) error {
	_, err := c.do(ctx, http.MethodPut, path, nil, body, result)
	return err
}

// Delete sends a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, result interface{}) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil, nil, result)
	return err
}

// Health returns GET /health. An unhealthy server answers 503, which is
// returned as *errors.APIError.
func (c *Client) Health(ctx context.Context) (*types.Health, error) {
	var health types.Health
	if _, err := c.do(ctx, http.MethodGet, "/health", nil, nil, &health); err != nil {
		return nil, err
	}
	return &health, nil
}

// Ping returns nil when the server reports itself healthy.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Health(ctx)
	return err
}

// do performs an authenticated request; see send.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out interface{}) (*types.Pagination, error) {
	return c.send(ctx, method, path, query, body, out, true)
}

// send performs a request and decodes its response into out (if non-nil). It
// returns the envelope's pagination, when the response has one. With
// authenticate false no Authorization header is sent (login and refresh,
// which must not trigger a token refresh themselves).
func (c *Client) send(ctx context.Context, method, path string, query url.Values, body, out interface{}, authenticate bool) (*types.Pagination, error) {
	req := c.httpClient.R().SetContext(ctx)
	if authenticate && c.auth != nil {
		header, err := c.auth.AuthorizationHeader(ctx)
		if err != nil {
			return nil, err
		}
		req.SetHeader("Authorization", header)
	}
	if len(query) > 0 {
		req.SetQueryParamsFromValues(query)
	}
	if body != nil {
		req.SetHeader("Content-Type", "application/json").SetBody(body)
	}

	resp, err := req.Execute(method, path)
	if err != nil {
		return nil, &errors.NetworkError{Method: method, URL: c.baseURL + path, Err: err}
	}
	return decodeResponse(resp.StatusCode(), resp.Body(), out)
}

// decodeResponse turns an API response into out or an error.
//
// GoatFlow wraps most responses in {"success": bool, "data": ..., "error": ...}
// (paginated lists add "pagination"). Some endpoints answer
// {"success": true, ...fields} without "data", and some send a bare object;
// both are decoded as a whole. Errors are {"error": "message"} or
// {"error": {"code": "...", "message": "..."}}, with or without "success".
func decodeResponse(status int, body []byte, out interface{}) (*types.Pagination, error) {
	body = bytes.TrimSpace(body)
	var fields map[string]json.RawMessage
	isObject := len(body) > 0 && body[0] == '{' && json.Unmarshal(body, &fields) == nil

	success, hasSuccess := boolField(fields, "success")
	if status < 200 || status > 299 || (hasSuccess && !success) {
		return nil, apiError(status, body, fields, isObject)
	}

	var pagination *types.Pagination
	if raw, ok := fields["pagination"]; ok {
		pagination = &types.Pagination{}
		if err := json.Unmarshal(raw, pagination); err != nil {
			return nil, &errors.DecodeError{StatusCode: status, Body: string(body), Err: err}
		}
	}

	if out == nil || len(body) == 0 {
		return pagination, nil
	}
	payload := body
	if data, ok := fields["data"]; ok && hasSuccess {
		payload = data
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return nil, &errors.DecodeError{StatusCode: status, Body: string(body), Err: err}
	}
	return pagination, nil
}

func boolField(fields map[string]json.RawMessage, key string) (value, ok bool) {
	raw, present := fields[key]
	if !present {
		return false, false
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false
	}
	return value, true
}

func apiError(status int, body []byte, fields map[string]json.RawMessage, isObject bool) *errors.APIError {
	e := &errors.APIError{StatusCode: status}
	if raw, ok := fields["error"]; ok {
		var structured struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &e.Message) != nil && json.Unmarshal(raw, &structured) == nil {
			e.Code, e.Message = structured.Code, structured.Message
		}
	}
	if e.Message == "" {
		if raw, ok := fields["message"]; ok {
			_ = json.Unmarshal(raw, &e.Message) //nolint:errcheck // a non-string message is ignored
		}
	}
	if e.Message == "" && (status < 200 || status > 299) {
		e.Message = http.StatusText(status)
	}
	if e.Message == "" {
		e.Message = "request failed"
	}
	if !isObject {
		e.Body = string(body)
	}
	return e
}
