//go:build integration

package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	_ "github.com/lib/pq"
)

// getTestDB returns a database connection for testing.
func getPostmasterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	driver := currentPostmasterDriver()

	host := firstNonEmptyPostmaster(os.Getenv("TEST_DB_HOST"), os.Getenv("DB_HOST"), defaultPostmasterHost(driver))
	user := firstNonEmptyPostmaster(os.Getenv("TEST_DB_USER"), os.Getenv("DB_USER"), defaultPostmasterUser(driver))
	password := firstNonEmptyPostmaster(os.Getenv("TEST_DB_PASSWORD"), os.Getenv("DB_PASSWORD"), defaultPostmasterPassword(driver))
	dbName := firstNonEmptyPostmaster(os.Getenv("TEST_DB_NAME"), os.Getenv("DB_NAME"), defaultPostmasterDBName(driver))
	port := firstNonEmptyPostmaster(os.Getenv("TEST_DB_PORT"), os.Getenv("DB_PORT"), defaultPostmasterPort(driver))

	var db *sql.DB
	var err error

	switch driver {
	case "postgres", "pgsql", "pg":
		sslMode := firstNonEmptyPostmaster(os.Getenv("TEST_DB_SSLMODE"), os.Getenv("DB_SSL_MODE"), "disable")
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbName, sslMode)
		db, err = sql.Open("postgres", connStr)
	case "mysql", "mariadb":
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC", user, password, host, port, dbName)
		db, err = sql.Open("mysql", connStr)
	default:
		t.Fatalf("unsupported TEST_DB_DRIVER %q", driver)
	}

	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	return db
}

func setupPostmasterTestRouter(db *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Register routes for testing
	r.GET("/admin/api/postmaster-filters/:name", HandleAdminPostmasterFilterGet)
	r.POST("/admin/api/postmaster-filters", HandleCreatePostmasterFilter)
	r.PUT("/admin/api/postmaster-filters/:name", HandleUpdatePostmasterFilter)
	r.DELETE("/admin/api/postmaster-filters/:name", HandleDeletePostmasterFilter)

	return r
}

func cleanupPostmasterFiltersTest(t *testing.T, db *sql.DB) {
	t.Helper()
	query := database.ConvertPlaceholders(`DELETE FROM postmaster_filter WHERE f_name LIKE ?`)
	_, err := db.Exec(query, "APITest%")
	if err != nil {
		t.Logf("cleanup warning: %v", err)
	}
}

