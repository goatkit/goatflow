package swconfig

import (
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goatkit/goatflow/internal/platform/pluginui"
)

func TestBuildDefaultsWithoutDB(t *testing.T) {
	cfg := Build(nil, nil)

	if !cfg.Enabled {
		t.Fatal("expected service worker enabled by default")
	}
	if cfg.DefaultNavigationStrategy != StrategyNetworkFirst {
		t.Fatalf("default navigation strategy = %q", cfg.DefaultNavigationStrategy)
	}
	if cfg.OfflineURL != DefaultOfflineURL {
		t.Fatalf("offline URL = %q", cfg.OfflineURL)
	}
	if len(cfg.PrecacheURLs) == 0 {
		t.Fatal("expected default precache URLs")
	}
	if cfg.Version == "" || cfg.CacheName == "" {
		t.Fatalf("expected version and cache name, got %+v", cfg)
	}
}

func TestBuildIgnoresInvalidGlobalRoutesJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	expectSysconfigValue(mock, "ServiceWorker::Enabled", "true")
	expectSysconfigValue(mock, "ServiceWorker::DefaultNavigationStrategy", "network-first")
	expectSysconfigValue(mock, "ServiceWorker::Routes", "{bad json")

	cfg := Build(db, nil)
	if len(cfg.Routes) != 0 {
		t.Fatalf("expected no routes for invalid JSON, got %+v", cfg.Routes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildFallsBackOnInvalidDefaultStrategy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	expectSysconfigValue(mock, "ServiceWorker::Enabled", "true")
	expectSysconfigValue(mock, "ServiceWorker::DefaultNavigationStrategy", "surprise-me")
	expectSysconfigValue(mock, "ServiceWorker::Routes", "[]")

	cfg := Build(db, nil)
	if cfg.DefaultNavigationStrategy != StrategyNetworkFirst {
		t.Fatalf("expected fallback strategy %q, got %q", StrategyNetworkFirst, cfg.DefaultNavigationStrategy)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildFiltersGlobalRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	routes := []RouteRule{
		{Path: "/dashboard", Strategy: StrategyNetworkFirst},
		{Path: "tickets/", Strategy: StrategyStaleWhileRevalidate},
		{Path: "https://example.com/outside", Strategy: StrategyCacheFirst},
		{Path: "/bad", Strategy: "mystery"},
	}
	data, _ := json.Marshal(routes)

	expectSysconfigValue(mock, "ServiceWorker::Enabled", "true")
	expectSysconfigValue(mock, "ServiceWorker::DefaultNavigationStrategy", "cache-first")
	expectSysconfigValue(mock, "ServiceWorker::Routes", string(data))

	cfg := Build(db, nil)
	if cfg.DefaultNavigationStrategy != StrategyCacheFirst {
		t.Fatalf("strategy = %q", cfg.DefaultNavigationStrategy)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("expected 2 valid routes, got %+v", cfg.Routes)
	}
	if cfg.Routes[0].Path != "/dashboard" || cfg.Routes[1].Path != "/tickets/" {
		t.Fatalf("unexpected routes: %+v", cfg.Routes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPluginUIRulesNormalizeAndFilter(t *testing.T) {
	enabledPWA := rawConfig(`{
		"routes":[
			{"path":"/","handler":"home"},
			{"path":"/offline","handler":"offline"},
			{"path":"/submit","method":"POST","handler":"submit"}
		],
		"pwa":{"enabled":true,"cache_routes":["/","offline","/submit","../escape","https://example.com/nope"]}
	}`)
	pwaOff := rawConfig(`{"pwa":{"enabled":false,"cache_routes":["/"]}}`)

	rules := PluginUIRules([]pluginui.PluginUI{
		{FullID: "demo_app", Config: enabledPWA, Enabled: true, ValidID: 1},
		{FullID: "demo_disabled", Config: enabledPWA, Enabled: false, ValidID: 1},
		{FullID: "demo_pwaoff", Config: pwaOff, Enabled: true, ValidID: 1},
	})

	if len(rules) != 2 {
		t.Fatalf("expected 2 cacheable plugin routes, got %+v", rules)
	}
	if rules[0].Path != "/ui/demo_app/" || rules[1].Path != "/ui/demo_app/offline" {
		t.Fatalf("unexpected plugin rules: %+v", rules)
	}
	for _, rule := range rules {
		if rule.Strategy != StrategyNetworkFirst {
			t.Fatalf("plugin rule strategy = %q", rule.Strategy)
		}
		if rule.Source != "plugin:demo_app" {
			t.Fatalf("plugin rule source = %q", rule.Source)
		}
	}
}

func expectSysconfigValue(mock sqlmock.Sqlmock, name, value string) {
	mock.ExpectQuery(`(?s)SELECT effective_value.*FROM sysconfig_modified`).
		WithArgs(name).
		WillReturnRows(sqlmock.NewRows([]string{"effective_value"}).AddRow(value))
}

func rawConfig(s string) *json.RawMessage {
	raw := json.RawMessage(s)
	return &raw
}
