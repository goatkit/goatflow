package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/pluginui"
)

func TestBuildPluginUIAdminResponse(t *testing.T) {
	raw := json.RawMessage(`{
		"branding":{"app_name":"Ops Console","color":"#123456"},
		"pwa":{"enabled":true}
	}`)
	domain := "ops.example.com"
	ui := &pluginui.PluginUI{
		ID:           42,
		PluginName:   "ops",
		UIID:         "console",
		FullID:       "ops_console",
		Name:         "Ops Console",
		UIType:       pluginui.TypeAgentApp,
		Shell:        pluginui.ShellStandard,
		Config:       &raw,
		Enabled:      true,
		ValidID:      1,
		CustomDomain: &domain,
		ChangeTime:   time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC),
	}

	got := buildPluginUIAdminResponse(ui)
	if got.ID != 42 || got.FullID != "ops_console" {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if !got.Active || !got.Enabled {
		t.Fatalf("expected active enabled UI: %+v", got)
	}
	if got.BasePath != "/ui/ops_console/" {
		t.Errorf("BasePath = %q", got.BasePath)
	}
	if got.ManifestPath != "/ui/ops_console/manifest.json" {
		t.Errorf("ManifestPath = %q", got.ManifestPath)
	}
	if got.Branding == nil || got.Branding.AppName != "Ops Console" || got.Branding.Color != "#123456" {
		t.Fatalf("unexpected branding: %+v", got.Branding)
	}
	if got.CustomDomain == nil || *got.CustomDomain != domain {
		t.Fatalf("unexpected custom domain: %+v", got.CustomDomain)
	}
}

func TestRegisterPluginAPIRoutesIncludesPluginUIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterPluginAPIRoutes panicked: %v", r)
		}
	}()

	RegisterPluginAPIRoutes(group)
}