func TestPostmasterFilterAPI_Create(t *testing.T) {
	db := getPostmasterTestDB(t)
	defer db.Close()

	// Initialize database singleton for handlers
	database.SetDB(db)
	defer database.ResetDB()

	cleanupPostmasterFiltersTest(t, db)
	defer cleanupPostmasterFiltersTest(t, db)

	router := setupPostmasterTestRouter(db)

	t.Run("CreateFilter", func(t *testing.T) {
		input := PostmasterFilterInput{
			Name: "APITestFilter1",
			Stop: true,
			Matches: []repository.FilterMatch{
				{Key: "From", Value: ".*@test\\.com", Not: false},
			},
			Sets: []repository.FilterSet{
				{Key: "X-GOTRS-Queue", Value: "Test Queue"},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/postmaster-filters", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("expected success=true, got %v", response["success"])
		}
	})

	t.Run("CreateDuplicate", func(t *testing.T) {
		// Try to create another filter with the same name
		input := PostmasterFilterInput{
			Name: "APITestFilter1",
			Stop: false,
			Matches: []repository.FilterMatch{
				{Key: "To", Value: "support@", Not: false},
			},
			Sets: []repository.FilterSet{
				{Key: "X-GOTRS-Queue", Value: "Another Queue"},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/postmaster-filters", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status %d for duplicate, got %d", http.StatusConflict, w.Code)
		}
	})

	t.Run("CreateEmptyName", func(t *testing.T) {
		input := PostmasterFilterInput{
			Name: "",
			Matches: []repository.FilterMatch{
				{Key: "From", Value: ".*", Not: false},
			},
			Sets: []repository.FilterSet{
				{Key: "X-GOTRS-Queue", Value: "Test"},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/postmaster-filters", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d for empty name, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("CreateNoMatches", func(t *testing.T) {
		input := PostmasterFilterInput{
			Name:    "APITestNoMatches",
			Matches: []repository.FilterMatch{},
			Sets: []repository.FilterSet{
				{Key: "X-GOTRS-Queue", Value: "Test"},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/postmaster-filters", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d for no matches, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("CreateNoSets", func(t *testing.T) {
		input := PostmasterFilterInput{
			Name: "APITestNoSets",
			Matches: []repository.FilterMatch{
				{Key: "From", Value: ".*", Not: false},
			},
			Sets: []repository.FilterSet{},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/postmaster-filters", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d for no sets, got %d", http.StatusBadRequest, w.Code)
		}
	})
}

func TestPostmasterFilterAPI_Get(t *testing.T) {
	db := getPostmasterTestDB(t)
	defer db.Close()

	database.SetDB(db)
	defer database.ResetDB()

	cleanupPostmasterFiltersTest(t, db)
	defer cleanupPostmasterFiltersTest(t, db)

	// Create a filter to get
	repo := repository.NewPostmasterFilterRepository(db)
	filter := &repository.PostmasterFilter{
		Name: "APITestGetFilter",
		Stop: true,
		Matches: []repository.FilterMatch{
			{Key: "From", Value: ".*@example\\.com", Not: false},
		},
		Sets: []repository.FilterSet{
			{Key: "X-GOTRS-Queue", Value: "Example Queue"},
		},
	}
	if err := repo.Create(context.Background(), filter); err != nil {
		t.Fatalf("failed to create test filter: %v", err)
	}

	router := setupPostmasterTestRouter(db)

	t.Run("GetExisting", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/postmaster-filters/APITestGetFilter", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("expected success=true, got %v", response["success"])
		}

		data := response["data"].(map[string]interface{})
		if data["Name"] != "APITestGetFilter" {
			t.Errorf("expected Name=APITestGetFilter, got %v", data["Name"])
		}
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/postmaster-filters/NonExistentFilter", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

func TestPostmasterFilterAPI_Update(t *testing.T) {
	db := getPostmasterTestDB(t)
	defer db.Close()

	database.SetDB(db)
	defer database.ResetDB()

	cleanupPostmasterFiltersTest(t, db)
	defer cleanupPostmasterFiltersTest(t, db)

	// Create a filter to update
	repo := repository.NewPostmasterFilterRepository(db)
	filter := &repository.PostmasterFilter{
		Name: "APITestUpdateFilter",
		Stop: false,
		Matches: []repository.FilterMatch{
			{Key: "From", Value: ".*@old\\.com", Not: false},
		},
		Sets: []repository.FilterSet{
			{Key: "X-GOTRS-Queue", Value: "Old Queue"},
		},
	}
	if err := repo.Create(context.Background(), filter); err != nil {
		t.Fatalf("failed to create test filter: %v", err)
	}

	router := setupPostmasterTestRouter(db)

	t.Run("UpdateExisting", func(t *testing.T) {
		input := PostmasterFilterInput{
			Name: "APITestUpdateFilter",
			Stop: true,
			Matches: []repository.FilterMatch{
				{Key: "From", Value: ".*@new\\.com", Not: true},
			},
			Sets: []repository.FilterSet{
				{Key: "X-GOTRS-Queue", Value: "New Queue"},
				{Key: "X-GOTRS-PriorityID", Value: "4"},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPut, "/admin/api/postmaster-filters/APITestUpdateFilter", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}

		// Verify the update
		updated, err := repo.Get(context.Background(), "APITestUpdateFilter")
		if err != nil {
			t.Fatalf("failed to get updated filter: %v", err)
		}

		if !updated.Stop {
			t.Error("expected Stop=true after update")
		}
		if len(updated.Matches) != 1 || !updated.Matches[0].Not {
			t.Error("expected NOT flag to be true after update")
		}
		if len(updated.Sets) != 2 {
			t.Errorf("expected 2 sets after update, got %d", len(updated.Sets))
		}
	})

	t.Run("UpdateNonExistent", func(t *testing.T) {
		input := PostmasterFilterInput{
			Name: "NonExistent",
			Matches: []repository.FilterMatch{
				{Key: "From", Value: ".*", Not: false},
			},
			Sets: []repository.FilterSet{
				{Key: "X-GOTRS-Queue", Value: "Test"},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPut, "/admin/api/postmaster-filters/NonExistentFilter", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

func TestPostmasterFilterAPI_Delete(t *testing.T) {
	db := getPostmasterTestDB(t)
	defer db.Close()

	database.SetDB(db)
	defer database.ResetDB()

	cleanupPostmasterFiltersTest(t, db)
	defer cleanupPostmasterFiltersTest(t, db)

	// Create a filter to delete
	repo := repository.NewPostmasterFilterRepository(db)
	filter := &repository.PostmasterFilter{
		Name: "APITestDeleteFilter",
		Stop: false,
		Matches: []repository.FilterMatch{
			{Key: "From", Value: ".*@delete\\.com", Not: false},
		},
		Sets: []repository.FilterSet{
			{Key: "X-GOTRS-Queue", Value: "Delete Queue"},
		},
	}
	if err := repo.Create(context.Background(), filter); err != nil {
		t.Fatalf("failed to create test filter: %v", err)
	}

	router := setupPostmasterTestRouter(db)

	t.Run("DeleteExisting", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/api/postmaster-filters/APITestDeleteFilter", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}

		// Verify deletion
		_, err := repo.Get(context.Background(), "APITestDeleteFilter")
		if err != sql.ErrNoRows {
			t.Errorf("expected filter to be deleted, got err=%v", err)
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/api/postmaster-filters/NonExistentFilter", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

// Helper functions

func firstNonEmptyPostmaster(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultPostmasterHost(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "mariadb-test"
	default:
		return "postgres-test"
	}
}

func defaultPostmasterPort(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "3306"
	default:
		return "5432"
	}
}

func defaultPostmasterUser(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "otrs"
	default:
		return "gotrs_user"
	}
}

func defaultPostmasterPassword(driver string) string {
	if pw := os.Getenv("TEST_DB_PASSWORD"); pw != "" {
		return pw
	}
	switch driver {
	case "mysql", "mariadb":
		if pw := os.Getenv("TEST_DB_MYSQL_PASSWORD"); pw != "" {
			return pw
		}
	default:
		if pw := os.Getenv("TEST_DB_POSTGRES_PASSWORD"); pw != "" {
			return pw
		}
	}
	return ""
}

func defaultPostmasterDBName(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "otrs_test"
	default:
		return "gotrs_test"
	}
}

func currentPostmasterDriver() string {
	driver := strings.ToLower(firstNonEmptyPostmaster(os.Getenv("TEST_DB_DRIVER"), os.Getenv("DB_DRIVER")))
	if driver == "" {
		if strings.Contains(strings.ToLower(os.Getenv("DATABASE_URL")), "mysql") {
			driver = "mysql"
		} else {
			driver = "postgres"
		}
	}
	return driver
}
