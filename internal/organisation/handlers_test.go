package organisation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *Repository) {
	t.Helper()
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	return eng, repo
}

func TestHandleSwitchOrg(t *testing.T) {
	eng, repo := setupTestRouter(t)
	prefix := fmt.Sprintf("test-%d-", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestOrgs(t, repo.db, prefix) })

	// Create an org and add user 1 as member.
	orgID, _ := repo.CreateOrg(&Organisation{
		Name: "Switch Test", Slug: prefix + "switch", Status: StatusActive, ValidID: 1,
	}, 1)
	userID := 1
	repo.AddMember(&UserOrganisation{OrgID: orgID, UserID: &userID, Role: RoleMember}, 1)

	eng.POST("/api/v1/session/org", func(c *gin.Context) {
		c.Set("user_id", 1)
	}, HandleSwitchOrg(repo))

	t.Run("switch to member org succeeds", func(t *testing.T) {
		body := fmt.Sprintf(`{"org_id":%d}`, orgID)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/session/org", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["status"] != "switched" {
			t.Errorf("resp = %v", resp)
		}
	})

	t.Run("switch to non-member org forbidden", func(t *testing.T) {
		otherID, _ := repo.CreateOrg(&Organisation{
			Name: "Other Org", Slug: prefix + "other", Status: StatusActive, ValidID: 1,
		}, 1)
		body := fmt.Sprintf(`{"org_id":%d}`, otherID)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/session/org", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
	})

	t.Run("switch to nonexistent org returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/session/org", strings.NewReader(`{"org_id":999999}`))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("missing org_id returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/session/org", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestHandleListUserOrgs(t *testing.T) {
	eng, repo := setupTestRouter(t)
	prefix := fmt.Sprintf("test-%d-", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestOrgs(t, repo.db, prefix) })

	orgID, _ := repo.CreateOrg(&Organisation{
		Name: "List Test", Slug: prefix + "listtest", Status: StatusActive, ValidID: 1,
	}, 1)
	userID := 1
	repo.AddMember(&UserOrganisation{OrgID: orgID, UserID: &userID, Role: RoleMember, IsDefault: true}, 1)

	eng.GET("/api/v1/session/orgs", func(c *gin.Context) {
		c.Set("user_id", 1)
		setOrgContext(c, orgID)
	}, HandleListUserOrgs(repo))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/session/orgs", nil)
	eng.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	orgs, ok := resp["organisations"].([]any)
	if !ok {
		t.Fatal("expected organisations array")
	}
	found := false
	for _, o := range orgs {
		org := o.(map[string]any)
		if org["name"] == "List Test" {
			found = true
			if org["active"] != true {
				t.Error("expected active=true for current org")
			}
		}
	}
	if !found {
		t.Error("expected to find List Test org")
	}
}

