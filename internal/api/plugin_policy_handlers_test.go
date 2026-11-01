package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// setupPolicyTestRouter is the plugin test router with hello's stored policy
// dropped before and after the test, so runs start from its declarations.
func setupPolicyTestRouter(t *testing.T) (*gin.Engine, *plugin.Manager) {
	t.Helper()
	r, mgr := setupPluginTestRouter(t)
	_, err := mgr.ResetPolicy("hello")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = mgr.ResetPolicy("hello") })
	return r, mgr
}

// withPolicyReloader records the plugins the reload hook is asked to load.
func withPolicyReloader(t *testing.T, err error) *[]string {
	t.Helper()
	var reloaded []string
	prev := pluginReloader
	SetPluginReloader(func(_ context.Context, name string) error {
		reloaded = append(reloaded, name)
		return err
	})
	t.Cleanup(func() { SetPluginReloader(prev) })
	return &reloaded
}

// withPluginRuntime makes every plugin classify as the given runtime.
func withPluginRuntime(t *testing.T, runtime string) {
	t.Helper()
	prev := pluginRuntimeOf
	pluginRuntimeOf = func(plugin.Plugin) string { return runtime }
	t.Cleanup(func() { pluginRuntimeOf = prev })
}

func doPolicyRequest(t *testing.T, r *gin.Engine, method, name, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/plugins/"+name+"/policy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	return w.Code, resp
}

func hasNetwork(perms []plugin.Permission) int {
	n := 0
	for _, p := range perms {
		if p.Type == pkgplugin.PermissionNetwork {
			n++
		}
	}
	return n
}

