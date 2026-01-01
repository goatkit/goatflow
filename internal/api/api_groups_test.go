package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIV1GroupsRouteRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GOTRS_DISABLE_TEST_AUTH_BYPASS", "1")
	router := NewSimpleRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}
