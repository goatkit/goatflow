package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/goatkit/goatflow/sdk/go/auth"
	"github.com/goatkit/goatflow/sdk/go/types"
)

// UsersService covers /api/v1/users (agents).
type UsersService struct {
	client *Client
}

// List returns one page of agents.
func (s *UsersService) List(ctx context.Context, options *types.UserListOptions) (*types.UserList, error) {
	query := url.Values{}
	if o := options; o != nil {
		setInt(query, "page", o.Page)
		setInt(query, "per_page", o.PerPage)
		setString(query, "search", o.Search)
		setString(query, "valid", o.Valid)
		setUint(query, "group_id", o.GroupID)
	}
	list := &types.UserList{}
	pagination, err := s.client.do(ctx, http.MethodGet, "/api/v1/users", query, nil, &list.Users)
	if err != nil {
		return nil, err
	}
	if pagination != nil {
		list.Pagination = *pagination
	}
	return list, nil
}

// Get returns one agent, including email and preferences.
func (s *UsersService) Get(ctx context.Context, id uint) (*types.User, error) {
	var user types.User
	if _, err := s.client.do(ctx, http.MethodGet, userPath(id), nil, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Me returns the authenticated agent.
func (s *UsersService) Me(ctx context.Context) (*types.CurrentUser, error) {
	var user types.CurrentUser
	if _, err := s.client.do(ctx, http.MethodGet, "/api/v1/users/me", nil, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Create creates an agent.
func (s *UsersService) Create(ctx context.Context, request *types.UserCreateRequest) (*types.CreatedUser, error) {
	var user types.CreatedUser
	if _, err := s.client.do(ctx, http.MethodPost, "/api/v1/users", nil, request, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Update changes the non-nil fields of request. The API answers only with
// the id; use Get to read the result.
func (s *UsersService) Update(ctx context.Context, id uint, request *types.UserUpdateRequest) error {
	_, err := s.client.do(ctx, http.MethodPut, userPath(id), nil, request, nil)
	return err
}

// Delete deletes an agent.
func (s *UsersService) Delete(ctx context.Context, id uint) error {
	_, err := s.client.do(ctx, http.MethodDelete, userPath(id), nil, nil, nil)
	return err
}

func userPath(id uint) string { return fmt.Sprintf("/api/v1/users/%d", id) }

// QueuesService covers /api/v1/queues.
type QueuesService struct {
	client *Client
}

// List returns the queues the caller can read.
func (s *QueuesService) List(ctx context.Context, options *types.QueueListOptions) ([]types.Queue, error) {
	query := url.Values{}
	if o := options; o != nil {
		setString(query, "valid", o.Valid)
		if o.IncludeStats {
			query.Set("include_stats", "true")
		}
	}
	var queues []types.Queue
	if _, err := s.client.do(ctx, http.MethodGet, "/api/v1/queues", query, nil, &queues); err != nil {
		return nil, err
	}
	return queues, nil
}

// Get returns one queue.
func (s *QueuesService) Get(ctx context.Context, id uint) (*types.Queue, error) {
	var queue types.Queue
	if _, err := s.client.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/queues/%d", id), nil, nil, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

// StatisticsService covers /api/v1/statistics.
type StatisticsService struct {
	client *Client
}

// Dashboard returns ticket counts overall, per queue and per priority, and
// the newest tickets, limited to the queues the caller can read.
func (s *StatisticsService) Dashboard(ctx context.Context) (*types.DashboardStatistics, error) {
	var stats types.DashboardStatistics
	if _, err := s.client.do(ctx, http.MethodGet, "/api/v1/statistics/dashboard", nil, nil, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// SearchService covers POST /api/v1/search.
type SearchService struct {
	client *Client
}

// Query runs a full-text search.
func (s *SearchService) Query(ctx context.Context, query *types.SearchQuery) (*types.SearchResults, error) {
	var results types.SearchResults
	if _, err := s.client.do(ctx, http.MethodPost, "/api/v1/search", nil, query, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// WebhooksService covers /api/v1/webhooks (admin only).
type WebhooksService struct {
	client *Client
}

// List retrieves all webhooks
func (s *WebhooksService) List(ctx context.Context) ([]types.Webhook, error) {
	var result []types.Webhook
	err := s.client.Get(ctx, "/api/v1/webhooks", &result)
	return result, err
}

// Get retrieves a specific webhook by ID
func (s *WebhooksService) Get(ctx context.Context, id uint) (*types.Webhook, error) {
	path := fmt.Sprintf("/api/v1/webhooks/%d", id)
	var result types.Webhook
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// Create creates a new webhook
func (s *WebhooksService) Create(ctx context.Context, webhook *types.Webhook) (*types.Webhook, error) {
	var result types.Webhook
	err := s.client.Post(ctx, "/api/v1/webhooks", webhook, &result)
	return &result, err
}

// Update updates an existing webhook
func (s *WebhooksService) Update(ctx context.Context, id uint, webhook *types.Webhook) (*types.Webhook, error) {
	path := fmt.Sprintf("/api/v1/webhooks/%d", id)
	var result types.Webhook
	err := s.client.Put(ctx, path, webhook, &result)
	return &result, err
}

// Delete deletes a webhook
func (s *WebhooksService) Delete(ctx context.Context, id uint) error {
	path := fmt.Sprintf("/api/v1/webhooks/%d", id)
	return s.client.Delete(ctx, path, nil)
}

// Test sends a webhook.test event now and returns the recorded delivery
func (s *WebhooksService) Test(ctx context.Context, id uint) (*types.WebhookDelivery, error) {
	path := fmt.Sprintf("/api/v1/webhooks/%d/test", id)
	var result types.WebhookDelivery
	err := s.client.Post(ctx, path, nil, &result)
	return &result, err
}

// GetDeliveries retrieves the newest deliveries of a webhook
func (s *WebhooksService) GetDeliveries(ctx context.Context, id uint) ([]types.WebhookDelivery, error) {
	path := fmt.Sprintf("/api/v1/webhooks/%d/deliveries", id)
	var result []types.WebhookDelivery
	err := s.client.Get(ctx, path, &result)
	return result, err
}

// GetDelivery retrieves one delivery including payload and response body
func (s *WebhooksService) GetDelivery(ctx context.Context, deliveryID uint) (*types.WebhookDelivery, error) {
	path := fmt.Sprintf("/api/v1/webhook-deliveries/%d", deliveryID)
	var result types.WebhookDelivery
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// Redeliver sends a delivery's payload again and returns the new delivery
func (s *WebhooksService) Redeliver(ctx context.Context, deliveryID uint) (*types.WebhookDelivery, error) {
	path := fmt.Sprintf("/api/v1/webhook-deliveries/%d/redeliver", deliveryID)
	var result types.WebhookDelivery
	err := s.client.Post(ctx, path, nil, &result)
	return &result, err
}

// AuthService covers POST /api/v1/auth/login and /api/v1/auth/refresh. Both
// are sent without the client's credentials.
type AuthService struct {
	client *Client
}

// Login exchanges an agent's login and password for a token pair. It does
// not change the client's credentials; Client.Login does.
func (s *AuthService) Login(ctx context.Context, login, password string) (*types.TokenPair, error) {
	body := types.LoginRequest{Login: login, Password: password}
	return s.tokenPair(ctx, "/api/v1/auth/login", body)
}

// Refresh exchanges a refresh token for a new access token and a new
// (rotated) refresh token. A rejected refresh token is a 401 *errors.APIError.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*types.TokenPair, error) {
	return s.tokenPair(ctx, "/api/v1/auth/refresh", map[string]string{"refresh_token": refreshToken})
}

// RefreshFunc returns an auth.RefreshFunc backed by Refresh, for
// auth.NewJWTAuth.
func (s *AuthService) RefreshFunc() auth.RefreshFunc {
	return func(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
		pair, err := s.Refresh(ctx, refreshToken)
		if err != nil {
			return "", "", time.Time{}, err
		}
		return pair.AccessToken, pair.RefreshToken, pair.ExpiresAt(time.Now()), nil
	}
}

func (s *AuthService) tokenPair(ctx context.Context, path string, body interface{}) (*types.TokenPair, error) {
	var pair types.TokenPair
	if _, err := s.client.send(ctx, http.MethodPost, path, nil, body, &pair, false); err != nil {
		return nil, err
	}
	return &pair, nil
}
