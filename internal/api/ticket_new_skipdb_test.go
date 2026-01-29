package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTicketsNewSkipsDatabaseWhenDbWaitDisabled(t *testing.T) {
	t.Setenv("SKIP_DB_WAIT", "1")
	t.Setenv("APP_ENV", "integration")

	router := NewSimpleRouter()

	req := httptest.NewRequest(http.MethodGet, "/tickets/new", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Without database, we expect auth to fail (401) or redirect (302), but not crash (500+)
	if w.Code >= http.StatusInternalServerError {
		t.Fatalf("expected graceful response, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTicketsNewAfterDashboardSkipsDatabase(t *testing.T) {
	t.Setenv("SKIP_DB_WAIT", "1")
	t.Setenv("APP_ENV", "integration")

	router := NewSimpleRouter()

	// Mimic the link checker hitting the dashboard first
	if w := httptest.NewRecorder(); true {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		router.ServeHTTP(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/tickets/new", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Without database, we expect auth to fail (401) or redirect (302), but not crash (500+)
	if w.Code >= http.StatusInternalServerError {
		t.Fatalf("expected graceful response after dashboard, got %d: %s", w.Code, w.Body.String())
	}
}
