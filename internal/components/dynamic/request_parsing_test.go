package dynamic

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Non-numeric input for a numeric module field is an error; it used to be
// stored as 0.
func TestConvertValueRejectsNonNumericInput(t *testing.T) {
	h := &DynamicModuleHandler{}

	for _, tc := range []struct{ value, fieldType string }{
		{"high", "int"},
		{"", "integer"},
		{"heavy", "float"},
		{"x1.5", "decimal"},
	} {
		_, err := h.convertValue(tc.value, tc.fieldType)
		assert.Error(t, err, "%s %q", tc.fieldType, tc.value)
	}

	v, err := h.convertValue("42", "int")
	require.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = h.convertValue("1.5", "float")
	require.NoError(t, err)
	assert.Equal(t, 1.5, v)
}

// A malformed JSON create/update body is an error, not an empty record.
func TestParseFormDataRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &DynamicModuleHandler{}
	cfg := &ModuleConfig{}
	parse := func(body string) (map[string]interface{}, error) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/dynamic/x", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return h.parseFormData(c, cfg)
	}

	_, err := parse(`{"name":`)
	assert.Error(t, err)

	data, err := parse(`{"name":"ok"}`)
	require.NoError(t, err)
	assert.Equal(t, "ok", data["name"])
}

// The manual-run record body is optional, but a malformed one must not run
// the workflow with no record.
func TestHandleWorkflowExecuteBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	we := NewWorkflowEngine(nil)
	var runs []map[string]interface{}
	we.RegisterHandler("record", func(ctx *WorkflowContext, _ Action) error {
		runs = append(runs, ctx.Record)
		return nil
	})
	require.NoError(t, we.RegisterWorkflow(&Workflow{ID: "wf", Enabled: true, Actions: []Action{{Type: "record"}}}))

	r := gin.New()
	r.POST("/workflows/:id/execute", we.HandleWorkflowExecute)
	serve := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/workflows/wf/execute", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	assert.Equal(t, http.StatusBadRequest, serve(`{"id":`))
	assert.Empty(t, runs, "malformed body must not run the workflow")

	assert.Equal(t, http.StatusOK, serve(""))
	assert.Equal(t, http.StatusOK, serve(`{"id":7}`))
	require.Len(t, runs, 2)
	assert.Nil(t, runs[0])
	assert.Equal(t, float64(7), runs[1]["id"])
}
