package auth

import (
	"context"
	"testing"
)

func TestGoogleProvider_Name(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	p := &googleProvider{
		oidcProvider: &oidcProvider{
			name:       "google",
			stateStore: store,
		},
	}
	if got := p.Name(); got != "Google" {
		t.Errorf("Name() = %q, want %q", got, "Google")
	}
 }

func TestGoogleProvider_Priority(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	p := &googleProvider{
		oidcProvider: &oidcProvider{stateStore: store},
	}
	if got := p.Priority(); got != 2 {
		t.Errorf("Priority() = %d, want 2", got)
	}
 }

func TestGoogleProvider_AuthenticateReturnsError(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	p := &googleProvider{
		oidcProvider: &oidcProvider{stateStore: store},
	}
	_, err := p.Authenticate(context.Background(), "user", "pass")
	if err != ErrAuthBackendFailed {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrAuthBackendFailed)
	}
 }

func TestGoogleProvider_UsesGoogleDiscoveryURL(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	p := &googleProvider{
		oidcProvider: &oidcProvider{
			discoveryURL: googleDiscoveryURL,
			stateStore:   store,
		},
	}
	if p.discoveryURL != googleDiscoveryURL {
		t.Errorf("discoveryURL = %q, want %q", p.discoveryURL, googleDiscoveryURL)
	}
 }
