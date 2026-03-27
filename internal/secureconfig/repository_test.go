package secureconfig

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Test database not available")
	}
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'gk_secure_config'",
	).Scan(&count)
	if err != nil || count == 0 {
		t.Skipf("gk_secure_config table not found — run migration 000012")
	}
	return db
}

func cleanupTestSecrets(t *testing.T, db *sql.DB, prefix string) {
	t.Helper()
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_secure_config WHERE plugin_name LIKE ?"), prefix+"%")
}

func TestRepository_SetAndGet(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestSecrets(t, db, prefix) })

	key := make([]byte, 32)
	rand.Read(key)
	SetKey(key)
	defer SetKey(nil)

	pluginName := prefix + "inventory"
	secretName := "api_key"
	secretValue := "sk_live_test1234567890"

	// Encrypt and store.
	encrypted, err := Encrypt([]byte(secretValue), key)
	if err != nil {
		t.Fatal(err)
	}
	hint := ValueHint(secretValue)

	err = repo.Set(pluginName, secretName, encrypted, hint, 0, 1)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Retrieve and decrypt.
	entry, err := repo.Get(pluginName, secretName, 0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	decrypted, err := Decrypt(entry.EncryptedValue, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != secretValue {
		t.Errorf("decrypted = %q, want %q", string(decrypted), secretValue)
	}
	if entry.ValueHint == nil || *entry.ValueHint != "7890" {
		t.Errorf("hint = %v, want '7890'", entry.ValueHint)
	}
}

func TestRepository_SetUpdatesExisting(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestSecrets(t, db, prefix) })

	key := make([]byte, 32)
	rand.Read(key)

	pluginName := prefix + "inventory"
	enc1, _ := Encrypt([]byte("value1"), key)
	enc2, _ := Encrypt([]byte("value2"), key)

	repo.Set(pluginName, "key1", enc1, "lue1", 0, 1)
	repo.Set(pluginName, "key1", enc2, "lue2", 0, 1)

	entry, _ := repo.Get(pluginName, "key1", 0)
	if entry == nil {
		t.Fatal("expected entry")
	}
	decrypted, _ := Decrypt(entry.EncryptedValue, key)
	if string(decrypted) != "value2" {
		t.Errorf("expected updated value, got %q", string(decrypted))
	}
	if entry.ValueHint == nil || *entry.ValueHint != "lue2" {
		t.Errorf("hint should be updated: %v", entry.ValueHint)
	}
}

func TestRepository_OrgScoping(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestSecrets(t, db, prefix) })

	key := make([]byte, 32)
	rand.Read(key)

	pluginName := prefix + "inventory"

	// Set global value.
	encGlobal, _ := Encrypt([]byte("global_key"), key)
	repo.Set(pluginName, "api_key", encGlobal, "key!", 0, 1)

	// Set org-specific value.
	encOrg, _ := Encrypt([]byte("org42_key"), key)
	repo.Set(pluginName, "api_key", encOrg, "key!", 42, 1)

	t.Run("org-specific returns org value", func(t *testing.T) {
		entry, _ := repo.Get(pluginName, "api_key", 42)
		if entry == nil {
			t.Fatal("expected entry")
		}
		dec, _ := Decrypt(entry.EncryptedValue, key)
		if string(dec) != "org42_key" {
			t.Errorf("expected org value, got %q", string(dec))
		}
	})

	t.Run("different org falls back to global", func(t *testing.T) {
		entry, _ := repo.Get(pluginName, "api_key", 99)
		if entry == nil {
			t.Fatal("expected global fallback")
		}
		dec, _ := Decrypt(entry.EncryptedValue, key)
		if string(dec) != "global_key" {
			t.Errorf("expected global value, got %q", string(dec))
		}
	})

	t.Run("no org returns global", func(t *testing.T) {
		entry, _ := repo.Get(pluginName, "api_key", 0)
		if entry == nil {
			t.Fatal("expected global")
		}
		dec, _ := Decrypt(entry.EncryptedValue, key)
		if string(dec) != "global_key" {
			t.Errorf("expected global, got %q", string(dec))
		}
	})
}

func TestRepository_Delete(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestSecrets(t, db, prefix) })

	key := make([]byte, 32)
	rand.Read(key)

	pluginName := prefix + "inventory"
	enc, _ := Encrypt([]byte("delete_me"), key)
	repo.Set(pluginName, "temp", enc, "e_me", 0, 1)

	err := repo.Delete(pluginName, "temp", 0)
	if err != nil {
		t.Fatal(err)
	}

	entry, _ := repo.Get(pluginName, "temp", 0)
	if entry != nil {
		t.Error("expected nil after delete")
	}
}

func TestRepository_ListForPlugin(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestSecrets(t, db, prefix) })

	key := make([]byte, 32)
	rand.Read(key)

	pluginName := prefix + "inventory"
	enc1, _ := Encrypt([]byte("val1"), key)
	enc2, _ := Encrypt([]byte("val2"), key)
	repo.Set(pluginName, "key_a", enc1, "val1", 0, 1)
	repo.Set(pluginName, "key_b", enc2, "val2", 0, 1)

	entries, err := repo.ListForPlugin(pluginName)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)

	entry, err := repo.Get("nonexistent_plugin_xyz", "nonexistent_key", 0)
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Error("expected nil for nonexistent key")
	}
}
