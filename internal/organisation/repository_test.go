package organisation

import (
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
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'gk_organisation'",
	).Scan(&count)
	if err != nil || count == 0 {
		t.Skipf("gk_organisation table not found — run migration 000011")
	}
	return db
}

func cleanupTestOrgs(t *testing.T, db *sql.DB, prefix string) {
	t.Helper()
	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	db.Exec(database.ConvertPlaceholders("DELETE FROM sysconfig_org WHERE org_id IN (SELECT id FROM gk_organisation WHERE slug LIKE ?)"), prefix+"%")
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_user_organisation WHERE org_id IN (SELECT id FROM gk_organisation WHERE slug LIKE ?)"), prefix+"%")
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_organisation WHERE slug LIKE ?"), prefix+"%")
	db.Exec("SET FOREIGN_KEY_CHECKS=1")
}

func TestRepository_OrgCRUD(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestOrgs(t, db, prefix) })

	t.Run("CreateOrg and GetOrg", func(t *testing.T) {
		org := &Organisation{
			Name:    "Acme Corp",
			Slug:    prefix + "acme",
			Status:  StatusActive,
			ValidID: 1,
		}

		id, err := repo.CreateOrg(org, 1)
		if err != nil {
			t.Fatalf("CreateOrg: %v", err)
		}
		if id == 0 {
			t.Fatal("expected non-zero ID")
		}

		got, err := repo.GetOrg(id)
		if err != nil {
			t.Fatalf("GetOrg: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil org")
		}
		if got.Name != "Acme Corp" {
			t.Errorf("name = %q", got.Name)
		}
		if got.Slug != prefix+"acme" {
			t.Errorf("slug = %q", got.Slug)
		}
		if got.Status != StatusActive {
			t.Errorf("status = %q", got.Status)
		}
		if !got.IsActive() {
			t.Error("expected active")
		}
	})

	t.Run("GetOrgBySlug", func(t *testing.T) {
		repo.CreateOrg(&Organisation{Name: "Beta Ltd", Slug: prefix + "beta", Status: StatusActive, ValidID: 1}, 1)

		got, err := repo.GetOrgBySlug(prefix + "beta")
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || got.Name != "Beta Ltd" {
			t.Error("expected Beta Ltd")
		}
	})

	t.Run("GetOrgBySlug not found", func(t *testing.T) {
		got, err := repo.GetOrgBySlug("nonexistent_slug_xyz")
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Error("expected nil")
		}
	})

	t.Run("UpdateOrg", func(t *testing.T) {
		id, _ := repo.CreateOrg(&Organisation{Name: "Gamma", Slug: prefix + "gamma", Status: StatusActive, ValidID: 1}, 1)
		got, _ := repo.GetOrg(id)
		got.Name = "Gamma Updated"
		got.Status = StatusSuspended

		err := repo.UpdateOrg(got, 1)
		if err != nil {
			t.Fatal(err)
		}

		updated, _ := repo.GetOrg(id)
		if updated.Name != "Gamma Updated" {
			t.Errorf("name = %q", updated.Name)
		}
		if updated.Status != StatusSuspended {
			t.Errorf("status = %q", updated.Status)
		}
		if updated.IsActive() {
			t.Error("suspended org should not be active")
		}
	})

	t.Run("DeleteOrg cascades memberships and config", func(t *testing.T) {
		id, _ := repo.CreateOrg(&Organisation{Name: "Delete Me", Slug: prefix + "delete", Status: StatusActive, ValidID: 1}, 1)

		// Add a membership and config.
		userID := 1
		repo.AddMember(&UserOrganisation{OrgID: id, UserID: &userID, Role: RoleMember}, 1)
		repo.SetOrgConfig(id, "test.key", []byte("test_value"), 1)

		err := repo.DeleteOrg(id)
		if err != nil {
			t.Fatal(err)
		}

		gone, _ := repo.GetOrg(id)
		if gone != nil {
			t.Error("org should be gone")
		}
		// Memberships and config should cascade.
		members, _ := repo.ListMembers(id)
		if len(members) != 0 {
			t.Error("memberships should be cascaded")
		}
		val, _ := repo.GetOrgConfig(id, "test.key")
		if val != nil {
			t.Error("config should be cascaded")
		}
	})

	t.Run("ListOrgs", func(t *testing.T) {
		repo.CreateOrg(&Organisation{Name: "List A", Slug: prefix + "lista", Status: StatusActive, ValidID: 1}, 1)
		repo.CreateOrg(&Organisation{Name: "List B", Slug: prefix + "listb", Status: StatusSuspended, ValidID: 1}, 1)

		all, err := repo.ListOrgs("", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(all) < 2 {
			t.Error("expected at least 2 orgs")
		}

		activeOnly, err := repo.ListOrgs("", true)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range activeOnly {
			if o.Status != StatusActive {
				t.Errorf("ListOrgs(activeOnly) returned %q status", o.Status)
			}
		}

		suspended, err := repo.ListOrgs(StatusSuspended, false)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, o := range suspended {
			if o.Slug == prefix+"listb" {
				found = true
			}
		}
		if !found {
			t.Error("expected listb in suspended filter")
		}
	})
}