func TestHandleAdminOrgCRUD(t *testing.T) {
	eng, repo := setupTestRouter(t)
	prefix := fmt.Sprintf("test-%d-", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestOrgs(t, repo.db, prefix) })

	authMiddleware := func(c *gin.Context) { c.Set("user_id", 1) }

	eng.POST("/admin/api/organisations", authMiddleware, HandleAdminCreateOrg(repo))
	eng.PUT("/admin/api/organisations/:id", authMiddleware, HandleAdminUpdateOrg(repo))
	eng.DELETE("/admin/api/organisations/:id", authMiddleware, HandleAdminDeleteOrg(repo))
	eng.GET("/admin/api/organisations", authMiddleware, HandleAdminListOrgs(repo))
	eng.GET("/admin/api/organisations/:id/members", authMiddleware, HandleAdminListMembers(repo))
	eng.POST("/admin/api/organisations/:id/members", authMiddleware, HandleAdminAddMember(repo))
	eng.DELETE("/admin/api/organisations/:id/members/:member_id", authMiddleware, HandleAdminRemoveMember(repo))
	eng.GET("/admin/api/organisations/:id/config", authMiddleware, HandleAdminListOrgConfigs(repo))
	eng.PUT("/admin/api/organisations/:id/config", authMiddleware, HandleAdminSetOrgConfig(repo))

	t.Run("create org", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"Handler Test","slug":"%shandler"}`, prefix)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/organisations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("create org invalid slug", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/organisations", strings.NewReader(`{"name":"Bad","slug":"BAD SLUG"}`))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("create org duplicate slug", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"Dupe","slug":"%shandler"}`, prefix)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/organisations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", w.Code)
		}
	})

	t.Run("list orgs", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/api/organisations", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("update org", func(t *testing.T) {
		org, _ := repo.GetOrgBySlug(prefix + "handler")
		body := `{"name":"Updated Name","status":"suspended"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/admin/api/organisations/%d", org.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("add member", func(t *testing.T) {
		org, _ := repo.GetOrgBySlug(prefix + "handler")
		body := `{"user_id":1,"role":"admin"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", fmt.Sprintf("/admin/api/organisations/%d/members", org.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("list members", func(t *testing.T) {
		org, _ := repo.GetOrgBySlug(prefix + "handler")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/admin/api/organisations/%d/members", org.ID), nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		members := resp["members"].([]any)
		if len(members) != 1 {
			t.Errorf("expected 1 member, got %d", len(members))
		}
	})

	t.Run("set and list org config", func(t *testing.T) {
		org, _ := repo.GetOrgBySlug(prefix + "handler")

		// Set config.
		body := `{"name":"Branding::AppName","value":"My Custom App"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/admin/api/organisations/%d/config", org.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("set config: status = %d", w.Code)
		}

		// List configs.
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", fmt.Sprintf("/admin/api/organisations/%d/config", org.ID), nil)
		eng.ServeHTTP(w2, req2)
		if w2.Code != http.StatusOK {
			t.Fatalf("list config: status = %d", w2.Code)
		}
		var resp map[string]any
		json.Unmarshal(w2.Body.Bytes(), &resp)
		config := resp["config"].(map[string]any)
		if config["Branding::AppName"] != "My Custom App" {
			t.Errorf("config = %v", config)
		}
	})

	t.Run("delete org", func(t *testing.T) {
		org, _ := repo.GetOrgBySlug(prefix + "handler")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/admin/api/organisations/%d", org.ID), nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d", w.Code)
		}
	})
}

func TestMiddleware(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test-%d-", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestOrgs(t, repo.db, prefix) })

	orgID, _ := repo.CreateOrg(&Organisation{
		Name: "MW Test", Slug: prefix + "mw", Status: StatusActive, ValidID: 1,
	}, 1)
	userID := 1
	repo.AddMember(&UserOrganisation{OrgID: orgID, UserID: &userID, Role: RoleMember, IsDefault: true}, 1)

	gin.SetMode(gin.TestMode)

	t.Run("resolves from cookie", func(t *testing.T) {
		eng := gin.New()
		eng.Use(func(c *gin.Context) { c.Set("user_id", 1) })
		eng.Use(Middleware(repo))
		var capturedOrgID int64
		eng.GET("/test", func(c *gin.Context) {
			capturedOrgID = ActiveOrgFromGin(c)
			c.String(200, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "active_org_id", Value: fmt.Sprintf("%d", orgID)})
		eng.ServeHTTP(w, req)

		if capturedOrgID != orgID {
			t.Errorf("org from cookie = %d, want %d", capturedOrgID, orgID)
		}
	})

	t.Run("falls back to default org", func(t *testing.T) {
		eng := gin.New()
		eng.Use(func(c *gin.Context) { c.Set("user_id", 1) })
		eng.Use(Middleware(repo))
		var capturedOrgID int64
		eng.GET("/test", func(c *gin.Context) {
			capturedOrgID = ActiveOrgFromGin(c)
			c.String(200, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		// No cookie — should resolve from default org.
		eng.ServeHTTP(w, req)

		if capturedOrgID != orgID {
			t.Errorf("org from default = %d, want %d", capturedOrgID, orgID)
		}
	})

	t.Run("no org context when no membership", func(t *testing.T) {
		eng := gin.New()
		eng.Use(func(c *gin.Context) { c.Set("user_id", 99999) }) // user with no membership
		eng.Use(Middleware(repo))
		var capturedOrgID int64
		eng.GET("/test", func(c *gin.Context) {
			capturedOrgID = ActiveOrgFromGin(c)
			c.String(200, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		eng.ServeHTTP(w, req)

		if capturedOrgID != 0 {
			t.Errorf("expected 0 (no org), got %d", capturedOrgID)
		}
	})
}

func TestIsValidSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"acme", true},
		{"acme-corp", true},
		{"my-org-123", true},
		{"a", true},
		{"UPPER", false},
		{"has spaces", false},
		{"-starts-with-dash", false},
		{"ends-with-dash-", false},
		{"", false},
		{"has_underscore", false},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			if got := isValidSlug(tt.slug); got != tt.want {
				t.Errorf("isValidSlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}
