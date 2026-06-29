package pluginui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Test database not available")
	}
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'gk_plugin_ui'",
	).Scan(&count)
	if err != nil || count == 0 {
		t.Skipf("gk_plugin_ui table not found — run migration 000010")
	}
	return db
}

func cleanupTestUIs(t *testing.T, db *sql.DB, prefix string) {
	t.Helper()
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_plugin_ui WHERE full_id LIKE ?"), prefix+"%")
}

func TestRepository_CRUD(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestUIs(t, db, prefix) })

	cfg := json.RawMessage(`{"routes":[{"path":"/","handler":"home"}],"branding":{"app_name":"Test App","color":"#ff0000"}}`)

	t.Run("Create and GetByFullID", func(t *testing.T) {
		fullID := prefix + "myapp"
		u := &PluginUI{
			PluginName: prefix + "testplugin",
			UIID:       "myapp",
			FullID:     fullID,
			Name:       "My Test App",
			UIType:     TypeCustomerApp,
			Shell:      ShellMinimal,
			Config:     &cfg,
			Enabled:    true,
			ValidID:    1,
		}

		id, err := repo.Create(u, 1)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if id == 0 {
			t.Fatal("expected non-zero ID")
		}

		got, err := repo.GetByFullID(fullID)
		if err != nil {
			t.Fatalf("GetByFullID: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil result")
		}
		if got.Name != "My Test App" {
			t.Errorf("name = %q", got.Name)
		}
		if got.UIType != TypeCustomerApp {
			t.Errorf("ui_type = %q", got.UIType)
		}
		if got.Shell != ShellMinimal {
			t.Errorf("shell = %q", got.Shell)
		}
		if !got.Enabled {
			t.Error("expected enabled")
		}
		if got.BasePath() != "/ui/"+fullID+"/" {
			t.Errorf("BasePath() = %q", got.BasePath())
		}

		// GetByID
		byID, err := repo.GetByID(id)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if byID == nil || byID.FullID != fullID {
			t.Error("GetByID returned wrong result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		fullID := prefix + "updateme"
		repo.Create(&PluginUI{
			PluginName: prefix + "testplugin", UIID: "updateme", FullID: fullID,
			Name: "Before", UIType: TypeAgentApp, Shell: ShellStandard,
			Enabled: true, ValidID: 1,
		}, 1)

		got, _ := repo.GetByFullID(fullID)
		got.Name = "After"
		got.Shell = ShellMinimal
		desc := "Updated description"
		got.Description = &desc

		err := repo.Update(got, 1)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}

		updated, _ := repo.GetByFullID(fullID)
		if updated.Name != "After" {
			t.Errorf("name = %q", updated.Name)
		}
		if updated.Shell != ShellMinimal {
			t.Errorf("shell = %q", updated.Shell)
		}
	})

	t.Run("SetEnabled", func(t *testing.T) {
		fullID := prefix + "toggleme"
		repo.Create(&PluginUI{
			PluginName: prefix + "testplugin", UIID: "toggleme", FullID: fullID,
			Name: "Toggle", UIType: TypeAgentApp, Shell: ShellStandard,
			Enabled: true, ValidID: 1,
		}, 1)

		got, _ := repo.GetByFullID(fullID)
		err := repo.SetEnabled(got.ID, false, 1)
		if err != nil {
			t.Fatal(err)
		}

		disabled, _ := repo.GetByFullID(fullID)
		if disabled.Enabled {
			t.Error("expected disabled")
		}

		repo.SetEnabled(got.ID, true, 1)
		reenabled, _ := repo.GetByFullID(fullID)
		if !reenabled.Enabled {
			t.Error("expected re-enabled")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		fullID := prefix + "deleteme"
		id, _ := repo.Create(&PluginUI{
			PluginName: prefix + "testplugin", UIID: "deleteme", FullID: fullID,
			Name: "Delete Me", UIType: TypePublicPage, Shell: ShellMinimal,
			Enabled: true, ValidID: 1,
		}, 1)

		err := repo.Delete(id)
		if err != nil {
			t.Fatal(err)
		}

		gone, _ := repo.GetByFullID(fullID)
		if gone != nil {
			t.Error("expected nil after delete")
		}
	})

	t.Run("GetByFullID not found", func(t *testing.T) {
		got, err := repo.GetByFullID("nonexistent_xyz_123")
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Error("expected nil")
		}
	})
}

func TestRepository_List(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestUIs(t, db, prefix) })

	// Create test UIs.
	repo.Create(&PluginUI{
		PluginName: prefix + "alpha", UIID: "app", FullID: prefix + "alpha_app",
		Name: "Alpha App", UIType: TypeCustomerApp, Shell: ShellMinimal,
		Enabled: true, ValidID: 1,
	}, 1)
	repo.Create(&PluginUI{
		PluginName: prefix + "alpha", UIID: "admin", FullID: prefix + "alpha_admin",
		Name: "Alpha Admin", UIType: TypeAdminPage, Shell: ShellStandard,
		Enabled: true, ValidID: 1,
	}, 1)
	repo.Create(&PluginUI{
		PluginName: prefix + "beta", UIID: "app", FullID: prefix + "beta_app",
		Name: "Beta App", UIType: TypeAgentApp, Shell: ShellStandard,
		Enabled: false, ValidID: 1,
	}, 1)

	t.Run("list all", func(t *testing.T) {
		uis, err := repo.List(prefix+"alpha", "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(uis) != 2 {
			t.Errorf("expected 2 UIs for alpha, got %d", len(uis))
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		uis, err := repo.List("", TypeCustomerApp, false)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, u := range uis {
			if u.FullID == prefix+"alpha_app" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find alpha_app in customer_app filter")
		}
	})

	t.Run("enabled only excludes disabled", func(t *testing.T) {
		uis, err := repo.List(prefix+"beta", "", true)
		if err != nil {
			t.Fatal(err)
		}
		if len(uis) != 0 {
			t.Errorf("expected 0 enabled beta UIs, got %d", len(uis))
		}
	})

	t.Run("ListActive", func(t *testing.T) {
		uis, err := repo.ListActive()
		if err != nil {
			t.Fatal(err)
		}
		// Should include alpha UIs but not beta (disabled).
		for _, u := range uis {
			if !u.Enabled || u.ValidID != 1 {
				t.Errorf("ListActive returned inactive UI: %s (enabled=%v, valid_id=%d)", u.FullID, u.Enabled, u.ValidID)
			}
		}
	})
}
