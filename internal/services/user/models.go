package user

import (
	"context"
	"time"
)

// User represents a system user
type User struct {
	ID             string                 `json:"id" db:"id"`
	Email          string                 `json:"email" db:"email"`
	FirstName      string                 `json:"first_name" db:"first_name"`
	LastName       string                 `json:"last_name" db:"last_name"`
	PasswordHash   string                 `json:"-" db:"password_hash"`
	Role           string                 `json:"role" db:"role"`
	Department     string                 `json:"department,omitempty" db:"department"`
	Phone          string                 `json:"phone,omitempty" db:"phone"`
	Mobile         string                 `json:"mobile,omitempty" db:"mobile"`
	IsActive       bool                   `json:"is_active" db:"is_active"`
	EmailVerified  bool                   `json:"email_verified" db:"email_verified"`
	TwoFactorAuth  bool                   `json:"two_factor_auth" db:"two_factor_auth"`
	Preferences    map[string]interface{} `json:"preferences" db:"preferences"`
	LastLoginAt    *time.Time             `json:"last_login_at,omitempty" db:"last_login_at"`
	LastActivityAt *time.Time             `json:"last_activity_at,omitempty" db:"last_activity_at"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
	DeletedAt      *time.Time             `json:"deleted_at,omitempty" db:"deleted_at"`
	
	// Relationships
	Groups      []Group      `json:"groups,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`
	Sessions    []Session    `json:"sessions,omitempty"`
}

// Group represents a user group
type Group struct {
	ID          string                 `json:"id" db:"id"`
	Name        string                 `json:"name" db:"name"`
	Description string                 `json:"description" db:"description"`
	Permissions []Permission           `json:"permissions,omitempty"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// Permission represents a system permission
type Permission struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Resource    string    `json:"resource" db:"resource"`
	Action      string    `json:"action" db:"action"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Session represents a user session
type Session struct {
	ID         string                 `json:"id" db:"id"`
	UserID     string                 `json:"user_id" db:"user_id"`
	Token      string                 `json:"token" db:"token"`
	IPAddress  string                 `json:"ip_address" db:"ip_address"`
	UserAgent  string                 `json:"user_agent" db:"user_agent"`
	ExpiresAt  time.Time              `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
	LastUsedAt time.Time              `json:"last_used_at" db:"last_used_at"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
}

// ListFilter represents user list filtering options
type ListFilter struct {
	Role       string    `json:"role,omitempty"`
	Department string    `json:"department,omitempty"`
	IsActive   *bool     `json:"is_active,omitempty"`
	GroupID    string    `json:"group_id,omitempty"`
	CreatedFrom time.Time `json:"created_from,omitempty"`
	CreatedTo   time.Time `json:"created_to,omitempty"`
	SortBy     string    `json:"sort_by,omitempty"`
	SortOrder  string    `json:"sort_order,omitempty"`
	Limit      int       `json:"limit,omitempty"`
	Offset     int       `json:"offset,omitempty"`
}

// Clone creates a deep copy of the user
func (u *User) Clone() *User {
	clone := *u
	
	// Clone maps
	if u.Preferences != nil {
		clone.Preferences = make(map[string]interface{})
		for k, v := range u.Preferences {
			clone.Preferences[k] = v
		}
	}
	
	// Clone relationships
	if u.Groups != nil {
		clone.Groups = make([]Group, len(u.Groups))
		copy(clone.Groups, u.Groups)
	}
	
	if u.Permissions != nil {
		clone.Permissions = make([]Permission, len(u.Permissions))
		copy(clone.Permissions, u.Permissions)
	}
	
	if u.Sessions != nil {
		clone.Sessions = make([]Session, len(u.Sessions))
		copy(clone.Sessions, u.Sessions)
	}
	
	return &clone
}

// ToProto converts the user to protobuf format
func (u *User) ToProto() *UserProto {
	proto := &UserProto{
		Id:            u.ID,
		Email:         u.Email,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Role:          u.Role,
		Department:    u.Department,
		Phone:         u.Phone,
		Mobile:        u.Mobile,
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		TwoFactorAuth: u.TwoFactorAuth,
		CreatedAt:     u.CreatedAt.Unix(),
		UpdatedAt:     u.UpdatedAt.Unix(),
	}
	
	if u.LastLoginAt != nil {
		proto.LastLoginAt = u.LastLoginAt.Unix()
	}
	
	if u.LastActivityAt != nil {
		proto.LastActivityAt = u.LastActivityAt.Unix()
	}
	
	return proto
}

// User roles
const (
	RoleAdmin    = "Admin"
	RoleAgent    = "Agent"
	RoleCustomer = "Customer"
	RoleGuest    = "Guest"
)

// Permission actions
const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionList   = "list"
	ActionManage = "manage"
)

