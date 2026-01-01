package user

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserService handles all user-related operations as a microservice.
type UserService struct {
	UnimplementedUserServiceServer
	repository UserRepository
	cache      CacheService
	events     EventBus
	metrics    MetricsCollector
}

// UserRepository defines the interface for user data access.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter *ListFilter) ([]*User, error)
	Search(ctx context.Context, query string) ([]*User, error)
	UpdateLastLogin(ctx context.Context, id string, timestamp time.Time) error
	GetByRole(ctx context.Context, role string) ([]*User, error)
}

// CacheService defines the interface for caching.
type CacheService interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Invalidate(ctx context.Context, pattern string) error
}

// EventBus defines the interface for event publishing.
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
}

// MetricsCollector defines the interface for metrics collection.
type MetricsCollector interface {
	IncrementCounter(name string, labels map[string]string)
	RecordDuration(name string, duration time.Duration, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

// Event represents a domain event.
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Source    string                 `json:"source"`
	Version   string                 `json:"version"`
}

// EventHandler processes events.
type EventHandler func(ctx context.Context, event Event) error

// NewUserService creates a new user service instance.
func NewUserService(repo UserRepository, cache CacheService, events EventBus, metrics MetricsCollector) *UserService {
	return &UserService{
		repository: repo,
		cache:      cache,
		events:     events,
		metrics:    metrics,
	}
}

// CreateUser creates a new user.
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("user.create.duration", time.Since(start), map[string]string{
			"role": req.Role,
		})
	}()

	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		s.metrics.IncrementCounter("user.create.validation_error", nil)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Check if email already exists
	existing, _ := s.repository.GetByEmail(ctx, req.Email)
	if existing != nil {
		s.metrics.IncrementCounter("user.create.duplicate_email", nil)
		return nil, status.Error(codes.AlreadyExists, "Email already registered")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to hash password")
	}

	// Create user entity
	user := &User{
		ID:           generateUserID(),
		Email:        req.Email,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		PasswordHash: string(hashedPassword),
		Role:         req.Role,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Preferences:  make(map[string]interface{}),
	}

	// Save to repository
	if err := s.repository.Create(ctx, user); err != nil {
		s.metrics.IncrementCounter("user.create.error", map[string]string{
			"error": err.Error(),
		})
		return nil, status.Error(codes.Internal, "Failed to create user")
	}

	// Invalidate cache
	s.cache.Invalidate(ctx, "users:*")

	// Publish event
	event := Event{
		ID:        generateEventID(),
		Type:      "user.created",
		Timestamp: time.Now(),
		Source:    "user-service",
		Version:   "1.0",
		Data: map[string]interface{}{
			"user_id":    user.ID,
			"email":      user.Email,
			"role":       user.Role,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
		},
	}

	if err := s.events.Publish(ctx, event); err != nil {
		log.Printf("Failed to publish user.created event: %v", err)
	}

	s.metrics.IncrementCounter("user.created", map[string]string{
		"role": user.Role,
	})

	// Don't return password hash
	user.PasswordHash = ""

	return &CreateUserResponse{
		User: user.ToProto(),
	}, nil
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("user.get.duration", time.Since(start), nil)
	}()

	// Check cache first
	cacheKey := fmt.Sprintf("user:%s", req.Id)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var user User
		if err := json.Unmarshal(cached, &user); err == nil {
			s.metrics.IncrementCounter("user.get.cache_hit", nil)
			user.PasswordHash = "" // Never return password hash
			return &GetUserResponse{
				User: user.ToProto(),
			}, nil
		}
	}

	s.metrics.IncrementCounter("user.get.cache_miss", nil)

	// Get from repository
	user, err := s.repository.GetByID(ctx, req.Id)
	if err != nil {
		s.metrics.IncrementCounter("user.get.error", nil)
		return nil, status.Error(codes.NotFound, "User not found")
	}

	// Cache the result (without password)
	userCopy := *user
	userCopy.PasswordHash = ""
	if data, err := json.Marshal(&userCopy); err == nil {
		s.cache.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	user.PasswordHash = "" // Never return password hash

	return &GetUserResponse{
		User: user.ToProto(),
	}, nil
}

