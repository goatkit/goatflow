package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// The Add Group modal posts the selected status as valid_id; the group must be
// stored with that status (2 = inactive), and active when none is sent.
func TestCreateGroupStoresSelectedStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)
	db := isolatedDB(t)

	cases := []struct {
		name    string
		validID string
		want    int
	}{
		{name: "inactive", validID: "2", want: 2},
		{name: "active", validID: "1", want: 1},
		{name: "unset", validID: "", want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groupName := fmt.Sprintf("create_status_%s_%d", tc.name, time.Now().UnixNano())
			cleanupGroupByNameAtEnd(t, groupName)

			form := url.Values{"name": {groupName}, "comments": {"status test"}}
			if tc.validID != "" {
				form.Set("valid_id", tc.validID)
			}
			router := gin.New()
			SetupHTMXRoutes(router)
			req := httptest.NewRequest(http.MethodPost, "/admin/groups", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			AddTestAuthCookie(req, token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
			var validID int
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
				"SELECT valid_id FROM `groups` WHERE name = ?"), groupName).Scan(&validID))
			assert.Equal(t, tc.want, validID)
		})
	}
}
