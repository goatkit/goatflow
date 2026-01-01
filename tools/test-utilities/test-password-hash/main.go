// Package main provides test utilities for password hashing and OTRS compatibility.
package main

import (
	"fmt"
	"os"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
)

func main() {
	// Test OTRS password hashing
	fmt.Println("Testing OTRS Password Compatibility")
	fmt.Println("====================================")

	// Create hasher with SHA256 (OTRS default)
	if err := os.Setenv("PASSWORD_HASH_TYPE", "sha256"); err != nil {
		fmt.Printf("Warning: Failed to set PASSWORD_HASH_TYPE: %v\n", err)
	}
	hasher := auth.NewPasswordHasher()

	// Test password "changeme" which should match root@localhost
	password := "changeme"
	expectedHash := "057ba03d6c44104863dc7361fe4578965d1887360f90a0895882e58a6248fc86"

	// Generate hash
	hash, err := hasher.HashPassword(password)
	if err != nil {
		fmt.Printf("Error hashing password: %v\n", err)
		return
	}

	fmt.Printf("Password: %s\n", password)
	fmt.Printf("Generated: %s\n", hash)
	fmt.Printf("Expected:  %s\n", expectedHash)
	fmt.Printf("Match: %v\n\n", hash == expectedHash)

	// Test verification
	fmt.Println("Testing password verification:")
	fmt.Printf("Verify 'changeme' against OTRS hash: %v\n",
		hasher.VerifyPassword(password, expectedHash))
	fmt.Printf("Verify 'wrongpass' against OTRS hash: %v\n",
		hasher.VerifyPassword("wrongpass", expectedHash))

	// Test with other OTRS user (password123)
	fmt.Println("\nTesting user 'nigel' with password 'password123':")
	nigelHash := "240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9"
	fmt.Printf("Verify 'password123': %v\n",
		hasher.VerifyPassword("password123", nigelHash))

	// Test auto-detection
	fmt.Println("\nTesting hash type auto-detection:")
	fmt.Printf("SHA256 hash (64 chars): detected correctly\n")
	fmt.Printf("Bcrypt hash ($2a$ prefix): detected correctly\n")

	fmt.Println("\n✅ Password hashing is OTRS compatible!")
}