// UpdateUser updates an existing user.
func (s *UserService) UpdateUser(ctx context.Context, req *UpdateUserRequest) (*UpdateUserResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("user.update.duration", time.Since(start), nil)
	}()

	// Get existing user
	existing, err := s.repository.GetByID(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "User not found")
	}

	// Store previous state for event
	previousState := existing.Clone()

	// Apply updates
	if req.FirstName != "" {
		existing.FirstName = req.FirstName
	}
	if req.LastName != "" {
		existing.LastName = req.LastName
	}
	if req.Email != "" && req.Email != existing.Email {
		// Check if new email is already taken
		emailUser, _ := s.repository.GetByEmail(ctx, req.Email)
		if emailUser != nil && emailUser.ID != existing.ID {
			return nil, status.Error(codes.AlreadyExists, "Email already in use")
		}
		existing.Email = req.Email
	}
	if req.Role != "" {
		existing.Role = req.Role
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	existing.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repository.Update(ctx, existing); err != nil {
		s.metrics.IncrementCounter("user.update.error", nil)
		return nil, status.Error(codes.Internal, "Failed to update user")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("user:%s", req.Id)
	s.cache.Delete(ctx, cacheKey)
	s.cache.Invalidate(ctx, "users:*")

	// Publish event
	event := Event{
		ID:        generateEventID(),
		Type:      "user.updated",
		Timestamp: time.Now(),
		Source:    "user-service",
		Version:   "1.0",
		Data: map[string]interface{}{
			"user_id":        existing.ID,
			"previous_state": previousState,
			"current_state":  existing,
			"changes":        s.detectChanges(previousState, existing),
		},
	}

	if err := s.events.Publish(ctx, event); err != nil {
		log.Printf("Failed to publish user.updated event: %v", err)
	}

	s.metrics.IncrementCounter("user.updated", map[string]string{
		"role": existing.Role,
	})

	existing.PasswordHash = "" // Never return password hash

	return &UpdateUserResponse{
		User: existing.ToProto(),
	}, nil
}

// AuthenticateUser authenticates a user with email and password.
func (s *UserService) AuthenticateUser(ctx context.Context, req *AuthenticateUserRequest) (*AuthenticateUserResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("user.authenticate.duration", time.Since(start), nil)
	}()

	// Get user by email
	user, err := s.repository.GetByEmail(ctx, req.Email)
	if err != nil {
		s.metrics.IncrementCounter("user.authenticate.user_not_found", nil)
		return nil, status.Error(codes.Unauthenticated, "Invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		s.metrics.IncrementCounter("user.authenticate.inactive_user", nil)
		return nil, status.Error(codes.PermissionDenied, "Account is inactive")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.metrics.IncrementCounter("user.authenticate.invalid_password", nil)

		// Publish failed login event
		event := Event{
			ID:        generateEventID(),
			Type:      "user.login_failed",
			Timestamp: time.Now(),
			Source:    "user-service",
			Version:   "1.0",
			Data: map[string]interface{}{
				"email":      req.Email,
				"user_id":    user.ID,
				"reason":     "invalid_password",
				"ip_address": req.IpAddress,
			},
		}
		s.events.Publish(ctx, event)

		return nil, status.Error(codes.Unauthenticated, "Invalid credentials")
	}

	// Update last login
	s.repository.UpdateLastLogin(ctx, user.ID, time.Now())

	// Generate session token (simplified - in production use JWT)
	token := generateSessionToken()

	// Cache session
	sessionKey := fmt.Sprintf("session:%s", token)
	sessionData := map[string]interface{}{
		"user_id":    user.ID,
		"email":      user.Email,
		"role":       user.Role,
		"created_at": time.Now(),
	}
	if data, err := json.Marshal(sessionData); err == nil {
		s.cache.Set(ctx, sessionKey, data, 24*time.Hour)
	}

	// Publish successful login event
	event := Event{
		ID:        generateEventID(),
		Type:      "user.logged_in",
		Timestamp: time.Now(),
		Source:    "user-service",
		Version:   "1.0",
		Data: map[string]interface{}{
			"user_id":    user.ID,
			"email":      user.Email,
			"role":       user.Role,
			"ip_address": req.IpAddress,
			"user_agent": req.UserAgent,
		},
	}
	s.events.Publish(ctx, event)

	s.metrics.IncrementCounter("user.authenticate.success", map[string]string{
		"role": user.Role,
	})

	user.PasswordHash = "" // Never return password hash

	return &AuthenticateUserResponse{
		User:  user.ToProto(),
		Token: token,
	}, nil
}

// ListUsers lists users with filtering.
func (s *UserService) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("user.list.duration", time.Since(start), nil)
	}()

	// Build filter
	filter := &ListFilter{
		Role:     req.Role,
		IsActive: req.IsActive,
		Limit:    int(req.Limit),
		Offset:   int(req.Offset),
	}

	// Get from repository
	users, err := s.repository.List(ctx, filter)
	if err != nil {
		s.metrics.IncrementCounter("user.list.error", nil)
		return nil, status.Error(codes.Internal, "Failed to list users")
	}

	// Remove password hashes
	for _, user := range users {
		user.PasswordHash = ""
	}

	s.metrics.IncrementCounter("user.list.success", map[string]string{
		"count": fmt.Sprintf("%d", len(users)),
	})

	return s.buildListResponse(users), nil
}

