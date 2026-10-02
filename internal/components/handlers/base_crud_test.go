package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// A malformed body must be rejected before any SQL runs; it used to be parsed
// as empty or zero-valued data and written anyway. No DB is configured, so
// reaching the database would panic.
func TestBaseCRUDRejectsMalformedBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewBaseCRUDHandler(CRUDConfig{
		EntityName:  "widget",
		TableName:   "widget",
		RoutePrefix: "/widgets",
		Fields: []FieldConfig{
			{Name: "name", DBColumn: "name", Type: FieldTypeString},
			{Name: "rank", DBColumn: "rank", Type: FieldTypeInt},
			{Name: "weight", DBColumn: "weight", Type: FieldTypeFloat},
		},
	}, nil, nil)
	r := gin.New()
	h.RegisterRoutes(&r.RouterGroup)

	tests := []struct {
		name, method, path, contentType, body string
	}{
		{"create malformed JSON", http.MethodPost, "/widgets", "application/json", `{"name":`},
		{"update malformed JSON", http.MethodPut, "/widgets/1", "application/json", `{"name":`},
		{"create non-numeric int field", http.MethodPost, "/widgets", "application/x-www-form-urlencoded", "name=a&rank=high"},
		{"update non-numeric float field", http.MethodPut, "/widgets/1", "application/x-www-form-urlencoded", "name=a&weight=heavy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}
