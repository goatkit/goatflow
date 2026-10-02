package pluginui

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func rateLimitedUI(uiType string, rateLimit int) PluginUI {
	cfgJSON, _ := json.Marshal(UIConfig{Routes: []UIRouteConfig{{Path: "/", Handler: "home"}}, RateLimit: rateLimit})
	cfgRaw := json.RawMessage(cfgJSON)
	id := fmt.Sprintf("rl_%d", time.Now().UnixNano())
	return PluginUI{
		PluginName: "rl", UIID: id, FullID: id, Name: "Rate limited",
		UIType: uiType, Shell: ShellNone, Config: &cfgRaw, Enabled: true, ValidID: 1,
	}
}

func hitUI(eng *gin.Engine, ui PluginUI, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/"+ui.FullID+"/", nil)
	req.RemoteAddr = ip + ":40000"
	eng.ServeHTTP(w, req)
	return w
}

// A public UI's rate_limit (requests per minute per client) is enforced;
// an unset rate_limit falls back to DefaultPublicRateLimit.
func TestPublicUIRateLimitEnforced(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name      string
		rateLimit int
		budget    int
	}{
		{"configured", 3, 3},
		{"default", 0, DefaultPublicRateLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ui := rateLimitedUI(TypePublicPage, tc.rateLimit)
			eng := gin.New()
			if err := registerOneUI(eng, ui, nil, &mockCaller{}, &mockRenderer{}, headerAuth(), slog.Default()); err != nil {
				t.Fatalf("registerOneUI: %v", err)
			}
			for i := range tc.budget {
				if w := hitUI(eng, ui, "198.51.100.7"); w.Code != http.StatusOK {
					t.Fatalf("request %d: status %d, want 200", i+1, w.Code)
				}
			}
			w := hitUI(eng, ui, "198.51.100.7")
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("request over budget: status %d, want 429", w.Code)
			}
			if w.Header().Get("Retry-After") == "" {
				t.Error("429 without Retry-After")
			}
			if w := hitUI(eng, ui, "198.51.100.8"); w.Code != http.StatusOK {
				t.Errorf("other client: status %d, want 200", w.Code)
			}
		})
	}
}