// Permission resources
const (
	ResourceTicket       = "ticket"
	ResourceUser         = "user"
	ResourceQueue        = "queue"
	ResourceReport       = "report"
	ResourceSystem       = "system"
	ResourceOrganization = "organization"
)

// Proto stub definitions

// UserProto represents a user in protobuf format
type UserProto struct {
	Id             string `json:"id"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Role           string `json:"role"`
	Department     string `json:"department"`
	Phone          string `json:"phone"`
	Mobile         string `json:"mobile"`
	IsActive       bool   `json:"is_active"`
	EmailVerified  bool   `json:"email_verified"`
	TwoFactorAuth  bool   `json:"two_factor_auth"`
	LastLoginAt    int64  `json:"last_login_at"`
	LastActivityAt int64  `json:"last_activity_at"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

// Request/Response types for gRPC

type CreateUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	Department string `json:"department"`
}

type CreateUserResponse struct {
	User *UserProto `json:"user"`
}

type GetUserRequest struct {
	Id string `json:"id"`
}

type GetUserResponse struct {
	User *UserProto `json:"user"`
}

type UpdateUserRequest struct {
	Id        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	IsActive  *bool  `json:"is_active"`
}

type UpdateUserResponse struct {
	User *UserProto `json:"user"`
}

type AuthenticateUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	IpAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
}

type AuthenticateUserResponse struct {
	User  *UserProto `json:"user"`
	Token string     `json:"token"`
}

type ListUsersRequest struct {
	Role     string `json:"role"`
	IsActive *bool  `json:"is_active"`
	Limit    int32  `json:"limit"`
	Offset   int32  `json:"offset"`
}

type ListUsersResponse struct {
	Users []*UserProto `json:"users"`
	Total int32        `json:"total"`
}

// gRPC Server interface
type UserServiceServer interface {
	CreateUser(context.Context, *CreateUserRequest) (*CreateUserResponse, error)
	GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error)
	UpdateUser(context.Context, *UpdateUserRequest) (*UpdateUserResponse, error)
	AuthenticateUser(context.Context, *AuthenticateUserRequest) (*AuthenticateUserResponse, error)
	ListUsers(context.Context, *ListUsersRequest) (*ListUsersResponse, error)
}

// UnimplementedUserServiceServer can be embedded to have forward compatible implementations
type UnimplementedUserServiceServer struct{}

func (UnimplementedUserServiceServer) CreateUser(context.Context, *CreateUserRequest) (*CreateUserResponse, error) {
	return nil, nil
}

func (UnimplementedUserServiceServer) GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error) {
	return nil, nil
}

func (UnimplementedUserServiceServer) UpdateUser(context.Context, *UpdateUserRequest) (*UpdateUserResponse, error) {
	return nil, nil
}

func (UnimplementedUserServiceServer) AuthenticateUser(context.Context, *AuthenticateUserRequest) (*AuthenticateUserResponse, error) {
	return nil, nil
}

func (UnimplementedUserServiceServer) ListUsers(context.Context, *ListUsersRequest) (*ListUsersResponse, error) {
	return nil, nil
}

// RegisterUserServiceServer registers the service with gRPC server
func RegisterUserServiceServer(s interface{}, srv UserServiceServer) {
	// Stub implementation for compilation
}