func TestPluginPolicyUpdateRejectsInvalidValues(t *testing.T) {
	r, mgr := setupPolicyTestRouter(t)
	before, ok := mgr.GetPolicy("hello")
	require.True(t, ok)

	cases := map[string]struct {
		body  string
		field string
	}{
		"memory below 16":         {`{"memory_mb": 15}`, "memory_mb"},
		"memory above 65536":      {`{"memory_mb": 65537}`, "memory_mb"},
		"cpu below 100":           {`{"cpu_millis": 99}`, "cpu_millis"},
		"cpu above 64000":         {`{"cpu_millis": 64001}`, "cpu_millis"},
		"call timeout not Go":     {`{"call_timeout": "thirty"}`, "call_timeout"},
		"call timeout under 1s":   {`{"call_timeout": "500ms"}`, "call_timeout"},
		"call timeout over 24h":   {`{"call_timeout": "24h1s"}`, "call_timeout"},
		"init timeout zero":       {`{"init_timeout": "0s"}`, "init_timeout"},
		"negative call rate":      {`{"max_calls_per_second": -1}`, "max_calls_per_second"},
		"db rate above a million": {`{"max_db_queries_per_min": 1000001}`, "max_db_queries_per_min"},
		"http rate negative":      {`{"max_http_req_per_min": -5}`, "max_http_req_per_min"},
		"unknown status":          {`{"status": "trusted"}`, "status"},
		"response size zero":      {`{"max_http_response_mb": 0}`, "max_http_response_mb"},
		"response size over 1024": {`{"max_http_response_mb": 1025}`, "max_http_response_mb"},
		"internal host wildcard":  {`{"allowed_internal_hosts": ["*.lan"]}`, "allowed_internal_hosts"},
		"internal host with port": {`{"allowed_internal_hosts": ["localhost:9998"]}`, "allowed_internal_hosts"},
		"internal host as URL":    {`{"allowed_internal_hosts": ["http://tika"]}`, "allowed_internal_hosts"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", tc.body)
			assert.Equal(t, http.StatusBadRequest, code)
			fields, _ := resp["fields"].(map[string]any)
			assert.Contains(t, fields, tc.field)
			assert.Len(t, fields, 1)
		})
	}

	t.Run("malformed body", func(t *testing.T) {
		code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": "lots"}`)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.NotEmpty(t, resp["error"])
	})

	t.Run("one bad field rejects the whole update", func(t *testing.T) {
		code, _ := doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 1024, "cpu_millis": 5}`)
		assert.Equal(t, http.StatusBadRequest, code)
		after, _ := mgr.GetPolicy("hello")
		assert.Equal(t, before, after)
		assert.False(t, mgr.HasStoredPolicy("hello"))
	})

	t.Run("bounds are inclusive", func(t *testing.T) {
		body := `{"memory_mb": 16, "cpu_millis": 64000, "call_timeout": "24h", "init_timeout": "1s",
			"max_calls_per_second": 0, "max_db_queries_per_min": 1000000, "max_http_req_per_min": 0, "status": "approved"}`
		code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", body)
		require.Equal(t, http.StatusOK, code, resp)
		p, _ := mgr.GetPolicy("hello")
		assert.Equal(t, 16, p.MemoryMB)
		assert.Equal(t, 64000, p.CPUMillis)
		assert.Equal(t, "24h", p.CallTimeout)
		assert.Equal(t, "1s", p.InitTimeout)
		assert.Equal(t, 1000000, p.MaxDBQueriesPerMin)
		assert.Equal(t, "approved", p.Status)
	})

	t.Run("internal hosts are normalised and replaced", func(t *testing.T) {
		body := `{"max_http_response_mb": 1024, "allowed_internal_hosts": [" Tika.LAN. ", "10.0.0.5", "[::1]", "tika.lan", ""]}`
		code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", body)
		require.Equal(t, http.StatusOK, code, resp)
		p, _ := mgr.GetPolicy("hello")
		assert.Equal(t, 1024, p.MaxHTTPResponseMB)
		assert.Equal(t, []string{"tika.lan", "10.0.0.5", "::1"}, p.AllowedInternalHosts)

		code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 64}`)
		require.Equal(t, http.StatusOK, code)
		p, _ = mgr.GetPolicy("hello")
		assert.Len(t, p.AllowedInternalHosts, 3, "an update without the field keeps the list")

		code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"allowed_internal_hosts": []}`)
		require.Equal(t, http.StatusOK, code)
		p, _ = mgr.GetPolicy("hello")
		assert.Nil(t, p.AllowedInternalHosts)
	})
}

// A Go plugin process cannot start below about 96 MB, so gRPC plugins get a
// higher memory floor than WASM ones.
func TestPluginPolicyGRPCMemoryFloor(t *testing.T) {
	r, mgr := setupPolicyTestRouter(t)
	withPluginRuntime(t, pluginRuntimeGRPC)
	withPolicyReloader(t, nil)

	code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 127}`)
	assert.Equal(t, http.StatusBadRequest, code)
	fields, _ := resp["fields"].(map[string]any)
	assert.Contains(t, fields, "memory_mb")

	code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 128}`)
	require.Equal(t, http.StatusOK, code)
	p, _ := mgr.GetPolicy("hello")
	assert.Equal(t, 128, p.MemoryMB)
}

func TestPluginPolicyUnknownPlugin(t *testing.T) {
	r, _ := setupPolicyTestRouter(t)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		code, _ := doPolicyRequest(t, r, method, "nonexistent", `{"memory_mb": 256}`)
		assert.Equal(t, http.StatusNotFound, code, method)
	}
}

func TestPluginPolicyNetworkToggleKeepsOtherPermissions(t *testing.T) {
	r, mgr := setupPolicyTestRouter(t)
	start, _ := mgr.GetPolicy("hello")
	start.Permissions = []plugin.Permission{
		{Type: "db", Access: "read", Scope: []string{"ticket"}},
		{Type: "http", Scope: []string{"api.example.net"}},
	}
	mgr.SetPolicy("hello", start)

	code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", `{"network": true}`)
	require.Equal(t, http.StatusOK, code, resp)
	p, _ := mgr.GetPolicy("hello")
	assert.Equal(t, 1, hasNetwork(p.Permissions))
	assert.Subset(t, p.Permissions, start.Permissions)
	assert.Len(t, p.Permissions, 3)
	assert.True(t, p.Launch().Network)

	code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"network": true}`)
	require.Equal(t, http.StatusOK, code)
	p, _ = mgr.GetPolicy("hello")
	assert.Equal(t, 1, hasNetwork(p.Permissions), "granting twice keeps one network permission")

	code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 768}`)
	require.Equal(t, http.StatusOK, code)
	p, _ = mgr.GetPolicy("hello")
	assert.Equal(t, 1, hasNetwork(p.Permissions), "omitting network keeps the grant")

	code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"network": false}`)
	require.Equal(t, http.StatusOK, code)
	p, _ = mgr.GetPolicy("hello")
	assert.Equal(t, 0, hasNetwork(p.Permissions))
	assert.Equal(t, start.Permissions, p.Permissions)
	assert.Equal(t, 768, p.MemoryMB)
}

