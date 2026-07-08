package auth

import (
	"testing"
	"time"
)

func TestStoreState(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	err := store.StoreState(1, "oidc", "state123", 0, "verifier456")
	if err != nil {
		t.Fatalf("StoreState: %v", err)
	}
	providerID, providerType, org, verifier, ok := store.GetState("state123")
	if !ok {
		t.Fatal("GetState returned ok=false")
	}
	if providerID != 1 {
		t.Errorf("providerID = %d, want %d", providerID, 1)
	}
	if providerType != "oidc" {
		t.Errorf("providerType = %q, want %q", providerType, "oidc")
	}
	if org != 0 {
		t.Errorf("org = %d, want 0", org)
	}
	if verifier != "verifier456" {
		t.Errorf("verifier = %q, want %q", verifier, "verifier456")
	}
 }

func TestStoreState_Duplicate(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	if err := store.StoreState(1, "oidc", "state123", 0, "v"); err != nil {
		t.Fatalf("first store: %v", err)
	}
	err := store.StoreState(1, "oidc", "state123", 0, "v2")
	if err == nil {
		t.Fatal("expected error for duplicate token")
	}
 }

func TestGetState_NotFound(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	providerID, providerType, org, verifier, ok := store.GetState("nonexistent")
	if ok {
		t.Fatal("GetState returned ok=true for missing token")
	}
	if providerID != 0 || providerType != "" || org != 0 || verifier != "" {
		t.Fatalf("expected zero values, got id=%d type=%q org=%d verifier=%q", providerID, providerType, org, verifier)
	}
 }

func TestConsumeState(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	err := store.StoreState(2, "google", "state789", 5, "v123")
	if err != nil {
		t.Fatalf("StoreState: %v", err)
	}
	providerID, providerType, org, verifier, ok := store.ConsumeState("state789")
	if !ok {
		t.Fatal("ConsumeState returned ok=false")
	}
	if providerID != 2 {
		t.Errorf("providerID = %d, want %d", providerID, 2)
	}
	if providerType != "google" {
		t.Errorf("providerType = %q, want %q", providerType, "google")
	}
	if org != 5 {
		t.Errorf("org = %d, want 5", org)
	}
	if verifier != "v123" {
		t.Errorf("verifier = %q, want %q", verifier, "v123")
	}
 }

func TestStateExpiry(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	// Store with 0 time to simulate expiry immediately
	store.entries["expired"] = &stateEntry{
		ProviderID:   1,
		ProviderType: "oidc",
		OrgID:        0,
		ExpiresAt:    time.Now().Add(-time.Hour),
		CodeVerifier: "v",
	}
	_, _, _, _, ok := store.GetState("expired")
	if ok {
		t.Fatal("GetState returned ok=true for expired token")
	}
	_, _, _, _, ok = store.ConsumeState("expired")
	if ok {
		t.Fatal("ConsumeState returned ok=true for expired token")
	}
 }

func TestConsumeState_Removes(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	err := store.StoreState(1, "oidc", "state_rm", 0, "v")
	if err != nil {
		t.Fatalf("StoreState: %v", err)
	}
	// First consume: should succeed
	_, _, _, _, ok := store.ConsumeState("state_rm")
	if !ok {
		t.Fatal("first consume returned ok=false")
	}
	// Second consume: should fail (already removed)
	_, _, _, _, ok = store.ConsumeState("state_rm")
	if ok {
		t.Fatal("second consume returned ok=true after removal")
	}
	// GetState after consume: should fail
	_, _, _, _, ok = store.GetState("state_rm")
	if ok {
		t.Fatal("GetState returned ok=true after consume")
	}
 }