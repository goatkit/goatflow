package client

import (
	"context"
	"fmt"

	"github.com/goatkit/goatflow/sdk/go/types"
)

// UsersService handles user-related API operations
type UsersService struct {
	client *Client
}

// List retrieves all users
func (s *UsersService) List(ctx context.Context) ([]types.User, error) {
	var result []types.User
	err := s.client.Get(ctx, "/api/v1/users", &result)
	return result, err
}

// Get retrieves a specific user by ID
func (s *UsersService) Get(ctx context.Context, id uint) (*types.User, error) {
	path := fmt.Sprintf("/api/v1/users/%d", id)
	var result types.User
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// Create creates a new user
func (s *UsersService) Create(ctx context.Context, request *types.UserCreateRequest) (*types.User, error) {
	var result types.User
	err := s.client.Post(ctx, "/api/v1/users", request, &result)
	return &result, err
}

// Update updates an existing user
func (s *UsersService) Update(ctx context.Context, id uint, request *types.UserUpdateRequest) (*types.User, error) {
	path := fmt.Sprintf("/api/v1/users/%d", id)
	var result types.User
	err := s.client.Put(ctx, path, request, &result)
	return &result, err
}

// Delete deletes a user
func (s *UsersService) Delete(ctx context.Context, id uint) error {
	path := fmt.Sprintf("/api/v1/users/%d", id)
	return s.client.Delete(ctx, path, nil)
}

// QueuesService handles queue-related API operations
type QueuesService struct {
	client *Client
}

// List retrieves all queues
func (s *QueuesService) List(ctx context.Context) ([]types.Queue, error) {
	var result []types.Queue
	err := s.client.Get(ctx, "/api/v1/queues", &result)
	return result, err
}

// Get retrieves a specific queue by ID
func (s *QueuesService) Get(ctx context.Context, id uint) (*types.Queue, error) {
	path := fmt.Sprintf("/api/v1/queues/%d", id)
	var result types.Queue
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// DashboardService handles dashboard-related API operations
type DashboardService struct {
	client *Client
}

// GetStats retrieves dashboard statistics
func (s *DashboardService) GetStats(ctx context.Context) (*types.DashboardStats, error) {
	var result types.DashboardStats
	err := s.client.Get(ctx, "/api/v1/dashboard/stats", &result)
	return &result, err
}

// GetMyTickets retrieves tickets assigned to the current user
func (s *DashboardService) GetMyTickets(ctx context.Context) ([]types.Ticket, error) {
	var result []types.Ticket
	err := s.client.Get(ctx, "/api/v1/dashboard/my-tickets", &result)
	return result, err
}

// GetRecentTickets retrieves recently created tickets
func (s *DashboardService) GetRecentTickets(ctx context.Context) ([]types.Ticket, error) {
	var result []types.Ticket
	err := s.client.Get(ctx, "/api/v1/dashboard/recent-tickets", &result)
	return result, err
}

// LDAPService handles LDAP-related API operations
type LDAPService struct {
	client *Client
}

// GetUsers retrieves users from LDAP
func (s *LDAPService) GetUsers(ctx context.Context) ([]types.LDAPUser, error) {
	var result []types.LDAPUser
	err := s.client.Get(ctx, "/api/v1/ldap/users", &result)
	return result, err
}

// GetUser retrieves a specific user from LDAP
func (s *LDAPService) GetUser(ctx context.Context, username string) (*types.LDAPUser, error) {
	path := fmt.Sprintf("/api/v1/ldap/users/%s", username)
	var result types.LDAPUser
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// SyncUsers synchronizes users from LDAP
func (s *LDAPService) SyncUsers(ctx context.Context) (*types.LDAPSyncResult, error) {
	var result types.LDAPSyncResult
	err := s.client.Post(ctx, "/api/v1/ldap/sync", nil, &result)
	return &result, err
}

// TestConnection tests LDAP connection
func (s *LDAPService) TestConnection(ctx context.Context) error {
	return s.client.Post(ctx, "/api/v1/ldap/test", nil, nil)
}

// GetSyncStatus retrieves LDAP sync status
func (s *LDAPService) GetSyncStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := s.client.Get(ctx, "/api/v1/ldap/sync/status", &result)
	return result, err
}

// WebhooksService handles webhook-related API operations
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

// Test tests a webhook
func (s *WebhooksService) Test(ctx context.Context, id uint) error {
	path := fmt.Sprintf("/api/v1/webhooks/%d/test", id)
	return s.client.Post(ctx, path, nil, nil)
}

// GetDeliveries retrieves webhook deliveries
func (s *WebhooksService) GetDeliveries(ctx context.Context, id uint) ([]types.WebhookDelivery, error) {
	path := fmt.Sprintf("/api/v1/webhooks/%d/deliveries", id)
	var result []types.WebhookDelivery
	err := s.client.Get(ctx, path, &result)
	return result, err
}

// NotesService handles internal notes-related API operations
type NotesService struct {
	client *Client
}

// GetNotes retrieves all notes for a ticket
func (s *NotesService) GetNotes(ctx context.Context, ticketID uint) ([]types.InternalNote, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/notes", ticketID)
	var result []types.InternalNote
	err := s.client.Get(ctx, path, &result)
	return result, err
}

// GetNote retrieves a specific note
func (s *NotesService) GetNote(ctx context.Context, ticketID, noteID uint) (*types.InternalNote, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/notes/%d", ticketID, noteID)
	var result types.InternalNote
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// CreateNote creates a new note
func (s *NotesService) CreateNote(ctx context.Context, ticketID uint, note *types.InternalNote) (*types.InternalNote, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/notes", ticketID)
	var result types.InternalNote
	err := s.client.Post(ctx, path, note, &result)
	return &result, err
}

// UpdateNote updates an existing note
func (s *NotesService) UpdateNote(ctx context.Context, ticketID, noteID uint, note *types.InternalNote) (*types.InternalNote, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/notes/%d", ticketID, noteID)
	var result types.InternalNote
	err := s.client.Put(ctx, path, note, &result)
	return &result, err
}

// DeleteNote deletes a note
func (s *NotesService) DeleteNote(ctx context.Context, ticketID, noteID uint) error {
	path := fmt.Sprintf("/api/v1/tickets/%d/notes/%d", ticketID, noteID)
	return s.client.Delete(ctx, path, nil)
}

// GetTemplates retrieves all note templates
func (s *NotesService) GetTemplates(ctx context.Context) ([]types.NoteTemplate, error) {
	var result []types.NoteTemplate
	err := s.client.Get(ctx, "/api/v1/notes/templates", &result)
	return result, err
}

// CreateTemplate creates a new note template
func (s *NotesService) CreateTemplate(ctx context.Context, template *types.NoteTemplate) (*types.NoteTemplate, error) {
	var result types.NoteTemplate
	err := s.client.Post(ctx, "/api/v1/notes/templates", template, &result)
	return &result, err
}

// AuthService handles authentication-related operations
type AuthService struct {
	client *Client
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, request *types.AuthLoginRequest) (*types.AuthLoginResponse, error) {
	var result types.AuthLoginResponse
	err := s.client.Post(ctx, "/api/v1/auth/login", request, &result)
	return &result, err
}

// Logout logs out the current user
func (s *AuthService) Logout(ctx context.Context) error {
	return s.client.Post(ctx, "/api/v1/auth/logout", nil, nil)
}

// RefreshToken refreshes the access token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*types.AuthLoginResponse, error) {
	body := map[string]string{"refresh_token": refreshToken}
	var result types.AuthLoginResponse
	err := s.client.Post(ctx, "/api/v1/auth/refresh", body, &result)
	return &result, err
}

// GetProfile retrieves the current user's profile
func (s *AuthService) GetProfile(ctx context.Context) (*types.User, error) {
	var result types.User
	err := s.client.Get(ctx, "/api/v1/auth/profile", &result)
	return &result, err
}

// UpdateProfile updates the current user's profile
func (s *AuthService) UpdateProfile(ctx context.Context, request *types.UserUpdateRequest) (*types.User, error) {
	var result types.User
	err := s.client.Put(ctx, "/api/v1/auth/profile", request, &result)
	return &result, err
}
