// Package main — first-boot admin bootstrap.
//
// This file implements the one-shot admin bootstrap that consumes the
// GOATFLOW_ADMIN_PASSWORD environment variable.
//
// Background: migration 000002 seeds the system admin (root@localhost) with a
// random OTRS-compatible password and valid_id = 2 (disabled), so no
// credential exists that a new install can log in with. Deployments that set
// GOATFLOW_ADMIN_PASSWORD (e.g. the TrueNAS community app, where it comes from
// the "Admin Password" field in the install wizard) expect that value to become
// the initial admin password automatically.
//
// Safety model — the bootstrap is strictly first-boot-only:
//   - It only runs when the seeded admin row still exists AND is in its
//     factory-disabled state (valid_id <> 1). The UPDATE itself carries the
//     same condition, so even a race between the check and the write cannot
//     clobber a live account.
//   - After a successful application it records a marker
//     (sysconfig_modified: admin.bootstrap.applied = "true"). If the
//     environment variable is still set afterwards (or a stale wizard value
//     lingers after the user changed the password via the UI), every future
//     startup is a no-op.
//   - The marker is written only AFTER the user update succeeded, so a failed
//     or interrupted run retries on the next boot.
//
// Failure semantics: any error here is logged and ignored — the server still
// starts, exactly like a migration warning. A failed bootstrap must never take
// the app down.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/database"
	"golang.org/x/crypto/bcrypt"
)

// bootstrapAdminLogin is the system admin account seeded by migration 000002
// (OTRS convention).
const bootstrapAdminLogin = "root@localhost"

// bootstrapMarkerName is the sysconfig flag that records a successful bootstrap
// so it never runs twice.
const bootstrapMarkerName = "admin.bootstrap.applied"

// bootstrapAdminFromEnv applies the first-boot admin bootstrap on the server
// startup path, right after migrations. It is a no-op unless
// GOATFLOW_ADMIN_PASSWORD is set and the admin account is still factory
// disabled. Safe to call with a nil db.
func bootstrapAdminFromEnv(db *sql.DB) {
	if db == nil {
		return
	}

	adminPassword := strings.TrimSpace(os.Getenv("GOATFLOW_ADMIN_PASSWORD"))
	if adminPassword == "" {
		return // not a bootstrap deployment (or nothing configured)
	}

	// 1. Already applied? The marker is authoritative: once the bootstrap has
	// succeeded, the env var is ignored for good. This also covers the case
	// where an operator later disables the admin account in the UI — we must
	// not re-enable it on the next restart.
	if adminBootstrapMarkerSet(db) {
		log.Printf("ℹ️  Admin bootstrap already applied; ignoring GOATFLOW_ADMIN_PASSWORD (admin credentials are managed in the app)")
		return
	}

	// 2. Guard: the seeded admin row must exist and still be in its factory
	// disabled state (valid_id <> 1). If the account has been enabled — by any
	// means — we refuse to touch it.
	var validID int
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT valid_id FROM users WHERE login = ? LIMIT 1",
	), bootstrapAdminLogin).Scan(&validID)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("ℹ️  Admin bootstrap skipped: user %q not found yet", bootstrapAdminLogin)
		return
	case err != nil:
		log.Printf("⚠️  Admin bootstrap skipped: could not read user state: %v", err)
		return
	}
	if validID == 1 {
		log.Printf("ℹ️  Admin bootstrap skipped: %q is already enabled; the wizard-provided password will not be applied", bootstrapAdminLogin)
		return
	}

	// 3. Apply. The WHERE clause repeats the guard so the check-then-act cannot
	// race: only a still-disabled admin row is ever updated.
	hashed, err := hashForBootstrap(adminPassword)
	if err != nil {
		log.Printf("⚠️  Admin bootstrap failed: %v", err)
		return
	}
	res, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE users
		SET pw = ?, valid_id = 1, change_time = NOW(), change_by = 1
		WHERE login = ? AND valid_id <> 1`), hashed, bootstrapAdminLogin)
	if err != nil {
		log.Printf("⚠️  Admin bootstrap failed: %v", err)
		return
	}
	affected, err := res.RowsAffected()
	if err != nil {
		log.Printf("⚠️  Admin bootstrap failed: %v", err)
		return
	}
	if affected == 0 {
		log.Printf("ℹ️  Admin bootstrap skipped: %q no longer in factory-disabled state", bootstrapAdminLogin)
		return
	}

	// 4. Record the marker so this never runs again. A marker failure is not
	// fatal: on the next boot the valid_id guard (step 2) makes the bootstrap a
	// no-op anyway, and the repeated log line is visible.
	if err := setAdminBootstrapMarker(db); err != nil {
		log.Printf("⚠️  Admin bootstrap applied but marker write failed (will be a no-op on next start): %v", err)
		return
	}

	log.Printf("✅ First-boot admin bootstrap complete: %q enabled with the password from GOATFLOW_ADMIN_PASSWORD (one-shot; ignored on future starts)", bootstrapAdminLogin)
}

// adminBootstrapMarkerSet reports whether a previous bootstrap already
// succeeded.
func adminBootstrapMarkerSet(db *sql.DB) bool {
	var value sql.NullString
	err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT effective_value FROM sysconfig_modified
         WHERE name = ? AND is_valid = 1
         ORDER BY change_time DESC
         LIMIT 1`,
	), bootstrapMarkerName).Scan(&value)
	if err != nil {
		// Absence of the marker is the expected first-boot case; a real error
		// just means "not found" for our purposes (the valid_id guard still
		// protects the account).
		return false
	}
	return value.Valid && strings.EqualFold(strings.TrimSpace(value.String), "true")
}

// setAdminBootstrapMarker persists the one-shot marker. The marker row must
// carry a valid sysconfig_default_id (FK), so we attach it to the
// setup.assistant.completed default (seeded by migration 000026) or, if that
// row is missing, to any existing sysconfig_default row.
func setAdminBootstrapMarker(db *sql.DB) error {
	var defaultID int
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM sysconfig_default WHERE name = ? LIMIT 1",
	), "setup.assistant.completed").Scan(&defaultID)
	if err != nil {
		// Fall back to any sysconfig_default row (the marker's own meaning is
		// independent of which default it points at).
		if fallbackErr := db.QueryRow("SELECT id FROM sysconfig_default ORDER BY id LIMIT 1").Scan(&defaultID); fallbackErr != nil {
			return fmt.Errorf("write bootstrap marker: no sysconfig_default row available: %w", fallbackErr)
		}
	}

	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO sysconfig_modified
			(sysconfig_default_id, name, user_id, is_valid, user_modification_active,
			 effective_value, is_dirty, reset_to_default,
			 create_time, create_by, change_time, change_by)
		VALUES (?, ?, NULL, 1, 0, 'true', 0, 0, NOW(), 1, NOW(), 1)`), defaultID, bootstrapMarkerName)
	if err != nil {
		return fmt.Errorf("write bootstrap marker: %w", err)
	}
	return nil
}

// hashForBootstrap hashes the bootstrap password. Indirection keeps the
// hashing scheme in one place and lets tests observe/skip the bcrypt call.
func hashForBootstrap(password string) (string, error) {
	return bcryptHash(password)
}

// bcryptHash is the concrete hashing implementation. It produces a bcrypt
// hash ($2a$...), which both the modern auth.PasswordHasher.VerifyPassword and
// the legacy OTRS verifyPassword path accept (they auto-detect the bcrypt
// prefix). Splitting it out keeps hashForBootstrap trivially testable.
func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(hash), nil
}
