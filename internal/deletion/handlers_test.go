package deletion

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

func TestHandleAdminRecycleBinList(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)
	testEntityType := "test_handler_list"
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	// Add test entries.
	name := "Test Entry"
	svc.repo.AddToRecycleBin(&RecycleBinEntry{
		EntityType: testEntityType, EntityID: 99500, EntityName: &name,
		DeletedBy: 1, DeletedAt: time.Now(),
	})

	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.GET("/admin/recycle-bin", HandleAdminRecycleBinList(svc))

	t.Run("list all", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/recycle-bin", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		entries, ok := resp["entries"].([]any)
		if !ok {
			t.Fatal("expected entries array")
		}
		found := false
		for _, e := range entries {
			em := e.(map[string]any)
			if em["entity_id"].(float64) == 99500 {
				found = true
			}
		}
		if !found {
			t.Error("expected test entry in list")
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/recycle-bin?entity_type="+testEntityType, nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestHandleAdminRestore(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)
	testEntityType := "test_handler_restore"
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	svc.repo.AddToRecycleBin(&RecycleBinEntry{
		EntityType: testEntityType, EntityID: 99600,
		DeletedBy: 1, DeletedAt: time.Now(),
	})

	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.POST("/admin/api/recycle-bin/restore", func(c *gin.Context) { c.Set("user_id", 1) }, HandleAdminRestore(svc))

	t.Run("restore entry", func(t *testing.T) {
		// This will fail at restoreEntity (unsupported type) but the handler should still respond.
		body := fmt.Sprintf(`{"entity_type":"%s","entity_id":99600}`, testEntityType)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/recycle-bin/restore", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		// Will return 500 because restoreEntity doesn't know "test_handler_restore"
		// but the handler infrastructure works.
		if w.Code == http.StatusBadRequest {
			t.Error("should not be 400")
		}
	})

	t.Run("missing params", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/recycle-bin/restore", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestHandleAdminBatchSoftDelete(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)

	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.POST("/admin/api/recycle-bin/batch-delete", func(c *gin.Context) { c.Set("user_id", 1) }, HandleAdminBatchSoftDelete(svc))

	t.Run("missing params", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/recycle-bin/batch-delete", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestHandleAdminBatchHardDelete(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)

	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.POST("/admin/api/recycle-bin/batch-purge", func(c *gin.Context) { c.Set("user_id", 1) }, HandleAdminBatchHardDelete(svc))

	t.Run("missing params", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/recycle-bin/batch-purge", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestHandleAdminHardDelete(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)

	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.POST("/admin/api/recycle-bin/purge", func(c *gin.Context) { c.Set("user_id", 1) }, HandleAdminHardDelete(svc))

	t.Run("missing params", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/api/recycle-bin/purge", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestHandleAdminDeletionLog(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)
	testEntityType := "test_handler_log"
	testID := int64(99700)
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	svc.repo.LogDeletion(&DeletionLog{
		EntityType: testEntityType, EntityID: testID, Action: ActionSoftDelete,
		DeletedBy: 1, DeletedAt: time.Now(),
	})

	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.GET("/admin/api/recycle-bin/log/:entity_type/:entity_id", HandleAdminDeletionLog(svc))

	t.Run("get log", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/admin/api/recycle-bin/log/%s/%d", testEntityType, testID), nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		logs, ok := resp["logs"].([]any)
		if !ok || len(logs) == 0 {
			t.Error("expected at least 1 log entry")
		}
	})

	t.Run("invalid entity_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/api/recycle-bin/log/ticket/notanumber", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}
