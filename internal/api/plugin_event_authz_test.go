package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// eventAuthzPlugin declares an EventAuthorizer that allows the "public"
// channel for everyone and "ops" for admins only, and fails on "boom".
type eventAuthzPlugin struct {
	name       string
	authorizer string
	mu         sync.Mutex
	seen       []map[string]any
}

func (p *eventAuthzPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{Name: p.name, Version: "1.0.0", EventAuthorizer: p.authorizer}
}
func (p *eventAuthzPlugin) Init(context.Context, plugin.HostAPI) error { return nil }
func (p *eventAuthzPlugin) Shutdown(context.Context) error             { return nil }
func (p *eventAuthzPlugin) Call(_ context.Context, fn string, raw json.RawMessage) (json.RawMessage, error) {
	if fn != "authorize_events" {
		return nil, errors.New("unknown function " + fn)
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.seen = append(p.seen, args)
	p.mu.Unlock()
	channel, _ := args["channel"].(string)
	isAdmin, _ := args["_is_admin"].(bool)
	switch channel {
	case "boom":
		return nil, errors.New("authorizer failed")
	case "public":
		return json.Marshal(map[string]bool{"allow": true})
	case "ops":
		return json.Marshal(map[string]bool{"allow": isAdmin})
	}
	return json.Marshal(map[string]bool{"allow": false})
}

func (p *eventAuthzPlugin) lastArgs() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.seen) == 0 {
		return nil
	}
	return p.seen[len(p.seen)-1]
}

// subscribe opens the SSE endpoint and gives up after a short while; an
// allowed subscription streams until then.
func subscribe(t *testing.T, router http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// GET /api/v1/plugins/:name/events/:channel asks the plugin's EventAuthorizer
// (with the caller's identity) before subscribing; plugins without one keep
// the previous behaviour of admitting every agent.
func TestPluginEventSubscriptionAuthorizedByPlugin(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)

	gated := &eventAuthzPlugin{name: "evauthz", authorizer: "authorize_events"}
	legacy := &eventAuthzPlugin{name: "evlegacy"}
	mgr := plugin.NewManager(plugin.NewProdHostAPI(plugin.WithDB("default", db)))
	for _, p := range []*eventAuthzPlugin{gated, legacy} {
		require.NoError(t, mgr.Register(context.Background(), p))
		require.NoError(t, mgr.Enable(p.name))
	}
	prevMgr, prevBroker := GetPluginManager(), pluginSSEBroker
	SetPluginManager(mgr)
	broker := plugin.NewSSEBroker()
	SetPluginSSEBroker(broker)
	t.Cleanup(func() {
		SetPluginManager(prevMgr)
		SetPluginSSEBroker(prevBroker)
	})

	jwt := shared.GetJWTManager()
	agentTok, err := jwt.GenerateTokenWithLogin(4242, "ev.agent", "ev.agent", "Agent", false, 0)
	require.NoError(t, err)
	adminTok, err := jwt.GenerateTokenWithLogin(4243, "ev.admin", "ev.admin", "Admin", true, 0)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPluginAPIRoutes(router.Group("/api/v1"))

	w := subscribe(t, router, "/api/v1/plugins/evauthz/events/ops", agentTok)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "event: connected")
	args := gated.lastArgs()
	require.NotNil(t, args, "authorizer was not called")
	assert.Equal(t, "ops", args["channel"])
	assert.EqualValues(t, 4242, args["_user_id"])
	assert.Equal(t, "Agent", args["_user_role"])
	assert.Equal(t, false, args["_is_admin"])

	w = subscribe(t, router, "/api/v1/plugins/evauthz/events/ops", adminTok)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "event: connected")

	w = subscribe(t, router, "/api/v1/plugins/evauthz/events/public", agentTok)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "event: connected")

	w = subscribe(t, router, "/api/v1/plugins/evauthz/events/boom", adminTok)
	assert.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "event: connected")

	w = subscribe(t, router, "/api/v1/plugins/evlegacy/events/anything", agentTok)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "event: connected")
	assert.Nil(t, legacy.lastArgs(), "plugin without EventAuthorizer is not asked")

	assert.Equal(t, 0, broker.ClientCount(), "every stream closed again")
}
