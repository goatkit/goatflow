package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/goatkit/goatflow/internal/models"
)

// StaticAuthProvider offers simple in-memory users for demos/tests.
type StaticAuthProvider struct {
	users    map[string]*models.User // key: login (lowercase)
	pwMap    map[string]string       // login -> plain or hashed (bcrypt/sha*)
	hasher   *PasswordHasher
	priority int
	name     string
}

// static user spec env format: user:password:Role(Agent|Customer|Admin)
// Multiple separated by commas.
func NewStaticAuthProvider(specs []string) *StaticAuthProvider {
	users := map[string]*models.User{}
	pw := map[string]string{}
	hasher := NewPasswordHasher()
	for _, s := range specs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		parts := strings.Split(s, ":")
		if len(parts) < 3 {
			continue
		}
		login := parts[0]
		pass := parts[1]
		role := parts[2]
		u := &models.User{Login: login, Email: login + "@static.local", Role: role, ValidID: 1}
		users[strings.ToLower(login)] = u
		pw[strings.ToLower(login)] = pass
	}
	return &StaticAuthProvider{users: users, pwMap: pw, hasher: hasher, priority: 1, name: "Static"}
}

func (p *StaticAuthProvider) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
	if p == nil {
		return nil, ErrAuthBackendFailed
	}
	u, ok := p.users[strings.ToLower(username)]
	if !ok {
		return nil, ErrUserNotFound
	}
	stored := p.pwMap[strings.ToLower(username)]
	// Accept either direct match or hashed match.
	if stored == password || p.hasher.VerifyPassword(password, stored) {
		clone := *u
		return &clone, nil
	}
	return nil, ErrInvalidCredentials
}

func (p *StaticAuthProvider) GetUser(ctx context.Context, identifier string) (*models.User, error) {
	if u, ok := p.users[strings.ToLower(identifier)]; ok {
		clone := *u
		return &clone, nil
	}
	return nil, ErrUserNotFound
}

func (p *StaticAuthProvider) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	return nil, ErrAuthBackendFailed
}

func (p *StaticAuthProvider) Name() string  { return p.name }
func (p *StaticAuthProvider) Priority() int { return p.priority }

// Register factory via init.
func init() {
	_ = RegisterProvider("static", func(deps ProviderDependencies) (AuthProvider, error) {
		// Accept either CONFIG env or STATIC_USERS env variable for now.
		raw := os.Getenv("GOATFLOW_STATIC_USERS")
		if raw == "" {
			// No static users defined, return error so caller can skip.
			return nil, errors.New("no static users configured")
		}
		specs := strings.Split(raw, ",")
		p := NewStaticAuthProvider(specs)
		if len(p.users) == 0 {
			return nil, fmt.Errorf("no valid static users parsed")
		}
		return p, nil
	})
}
