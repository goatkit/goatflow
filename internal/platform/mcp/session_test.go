package mcp

import (
	"testing"
	"time"
)

func TestSessionManager_CreateAndGet(t *testing.T) {
	sm := NewSessionManager(5 * time.Minute)
	bridge := NewAPIBridge()

	session := sm.Create(1, "admin", "Admin", bridge)
	if session == nil {
		t.Fatal("Expected session to be created")
	}
	if session.ID == "" {
		t.Error("Session should have a non-empty ID")
	}
	if session.UserID != 1 {
		t.Errorf("Session UserID = %d, want 1", session.UserID)
	}
	if session.Server == nil {
		t.Error("Session should have a Server")
	}

	// Get by ID
	got := sm.Get(session.ID)
	if got == nil {
		t.Fatal("Expected to retrieve session by ID")
	}
	if got.ID != session.ID {
		t.Errorf("Got session ID %q, want %q", got.ID, session.ID)
	}

	// Count
	if sm.Count() != 1 {
		t.Errorf("Session count = %d, want 1", sm.Count())
	}
}

func TestSessionManager_GetMissing(t *testing.T) {
	sm := NewSessionManager(5 * time.Minute)

	got := sm.Get("nonexistent")
	if got != nil {
		t.Error("Expected nil for missing session")
	}
}

func TestSessionManager_Delete(t *testing.T) {
	sm := NewSessionManager(5 * time.Minute)
	bridge := NewAPIBridge()

	session := sm.Create(1, "admin", "Admin", bridge)
	sm.Delete(session.ID)

	got := sm.Get(session.ID)
	if got != nil {
		t.Error("Expected nil after deletion")
	}
	if sm.Count() != 0 {
		t.Errorf("Session count = %d, want 0 after delete", sm.Count())
	}
}

func TestSessionManager_Cleanup(t *testing.T) {
	sm := NewSessionManager(10 * time.Millisecond)
	bridge := NewAPIBridge()

	sm.Create(1, "admin", "Admin", bridge)
	if sm.Count() != 1 {
		t.Fatalf("Expected 1 session, got %d", sm.Count())
	}

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)
	sm.cleanup()

	if sm.Count() != 0 {
		t.Errorf("Expected 0 sessions after cleanup, got %d", sm.Count())
	}
}

func TestSessionManager_MultipleSessions(t *testing.T) {
	sm := NewSessionManager(5 * time.Minute)
	bridge := NewAPIBridge()

	s1 := sm.Create(1, "user1", "Admin", bridge)
	s2 := sm.Create(2, "user2", "Agent", bridge)

	if sm.Count() != 2 {
		t.Errorf("Expected 2 sessions, got %d", sm.Count())
	}

	// Each session has unique ID
	if s1.ID == s2.ID {
		t.Error("Sessions should have unique IDs")
	}

	// Delete one
	sm.Delete(s1.ID)
	if sm.Count() != 1 {
		t.Errorf("Expected 1 session after delete, got %d", sm.Count())
	}

	// Other still accessible
	if sm.Get(s2.ID) == nil {
		t.Error("Second session should still be accessible")
	}
}

func TestNegotiateProtocolVersion(t *testing.T) {
	tests := []struct {
		clientVersion string
		want          string
	}{
		{ProtocolVersion, ProtocolVersion},
		{ProtocolVersion202503, ProtocolVersion202503},
		{"2023-01-01", ProtocolVersion}, // unknown → fallback
		{"", ProtocolVersion},           // empty → fallback
	}

	for _, tt := range tests {
		t.Run(tt.clientVersion, func(t *testing.T) {
			got := NegotiateProtocolVersion(tt.clientVersion)
			if got != tt.want {
				t.Errorf("NegotiateProtocolVersion(%q) = %q, want %q", tt.clientVersion, got, tt.want)
			}
		})
	}
}
