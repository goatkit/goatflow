package auth

import (
	"testing"
	"time"
)

func TestLoginRateLimiter_Basic(t *testing.T) {
	rl := NewLoginRateLimiter(3, 60, 1*time.Second, 10*time.Second)

	ip := "192.168.1.1"
	user := "testuser"

	// Should not be blocked initially
	blocked, _ := rl.IsBlocked(ip, user)
	if blocked {
		t.Error("should not be blocked initially")
	}

	// Record failures below threshold
	rl.RecordFailure(ip, user)
	rl.RecordFailure(ip, user)
	blocked, _ = rl.IsBlocked(ip, user)
	if blocked {
		t.Error("should not be blocked with 2 failures (threshold is 3)")
	}

	// Third failure should trigger block
	rl.RecordFailure(ip, user)
	blocked, remaining := rl.IsBlocked(ip, user)
	if !blocked {
		t.Error("should be blocked after 3 failures")
	}
	if remaining <= 0 {
		t.Error("remaining time should be positive")
	}
}

func TestLoginRateLimiter_DifferentUsers(t *testing.T) {
	rl := NewLoginRateLimiter(2, 60, 1*time.Second, 10*time.Second)

	ip := "192.168.1.1"

	// Block user1
	rl.RecordFailure(ip, "user1")
	rl.RecordFailure(ip, "user1")
	blocked1, _ := rl.IsBlocked(ip, "user1")

	// user2 should not be blocked
	blocked2, _ := rl.IsBlocked(ip, "user2")

	if !blocked1 {
		t.Error("user1 should be blocked")
	}
	if blocked2 {
		t.Error("user2 should not be blocked")
	}
}

func TestLoginRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewLoginRateLimiter(2, 60, 1*time.Second, 10*time.Second)

	user := "testuser"

	// Block from IP1
	rl.RecordFailure("1.1.1.1", user)
	rl.RecordFailure("1.1.1.1", user)
	blocked1, _ := rl.IsBlocked("1.1.1.1", user)

	// Same user from IP2 should not be blocked
	blocked2, _ := rl.IsBlocked("2.2.2.2", user)

	if !blocked1 {
		t.Error("should be blocked from IP1")
	}
	if blocked2 {
		t.Error("should not be blocked from IP2")
	}
}

func TestLoginRateLimiter_SuccessClearsRecord(t *testing.T) {
	rl := NewLoginRateLimiter(3, 60, 1*time.Second, 10*time.Second)

	ip := "192.168.1.1"
	user := "testuser"

	// Record some failures
	rl.RecordFailure(ip, user)
	rl.RecordFailure(ip, user)

	// Successful login clears record
	rl.RecordSuccess(ip, user)

	// Should be able to fail again without being blocked
	rl.RecordFailure(ip, user)
	rl.RecordFailure(ip, user)
	blocked, _ := rl.IsBlocked(ip, user)
	if blocked {
		t.Error("should not be blocked after success cleared record")
	}
}

func TestLoginRateLimiter_ExponentialBackoff(t *testing.T) {
	rl := NewLoginRateLimiter(2, 60, 1*time.Second, 30*time.Second)

	// Test backoff calculation
	backoff1 := rl.calculateBackoff(2) // At threshold
	backoff2 := rl.calculateBackoff(3) // 1 over
	backoff3 := rl.calculateBackoff(4) // 2 over

	if backoff1 != 1*time.Second {
		t.Errorf("expected 1s backoff at threshold, got %v", backoff1)
	}
	if backoff2 != 2*time.Second {
		t.Errorf("expected 2s backoff 1 over, got %v", backoff2)
	}
	if backoff3 != 4*time.Second {
		t.Errorf("expected 4s backoff 2 over, got %v", backoff3)
	}
}

func TestLoginRateLimiter_MaxBackoff(t *testing.T) {
	rl := NewLoginRateLimiter(2, 60, 1*time.Second, 5*time.Second)

	// Even with many failures, backoff should not exceed max
	backoff := rl.calculateBackoff(100)
	if backoff > 5*time.Second {
		t.Errorf("backoff should not exceed max, got %v", backoff)
	}
}

func TestLoginRateLimiter_Stats(t *testing.T) {
	rl := NewLoginRateLimiter(2, 60, 1*time.Second, 10*time.Second)

	// Record some activity
	rl.RecordFailure("1.1.1.1", "user1")
	rl.RecordFailure("1.1.1.1", "user1") // Block
	rl.RecordFailure("2.2.2.2", "user2")

	stats := rl.Stats()

	if stats["tracked_keys"].(int) != 2 {
		t.Errorf("expected 2 tracked keys, got %v", stats["tracked_keys"])
	}
	if stats["blocked_count"].(int) != 1 {
		t.Errorf("expected 1 blocked, got %v", stats["blocked_count"])
	}
}