func TestPluginPolicyReloadsOnlyForLaunchChanges(t *testing.T) {
	t.Run("grpc", func(t *testing.T) {
		r, _ := setupPolicyTestRouter(t)
		withPluginRuntime(t, pluginRuntimeGRPC)
		reloads := withPolicyReloader(t, nil)

		code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", `{"call_timeout": "45s", "init_timeout": "20s", "max_calls_per_second": 5,
			"max_http_response_mb": 100, "allowed_internal_hosts": ["tika"]}`)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Empty(t, *reloads, "timeouts, rates and host HTTP settings apply without a reload")
		assert.Equal(t, false, resp["reloaded"])

		code, resp = doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 1024}`)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, []string{"hello"}, *reloads)
		assert.Equal(t, true, resp["reloaded"])

		code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"cpu_millis": 2000}`)
		require.Equal(t, http.StatusOK, code)
		code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"network": true}`)
		require.Equal(t, http.StatusOK, code)
		assert.Len(t, *reloads, 3)

		code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 1024, "cpu_millis": 2000, "network": true}`)
		require.Equal(t, http.StatusOK, code)
		assert.Len(t, *reloads, 3, "unchanged launch limits do not reload")
	})

	t.Run("wasm reloads for memory only", func(t *testing.T) {
		r, _ := setupPolicyTestRouter(t)
		withPluginRuntime(t, pluginRuntimeWASM)
		reloads := withPolicyReloader(t, nil)

		code, _ := doPolicyRequest(t, r, http.MethodPut, "hello", `{"cpu_millis": 3000, "network": true}`)
		require.Equal(t, http.StatusOK, code)
		assert.Empty(t, *reloads)

		code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 64}`)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, []string{"hello"}, *reloads)
	})

	t.Run("builtin never reloads", func(t *testing.T) {
		r, _ := setupPolicyTestRouter(t)
		reloads := withPolicyReloader(t, nil)

		code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 2048, "network": true}`)
		require.Equal(t, http.StatusOK, code)
		assert.Empty(t, *reloads)
		assert.Equal(t, pluginRuntimeBuiltin, resp["runtime"])
	})

	t.Run("failed reload keeps the saved policy and says so", func(t *testing.T) {
		r, mgr := setupPolicyTestRouter(t)
		withPluginRuntime(t, pluginRuntimeGRPC)
		withPolicyReloader(t, errors.New("binary missing"))

		code, resp := doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 300}`)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, false, resp["reloaded"])
		assert.Equal(t, "binary missing", resp["reload_error"])
		assert.Contains(t, resp["message"], "binary missing")
		p, _ := mgr.GetPolicy("hello")
		assert.Equal(t, 300, p.MemoryMB)
	})
}

func TestPluginPolicyGetAndReset(t *testing.T) {
	r, mgr := setupPolicyTestRouter(t)
	withPluginRuntime(t, pluginRuntimeGRPC)
	reloads := withPolicyReloader(t, nil)
	declared, _ := mgr.GetPolicy("hello")

	code, resp := doPolicyRequest(t, r, http.MethodGet, "hello", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, false, resp["stored"])
	assert.Equal(t, pluginRuntimeGRPC, resp["runtime"])
	launch, _ := resp["launch"].(map[string]any)
	assert.EqualValues(t, declared.Launch().MemoryMB, launch["memory_mb"])

	code, _ = doPolicyRequest(t, r, http.MethodPut, "hello", `{"memory_mb": 4096, "network": true}`)
	require.Equal(t, http.StatusOK, code)
	code, resp = doPolicyRequest(t, r, http.MethodGet, "hello", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, resp["stored"])
	launch, _ = resp["launch"].(map[string]any)
	assert.EqualValues(t, 4096, launch["memory_mb"])
	assert.Equal(t, true, launch["network"])

	code, resp = doPolicyRequest(t, r, http.MethodDelete, "hello", "")
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, false, resp["stored"])
	assert.Equal(t, true, resp["reloaded"], "reset changed the launch limits back")
	assert.Len(t, *reloads, 2)
	p, _ := mgr.GetPolicy("hello")
	assert.Equal(t, declared.Launch(), p.Launch())
	assert.False(t, mgr.HasStoredPolicy("hello"))
}
