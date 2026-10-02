package auth

import (
	"testing"
	"time"
)

func TestStoreState(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	want := StateData{ProviderID: 1, ProviderType: "oidc", CodeVerifier: "verifier456", BrowserBinding: "hash"}
	if err := store.StoreState("state123", want); err != nil {
		t.Fatalf("StoreState: %v", err)
	}
	got, ok := store.GetState("state123")
	if !ok {
		t.Fatal("GetState returned ok=false")
	}
	if got != want {
		t.Errorf("GetState = %+v, want %+v", got, want)
	}
}

func TestStoreState_Duplicate(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	if err := store.StoreState("state123", StateData{ProviderID: 1, ProviderType: "oidc", CodeVerifier: "v"}); err != nil {
		t.Fatalf("first store: %v", err)
	}
	err := store.StoreState("state123", StateData{ProviderID: 1, ProviderType: "oidc", CodeVerifier: "v2"})
	if err == nil {
		t.Fatal("expected error for duplicate token")
	}
}

func TestGetState_NotFound(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	got, ok := store.GetState("nonexistent")
	if ok {
		t.Fatal("GetState returned ok=true for missing token")
	}
	if got != (StateData{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

func TestConsumeState(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	want := StateData{ProviderID: 2, ProviderType: "google", OrgID: 5, CodeVerifier: "v123"}
	if err := store.StoreState("state789", want); err != nil {
		t.Fatalf("StoreState: %v", err)
	}
	got, ok := store.ConsumeState("state789")
	if !ok {
		t.Fatal("ConsumeState returned ok=false")
	}
	if got != want {
		t.Errorf("ConsumeState = %+v, want %+v", got, want)
	}
}

func TestStateExpiry(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	store.entries["expired"] = &stateEntry{
		data:      StateData{ProviderID: 1, ProviderType: "oidc", CodeVerifier: "v"},
		expiresAt: time.Now().Add(-time.Hour),
	}
	if _, ok := store.GetState("expired"); ok {
		t.Fatal("GetState returned ok=true for expired token")
	}
	if _, ok := store.ConsumeState("expired"); ok {
		t.Fatal("ConsumeState returned ok=true for expired token")
	}
}

func TestConsumeState_Removes(t *testing.T) {
	t.Parallel()
	store := NewMemoryStateStore()
	if err := store.StoreState("state_rm", StateData{ProviderID: 1, ProviderType: "oidc", CodeVerifier: "v"}); err != nil {
		t.Fatalf("StoreState: %v", err)
	}
	if _, ok := store.ConsumeState("state_rm"); !ok {
		t.Fatal("first consume returned ok=false")
	}
	if _, ok := store.ConsumeState("state_rm"); ok {
		t.Fatal("second consume returned ok=true after removal")
	}
	if _, ok := store.GetState("state_rm"); ok {
		t.Fatal("GetState returned ok=true after consume")
	}
}
