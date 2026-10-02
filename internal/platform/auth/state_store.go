package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// StateData is what the store keeps for one SSO state token (OIDC state or
// SAML RelayState) between the redirect to the IdP and the callback.
type StateData struct {
	ProviderID   uint
	ProviderType string
	OrgID        uint
	// CodeVerifier is the PKCE verifier (OIDC only).
	CodeVerifier string
	// BrowserBinding is the hash of the nonce cookie set in the browser that
	// started the login. The callback must come from the same browser, so a
	// callback URL captured from another login cannot be replayed (login CSRF).
	BrowserBinding string
}

// StateStore manages SSO state tokens with TTL-based expiry.
type StateStore interface {
	// StoreState saves a state token. It fails when the token already exists.
	StoreState(token string, data StateData) error

	// GetState returns the data for a token without consuming it.
	// ok=false when the token is missing or expired.
	GetState(token string) (data StateData, ok bool)

	// ConsumeState atomically reads and removes the state token.
	// ok=false when the token is missing, already consumed, or expired.
	ConsumeState(token string) (data StateData, ok bool)
}

// stateEntry holds the stored data and its expiry.
type stateEntry struct {
	data      StateData
	expiresAt time.Time
}

// MemoryStateStore is an in-memory, thread-safe store for SSO state tokens.
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

// StoreState saves a state token. It fails when the token already exists.
func (s *MemoryStateStore) StoreState(token string, data StateData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[token]; exists {
		return fmt.Errorf("state token %s already exists", token)
	}

	s.entries[token] = &stateEntry{
		data:      data,
		expiresAt: time.Now().Add(stateTTL),
	}
	return nil
}

// GetState returns the data for a token without consuming it.
func (s *MemoryStateStore) GetState(token string) (StateData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, exists := s.entries[token]
	if !exists || time.Now().After(e.expiresAt) {
		return StateData{}, false
	}
	return e.data, true
}

// ConsumeState atomically reads and removes the state token.
func (s *MemoryStateStore) ConsumeState(token string) (StateData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, exists := s.entries[token]
	if !exists {
		return StateData{}, false
	}
	delete(s.entries, token)
	if time.Now().After(e.expiresAt) {
		return StateData{}, false
	}
	return e.data, true
}

// generateRandomToken returns a 32-byte hex-encoded random token.
func generateRandomToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
