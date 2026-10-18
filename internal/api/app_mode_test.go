package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

func TestAgentLandingRedirectPrecedence(t *testing.T) {
	t.Cleanup(func() {
		shared.SetAppModeProvider(nil)
		shared.SetLandingPageProvider(nil)
	})

	shared.SetAppModeProvider(nil)
	shared.SetLandingPageProvider(nil)
	if got := agentLandingRedirect(7); got != "/dashboard" {
		t.Fatalf("no providers: %q, want /dashboard", got)
	}

	shared.SetLandingPageProvider(func() string { return "/everyone" })
	if got := agentLandingRedirect(7); got != "/everyone" {
		t.Fatalf("global landing page: %q", got)
	}

	shared.SetAppModeProvider(func(userID uint) *shared.AppModeView {
		if userID == 7 {
			return &shared.AppModeView{LandingPage: "/ui/coach_app/"}
		}
		if userID == 8 {
			return &shared.AppModeView{} // in app mode, but the plugin set no landing page
		}
		return nil
	})
	if got := agentLandingRedirect(7); got != "/ui/coach_app/" {
		t.Fatalf("app mode landing page: %q", got)
	}
	if got := agentLandingRedirect(8); got != "/everyone" {
		t.Fatalf("app mode without landing page falls back to the global one: %q", got)
	}
	if got := agentLandingRedirect(9); got != "/everyone" {
		t.Fatalf("agent outside app mode: %q", got)
	}
	if got := agentLandingRedirect(0); got != "/everyone" {
		t.Fatalf("unknown user never resolves an app mode: %q", got)
	}
}

func TestHandlePluginAppModeToggleValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prev := pluginManager
	t.Cleanup(func() { pluginManager = prev })
	pluginManager = plugin.NewManager(nil)

	post := func(name, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "name", Value: name}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/plugins/"+name+"/app-mode", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		HandlePluginAppModeToggle(c)
		return w
	}

	if w := post("coach", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing enabled: %d %s", w.Code, w.Body.String())
	}
	if w := post("coach", `{"enabled": "yes"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("non-boolean enabled: %d %s", w.Code, w.Body.String())
	}
	if w := post("nope", `{"enabled": true}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown plugin: %d %s", w.Code, w.Body.String())
	}
}