func TestRepository_Membership(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestOrgs(t, db, prefix) })

	orgID, _ := repo.CreateOrg(&Organisation{Name: "Membership Test", Slug: prefix + "memtest", Status: StatusActive, ValidID: 1}, 1)

	t.Run("AddMember agent", func(t *testing.T) {
		userID := 1
		id, err := repo.AddMember(&UserOrganisation{
			OrgID: orgID, UserID: &userID, Role: RoleAdmin, IsDefault: true,
		}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if id == 0 {
			t.Fatal("expected non-zero membership ID")
		}
	})

	t.Run("AddMember customer", func(t *testing.T) {
		login := "customer@example.com"
		_, err := repo.AddMember(&UserOrganisation{
			OrgID: orgID, CustomerLogin: &login, Role: RoleMember,
		}, 1)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ListMembers", func(t *testing.T) {
		members, err := repo.ListMembers(orgID)
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != 2 {
			t.Errorf("expected 2 members, got %d", len(members))
		}
	})

	t.Run("GetUserOrgs", func(t *testing.T) {
		orgs, err := repo.GetUserOrgs(1)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, o := range orgs {
			if o.ID == orgID {
				found = true
			}
		}
		if !found {
			t.Error("expected org in user's org list")
		}
	})

	t.Run("GetDefaultOrgForUser", func(t *testing.T) {
		org, err := repo.GetDefaultOrgForUser(1)
		if err != nil {
			t.Fatal(err)
		}
		if org == nil {
			t.Fatal("expected default org")
		}
		if org.ID != orgID {
			t.Errorf("default org id = %d, want %d", org.ID, orgID)
		}
	})

	t.Run("SetDefaultOrg", func(t *testing.T) {
		org2ID, _ := repo.CreateOrg(&Organisation{Name: "Org 2", Slug: prefix + "org2", Status: StatusActive, ValidID: 1}, 1)
		userID := 1
		repo.AddMember(&UserOrganisation{OrgID: org2ID, UserID: &userID, Role: RoleMember}, 1)

		err := repo.SetDefaultOrg(1, org2ID)
		if err != nil {
			t.Fatal(err)
		}

		newDefault, _ := repo.GetDefaultOrgForUser(1)
		if newDefault == nil || newDefault.ID != org2ID {
			t.Error("default should now be org2")
		}
	})

	t.Run("RemoveMember", func(t *testing.T) {
		members, _ := repo.ListMembers(orgID)
		if len(members) == 0 {
			t.Skip("no members to remove")
		}
		err := repo.RemoveMember(members[0].ID)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestRepository_OrgConfig(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestOrgs(t, db, prefix) })

	orgID, _ := repo.CreateOrg(&Organisation{Name: "Config Test", Slug: prefix + "cfgtest", Status: StatusActive, ValidID: 1}, 1)

	t.Run("SetOrgConfig and GetOrgConfig", func(t *testing.T) {
		err := repo.SetOrgConfig(orgID, "Branding::AppName", []byte("Custom Name"), 1)
		if err != nil {
			t.Fatal(err)
		}

		val, err := repo.GetOrgConfig(orgID, "Branding::AppName")
		if err != nil {
			t.Fatal(err)
		}
		if string(val) != "Custom Name" {
			t.Errorf("value = %q, want 'Custom Name'", string(val))
		}
	})

	t.Run("SetOrgConfig updates existing", func(t *testing.T) {
		repo.SetOrgConfig(orgID, "Branding::AppName", []byte("Updated Name"), 1)

		val, _ := repo.GetOrgConfig(orgID, "Branding::AppName")
		if string(val) != "Updated Name" {
			t.Errorf("value = %q", string(val))
		}
	})

	t.Run("GetOrgConfig not set returns nil", func(t *testing.T) {
		val, err := repo.GetOrgConfig(orgID, "NonExistent::Key")
		if err != nil {
			t.Fatal(err)
		}
		if val != nil {
			t.Error("expected nil for unset config")
		}
	})

	t.Run("ListOrgConfigs", func(t *testing.T) {
		repo.SetOrgConfig(orgID, "SLA::ResponseHours", []byte("4"), 1)
		repo.SetOrgConfig(orgID, "SLA::ResolutionHours", []byte("24"), 1)

		configs, err := repo.ListOrgConfigs(orgID)
		if err != nil {
			t.Fatal(err)
		}
		if len(configs) < 2 {
			t.Errorf("expected at least 2 configs, got %d", len(configs))
		}
	})

	t.Run("DeleteOrgConfig", func(t *testing.T) {
		err := repo.DeleteOrgConfig(orgID, "SLA::ResponseHours")
		if err != nil {
			t.Fatal(err)
		}

		val, _ := repo.GetOrgConfig(orgID, "SLA::ResponseHours")
		if val != nil {
			t.Error("expected nil after delete")
		}
	})
}