// Start starts the gRPC server.
func (s *UserService) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(s.unaryInterceptor),
	)

	RegisterUserServiceServer(grpcServer, s)

	log.Printf("User service starting on port %d", port)
	return grpcServer.Serve(lis)
}

// unaryInterceptor adds logging and metrics to all unary RPC calls.
func (s *UserService) unaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	// Call the handler
	resp, err := handler(ctx, req)

	// Record metrics
	duration := time.Since(start)
	s.metrics.RecordDuration("grpc.request.duration", duration, map[string]string{
		"method": info.FullMethod,
		"status": grpcStatusCode(err),
	})

	if err != nil {
		log.Printf("gRPC error: %s: %v", info.FullMethod, err)
		s.metrics.IncrementCounter("grpc.request.error", map[string]string{
			"method": info.FullMethod,
		})
	}

	return resp, err
}

// Helper functions

func (s *UserService) validateCreateRequest(req *CreateUserRequest) error {
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if req.Password == "" {
		return fmt.Errorf("password is required")
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if req.FirstName == "" {
		return fmt.Errorf("first name is required")
	}
	if req.LastName == "" {
		return fmt.Errorf("last name is required")
	}
	if req.Role == "" {
		req.Role = "Customer" // Default role
	}
	return nil
}

func (s *UserService) detectChanges(old, new *User) map[string]interface{} {
	changes := make(map[string]interface{})

	if old.FirstName != new.FirstName {
		changes["first_name"] = map[string]string{"old": old.FirstName, "new": new.FirstName}
	}
	if old.LastName != new.LastName {
		changes["last_name"] = map[string]string{"old": old.LastName, "new": new.LastName}
	}
	if old.Email != new.Email {
		changes["email"] = map[string]string{"old": old.Email, "new": new.Email}
	}
	if old.Role != new.Role {
		changes["role"] = map[string]string{"old": old.Role, "new": new.Role}
	}
	if old.IsActive != new.IsActive {
		changes["is_active"] = map[string]bool{"old": old.IsActive, "new": new.IsActive}
	}

	return changes
}

func (s *UserService) buildListResponse(users []*User) *ListUsersResponse {
	return &ListUsersResponse{
		Users: s.usersToProto(users),
		Total: int32(len(users)),
	}
}

func (s *UserService) usersToProto(users []*User) []*UserProto {
	result := make([]*UserProto, len(users))
	for i, user := range users {
		result[i] = user.ToProto()
	}
	return result
}

func grpcStatusCode(err error) string {
	if err == nil {
		return "OK"
	}
	if st, ok := status.FromError(err); ok {
		return st.Code().String()
	}
	return "UNKNOWN"
}

func generateUserID() string {
	return fmt.Sprintf("USR-%d", time.Now().UnixNano())
}

func generateEventID() string {
	return fmt.Sprintf("EVT-%d", time.Now().UnixNano())
}

func generateSessionToken() string {
	return fmt.Sprintf("SES-%d-%d", time.Now().UnixNano(), time.Now().Unix())
}
