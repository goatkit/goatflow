package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// StateStore manages OIDC OAuth2 state tokens with TTL-based expiry.
type StateStore interface {
	// StoreState saves a state token with its provider ID, type, org, and optional code verifier.
	StoreState(providerID uint, providerType string, token string, orgID uint, codeVerifier string) error

	// GetState returns provider ID, type, org ID, and code verifier without consuming the token.
	// ok=false when the token is missing or expired.
	GetState(token string) (providerID uint, providerType string, orgID uint, codeVerifier string, ok bool)

	// ConsumeState atomically reads and removes the state token.
	// ok=false when the token is missing, already consumed, or expired.
	ConsumeState(token string) (providerID uint, providerType string, orgID uint, codeVerifier string, ok bool)
}

// stateEntry holds OIDC OAuth2 state metadata.
type stateEntry struct {
	ProviderID   uint
	ProviderType string
	OrgID        uint
	ExpiresAt    time.Time
	CodeVerifier string
}

// MemoryStateStore is an in-memory, thread-safe store for OIDC OAuth2 state tokens.
// State entries expire after 5 minutes and are lazily evicted on access.
type MemoryStateStore struct {
	mu      sync.RWMutex
	entries map[string]*stateEntry
}

const stateTTL = 5 * time.Minute

// NewMemoryStateStore creates a ready-to-use state store.
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		entries: make(map[string]*stateEntry),
	}
}

// StoreState saves a state token with its provider ID, type, org, and optional code verifier.
func (s *MemoryStateStore) StoreState(providerID uint, providerType string, token string, orgID uint, codeVerifier string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[token]; exists {
		return fmt.Errorf("state token %s already exists", token)
	}

	s.entries[token] = &stateEntry{
		ProviderID:   providerID,
		ProviderType: providerType,
		OrgID:        orgID,
		ExpiresAt:    time.Now().Add(stateTTL),
		CodeVerifier: codeVerifier,
	}
	return nil
}

// GetState returns provider ID, type, org ID, and code verifier for the given token.
func (s *MemoryStateStore) GetState(token string) (providerID uint, providerType string, orgID uint, codeVerifier string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, exists := s.entries[token]
	if !exists {
		return 0, "", 0, "", false
	}
	if time.Now().After(e.ExpiresAt) {
		return 0, "", 0, "", false
	}
	return e.ProviderID, e.ProviderType, e.OrgID, e.CodeVerifier, true
}

// ConsumeState atomically reads and removes the state token.
func (s *MemoryStateStore) ConsumeState(token string) (providerID uint, providerType string, orgID uint, codeVerifier string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, exists := s.entries[token]
	if !exists {
		return 0, "", 0, "", false
	}
	if time.Now().After(e.ExpiresAt) {
		delete(s.entries, token)
		return 0, "", 0, "", false
	}
	delete(s.entries, token)
	return e.ProviderID, e.ProviderType, e.OrgID, e.CodeVerifier, true
}

// generateRandomToken returns a 32-byte hex-encoded random token.
func generateRandomToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}