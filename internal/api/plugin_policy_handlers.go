package api

import (
	"context"
	"net/http"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/landlock"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	plugingrpc "github.com/goatkit/goatflow/internal/platform/plugin/grpc"
	pluginwasm "github.com/goatkit/goatflow/internal/platform/plugin/wasm"
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// Bounds an admin may set on a plugin's resource policy. A Go gRPC plugin
// process cannot start with less than about 96 MB of data segment.
const (
	policyMinMemoryMB     = 16
	policyMinGRPCMemoryMB = 128
	policyMaxMemoryMB     = 65536
	policyMinCPUMillis    = 100
	policyMaxCPUMillis    = 64000
	policyMinTimeout      = time.Second
	policyMaxTimeout      = 24 * time.Hour
	policyMaxRate         = 1_000_000
	policyMaxHTTPRespMB   = 1024
	policyMaxInternalHost = 20
)

// hostnamePattern matches a DNS hostname (lower case, no port or scheme).
var hostnamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// Plugin runtimes reported by the policy API.
const (
	pluginRuntimeGRPC    = "grpc"
	pluginRuntimeWASM    = "wasm"
	pluginRuntimeBuiltin = "builtin"
)

var pluginPolicyStatuses = []string{"pending_review", "approved", "restricted", "blocked"}

// pluginPolicyUpdate is the body of PUT /api/v1/plugins/:name/policy. Omitted
// fields keep their current value.
type pluginPolicyUpdate struct {
	MemoryMB           *int    `json:"memory_mb"`
	CPUMillis          *int    `json:"cpu_millis"`
	CallTimeout        *string `json:"call_timeout"`
	InitTimeout        *string `json:"init_timeout"`
	MaxCallsPerSecond  *int    `json:"max_calls_per_second"`
	MaxDBQueriesPerMin *int    `json:"max_db_queries_per_min"`
	MaxHTTPReqPerMin   *int    `json:"max_http_req_per_min"`
	MaxHTTPResponseMB  *int    `json:"max_http_response_mb"`
	// AllowedInternalHosts replaces the list when present ([] clears it).
	AllowedInternalHosts *[]string `json:"allowed_internal_hosts"`
	Network              *bool     `json:"network"`
	Status               *string   `json:"status"`
}

// normaliseInternalHosts lower-cases, trims and de-duplicates hosts,
// returning ok=false when one is not a hostname or IP address (wildcards,
// ports, schemes and paths are refused) or there are too many.
func normaliseInternalHosts(hosts []string) ([]string, bool) {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
		if h == "" {
			continue
		}
		if a, err := netip.ParseAddr(strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")); err == nil {
			h = a.WithZone("").String()
		} else if len(h) > 253 || !hostnamePattern.MatchString(h) {
			return nil, false
		}
		if !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out, len(out) <= policyMaxInternalHost
}

// validate returns a translated message per invalid field of an update to a
// plugin of the given runtime.
func (u pluginPolicyUpdate) validate(c *gin.Context, runtime string) map[string]string {
	fields := map[string]string{}
	checkRange := func(field string, v *int, lo, hi int) {
		if v != nil && (*v < lo || *v > hi) {
			fields[field] = middleware.T(c, "admin.plugin_limits.err_range", lo, hi)
		}
	}
	checkDuration := func(field string, v *string) {
		if v == nil {
			return
		}
		d, err := time.ParseDuration(strings.TrimSpace(*v))
		if err != nil || d < policyMinTimeout || d > policyMaxTimeout {
			fields[field] = middleware.T(c, "admin.plugin_limits.err_duration", policyMinTimeout.String(), "24h")
		}
	}
	minMemory := policyMinMemoryMB
	if runtime == pluginRuntimeGRPC {
		minMemory = policyMinGRPCMemoryMB
	}
	checkRange("memory_mb", u.MemoryMB, minMemory, policyMaxMemoryMB)
	checkRange("cpu_millis", u.CPUMillis, policyMinCPUMillis, policyMaxCPUMillis)
	checkDuration("call_timeout", u.CallTimeout)
	checkDuration("init_timeout", u.InitTimeout)
	checkRange("max_calls_per_second", u.MaxCallsPerSecond, 0, policyMaxRate)
	checkRange("max_db_queries_per_min", u.MaxDBQueriesPerMin, 0, policyMaxRate)
	checkRange("max_http_req_per_min", u.MaxHTTPReqPerMin, 0, policyMaxRate)
	checkRange("max_http_response_mb", u.MaxHTTPResponseMB, 1, policyMaxHTTPRespMB)
	if u.AllowedInternalHosts != nil {
		if _, ok := normaliseInternalHosts(*u.AllowedInternalHosts); !ok {
			fields["allowed_internal_hosts"] = middleware.T(c, "admin.plugin_limits.err_internal_hosts", policyMaxInternalHost)
		}
	}
	if u.Status != nil && !slices.Contains(pluginPolicyStatuses, *u.Status) {
		fields["status"] = middleware.T(c, "admin.plugin_limits.err_status")
	}
	return fields
}

// apply returns current with the update's fields set. Permissions other than
// network are kept.
func (u pluginPolicyUpdate) apply(current plugin.ResourcePolicy) plugin.ResourcePolicy {
	p := current
	p.Permissions = slices.Clone(current.Permissions)
	if u.MemoryMB != nil {
		p.MemoryMB = *u.MemoryMB
	}
	if u.CPUMillis != nil {
		p.CPUMillis = *u.CPUMillis
	}
	if u.CallTimeout != nil {
		p.CallTimeout = strings.TrimSpace(*u.CallTimeout)
	}
	if u.InitTimeout != nil {
		p.InitTimeout = strings.TrimSpace(*u.InitTimeout)
	}
	if u.MaxCallsPerSecond != nil {
		p.MaxCallsPerSecond = *u.MaxCallsPerSecond
	}
	if u.MaxDBQueriesPerMin != nil {
		p.MaxDBQueriesPerMin = *u.MaxDBQueriesPerMin
	}
	if u.MaxHTTPReqPerMin != nil {
		p.MaxHTTPReqPerMin = *u.MaxHTTPReqPerMin
	}
	if u.MaxHTTPResponseMB != nil {
		p.MaxHTTPResponseMB = *u.MaxHTTPResponseMB
	}
	if u.AllowedInternalHosts != nil {
		p.AllowedInternalHosts, _ = normaliseInternalHosts(*u.AllowedInternalHosts)
		if len(p.AllowedInternalHosts) == 0 {
			p.AllowedInternalHosts = nil
		}
	}
	if u.Status != nil {
		p.Status = *u.Status
	}
	if u.Network != nil {
		p.Permissions = withNetworkPermission(p.Permissions, *u.Network)
	}
	return p
}

// withNetworkPermission returns perms with the network permission present
// exactly once when on, absent otherwise.
func withNetworkPermission(perms []plugin.Permission, on bool) []plugin.Permission {
	out := make([]plugin.Permission, 0, len(perms)+1)
	for _, p := range perms {
		if p.Type != pkgplugin.PermissionNetwork {
			out = append(out, p)
		}
	}
	if on {
		out = append(out, plugin.Permission{Type: pkgplugin.PermissionNetwork})
	}
	return out
}

// pluginRuntime reports how a registered plugin runs: its own process
// (grpc), a WASM module, or compiled into GoatFlow (builtin). "" when the
// plugin is not registered.
func pluginRuntime(name string) string {
	p, ok := pluginManager.Lookup(name)
	if !ok {
		return ""
	}
	return pluginRuntimeOf(p)
}

// pluginRuntimeOf classifies a plugin by its runtime type. A variable so
// tests can stand in a process runtime for an in-process plugin.
var pluginRuntimeOf = func(p plugin.Plugin) string {
	switch p.(type) {
	case *plugingrpc.GRPCPlugin:
		return pluginRuntimeGRPC
	case *pluginwasm.WASMPlugin:
		return pluginRuntimeWASM
	}
	return pluginRuntimeBuiltin
}

// launchChanged reports whether the runtime applies a changed start-time
// limit only when the plugin is loaded again: WASM fixes its memory at
// load, a gRPC process all its launch limits.
func launchChanged(runtime string, before, after pkgplugin.LaunchLimits) bool {
	switch runtime {
	case pluginRuntimeGRPC:
		return before != after
	case pluginRuntimeWASM:
		return before.MemoryMB != after.MemoryMB
	}
	return false
}

// pluginPolicyView is the policy API's description of a plugin's limits.
func pluginPolicyView(name string, policy plugin.ResourcePolicy) gin.H {
	runtime := pluginRuntime(name)
	launch := policy.Launch()
	view := gin.H{
		"name":                     name,
		"policy":                   policy,
		"stored":                   pluginManager.HasStoredPolicy(name),
		"runtime":                  runtime,
		"default_http_response_mb": plugin.DefaultHTTPResponseMB,
		"launch": gin.H{
			"memory_mb":  launch.MemoryMB,
			"cpu_millis": launch.CPUMillis,
			"network":    launch.Network,
		},
		"enforced": gin.H{
			"memory":  runtime == pluginRuntimeGRPC || runtime == pluginRuntimeWASM,
			"cpu":     runtime == pluginRuntimeGRPC,
			"network": runtime == pluginRuntimeGRPC,
		},
	}
	if stats, ok := pluginManager.PluginStats(name); ok {
		view["usage"] = stats
	}
	if p, ok := pluginManager.Lookup(name); ok {
		if cg, ok := p.(interface{ CgroupEnforced() bool }); ok {
			view["cgroup_enforced"] = cg.CgroupEnforced()
		}
		if sb, ok := p.(interface{ SandboxABI() int }); ok {
			abi := sb.SandboxABI()
			view["sandbox"] = gin.H{
				"files":       abi >= landlock.FilesABI,
				"signals":     abi >= landlock.SignalsABI,
				"full_kernel": landlock.FullKernel,
			}
		}
	}
	return view
}

// policyAuditFields are the policy fields an admin edits, for audit diffs.
func policyAuditFields(p plugin.ResourcePolicy) map[string]any {
	return map[string]any{
		"status":                 p.Status,
		"memory_mb":              p.MemoryMB,
		"cpu_millis":             p.CPUMillis,
		"call_timeout":           p.CallTimeout,
		"init_timeout":           p.InitTimeout,
		"max_calls_per_second":   p.MaxCallsPerSecond,
		"max_db_queries_per_min": p.MaxDBQueriesPerMin,
		"max_http_req_per_min":   p.MaxHTTPReqPerMin,
		"max_http_response_mb":   effectiveHTTPResponseMB(p),
		"allowed_internal_hosts": strings.Join(p.AllowedInternalHosts, ", "),
		"network":                p.Launch().Network,
	}
}

func effectiveHTTPResponseMB(p plugin.ResourcePolicy) int {
	if p.MaxHTTPResponseMB > 0 {
		return p.MaxHTTPResponseMB
	}
	return plugin.DefaultHTTPResponseMB
}

// applyPolicyChange reloads a running plugin whose launch limits changed and
// answers with the new policy and a message saying how it takes effect.
func applyPolicyChange(c *gin.Context, name, action string, before, after plugin.ResourcePolicy) {
	runtime := pluginRuntime(name)
	reloaded := false
	messageKey := "admin.plugin_limits.saved"
	var reloadErr error
	if launchChanged(runtime, before.Launch(), after.Launch()) {
		messageKey = "admin.plugin_limits.saved_next_start"
		if pluginReloader != nil {
			if reloadErr = pluginReloader(context.WithoutCancel(c.Request.Context()), name); reloadErr == nil {
				reloaded = true
				messageKey = "admin.plugin_limits.saved_reloaded"
				plugin.GetLogBuffer().Log(name, "info", "Plugin reloaded to apply new resource limits", nil)
			} else {
				plugin.GetLogBuffer().Log(name, "error", "Reload to apply new resource limits failed: "+reloadErr.Error(), nil)
			}
		}
	}

	recordPluginAudit(c, action, name, map[string]any{
		"changes":  audit.Changes(policyAuditFields(before), policyAuditFields(after)),
		"reloaded": reloaded,
	})

	view := pluginPolicyView(name, after)
	view["reloaded"] = reloaded
	if reloadErr != nil {
		view["reload_error"] = reloadErr.Error()
		view["message"] = middleware.T(c, "admin.plugin_limits.reload_failed", reloadErr.Error())
	} else {
		view["message"] = middleware.T(c, messageKey)
	}
	c.JSON(http.StatusOK, view)
}

// HandlePluginPolicyGet returns a plugin's effective resource policy.
// GET /api/v1/plugins/:name/policy
func HandlePluginPolicyGet(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}
	name := c.Param("name")
	policy, ok := pluginManager.GetPolicy(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	c.JSON(http.StatusOK, pluginPolicyView(name, policy))
}

// HandlePluginPolicyUpdate stores an admin policy for a plugin, starting from
// its effective policy, and reloads the plugin when a launch limit changed.
// PUT /api/v1/plugins/:name/policy
func HandlePluginPolicyUpdate(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}
	name := c.Param("name")
	before, ok := pluginManager.GetPolicy(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	var req pluginPolicyUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": middleware.T(c, "admin.plugin_limits.err_body")})
		return
	}
	if fields := req.validate(c, pluginRuntime(name)); len(fields) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": middleware.T(c, "admin.plugin_limits.invalid"), "fields": fields})
		return
	}
	after := req.apply(before)
	pluginManager.SetPolicy(name, after)
	applyPolicyChange(c, name, "admin.plugin.policy.update", before, after)
}

// HandlePluginPolicyReset drops a plugin's admin-stored policy, so it runs
// with the limits and permissions it declares.
// DELETE /api/v1/plugins/:name/policy
func HandlePluginPolicyReset(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}
	name := c.Param("name")
	before, ok := pluginManager.GetPolicy(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	after, err := pluginManager.ResetPolicy(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	applyPolicyChange(c, name, "admin.plugin.policy.reset", before, after)
}
