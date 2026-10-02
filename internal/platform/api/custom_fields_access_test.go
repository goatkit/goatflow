package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/customfields"
)

// A queue id that is not positive names no queue; it used to wrap to a huge
// uint queue id and pass the admin shortcut.
func TestCustomFieldAccessQueueRejectsNonPositiveID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	a := &customFieldAccess{c: c, userID: 1, isAdmin: true}

	for _, id := range []int64{0, -1} {
		status, err := a.check(customfields.EntityQueue, id, false)
		if err != nil {
			t.Fatalf("id %d: unexpected error %v", id, err)
		}
		if status != http.StatusNotFound {
			t.Fatalf("id %d: status = %d, want %d", id, status, http.StatusNotFound)
		}
	}

	if status, err := a.check(customfields.EntityQueue, 1, false); err != nil || status != 0 {
		t.Fatalf("id 1 as admin: status = %d, err = %v, want allowed", status, err)
	}
}
