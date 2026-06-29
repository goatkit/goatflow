package swconfig

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"path"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/pluginui"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

const (
	StrategyNetworkFirst          = "network-first"
	StrategyCacheFirst            = "cache-first"
	StrategyStaleWhileRevalidate  = "stale-while-revalidate"
	StrategyNetworkOnly           = "network-only"
	DefaultOfflineURL             = "/static/offline.html"
	defaultGlobalRoutesConfigName = "ServiceWorker::Routes"
)

var DefaultPrecacheURLs = []string{
	DefaultOfflineURL,
	"/static/css/output.css",
	"/static/css/fonts.css",
	"/static/js/alpine.min.js",
	"/static/js/htmx.min.js",
	"/static/js/common.js",
	"/static/images/goatflow-logo.svg",
	"/static/images/icon-192.png",
}

// RouteRule describes one same-origin GET path handled by the service worker.
type RouteRule struct {
	Path     string `json:"path"`
	Strategy string `json:"strategy"`
	Source   string `json:"source,omitempty"`
}

// Config is the JSON document consumed by /sw.js.
type Config struct {
	Enabled                   bool        `json:"enabled"`
	Version                   string      `json:"version"`
	CacheName                 string      `json:"cache_name"`
	OfflineURL                string      `json:"offline_url"`
	PrecacheURLs              []string    `json:"precache_urls"`
	DefaultNavigationStrategy string      `json:"default_navigation_strategy"`
	Routes                    []RouteRule `json:"routes"`
}

// Build creates a complete service-worker config from sysconfig and plugin UI state.
func Build(db *sql.DB, uis []pluginui.PluginUI) Config {
	cfg := Config{
		Enabled:                   boolValue(db, "ServiceWorker::Enabled", true),
		OfflineURL:                DefaultOfflineURL,
		PrecacheURLs:              append([]string(nil), DefaultPrecacheURLs...),
		DefaultNavigationStrategy: strategyValue(db, "ServiceWorker::DefaultNavigationStrategy", StrategyNetworkFirst),
	}
	cfg.Routes = append(cfg.Routes, globalRules(db)...)
	cfg.Routes = append(cfg.Routes, PluginUIRules(uis)...)
	cfg.Version = hashConfig(cfg)
	cfg.CacheName = "goatflow-" + cfg.Version
	return cfg
}

// PluginUIRules converts enabled plugin UI PWA cache routes into absolute rules.
func PluginUIRules(uis []pluginui.PluginUI) []RouteRule {
	var rules []RouteRule
	for _, ui := range uis {
		if !ui.IsActive() {
			continue
		}
		cfg, err := ui.ParsedConfig()
		if err != nil || cfg.PWA == nil || !cfg.PWA.Enabled {
			continue
		}
		for _, raw := range cfg.PWA.CacheRoutes {
			if !cacheRouteAllowsGET(cfg, raw) {
				continue
			}
			if p, ok := normalizePluginRoute(ui.BasePath(), raw); ok {
				rules = append(rules, RouteRule{
					Path:     p,
					Strategy: StrategyNetworkFirst,
					Source:   "plugin:" + ui.FullID,
				})
			}
		}
	}
	return dedupeRules(rules)
}

func cacheRouteAllowsGET(cfg *pluginui.UIConfig, raw string) bool {
	cachePath, ok := normalizeSameOriginPath(raw)
	if !ok {
		return false
	}
	for _, route := range cfg.Routes {
		routePath := route.Path
		if routePath == "" {
			routePath = "/"
		}
		normalizedRoute, ok := normalizeSameOriginPath(routePath)
		if !ok || normalizedRoute != cachePath {
			continue
		}
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		return method == "" || method == "GET"
	}
	return true
}

func globalRules(db *sql.DB) []RouteRule {
	raw, ok := sysconfig.Value(db, defaultGlobalRoutesConfigName)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	var configured []RouteRule
	if err := json.Unmarshal([]byte(raw), &configured); err != nil {
		return nil
	}
	var rules []RouteRule
	for _, rule := range configured {
		p, ok := normalizeSameOriginPath(rule.Path)
		if !ok {
			continue
		}
		strategy := rule.Strategy
		if !ValidStrategy(strategy) {
			continue
		}
		rules = append(rules, RouteRule{Path: p, Strategy: strategy, Source: "global"})
	}
	return dedupeRules(rules)
}

func boolValue(db *sql.DB, name string, fallback bool) bool {
	raw, ok := sysconfig.Value(db, name)
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func strategyValue(db *sql.DB, name, fallback string) string {
	raw, ok := sysconfig.Value(db, name)
	if !ok || !ValidStrategy(raw) {
		return fallback
	}
	return raw
}

// ValidStrategy reports whether s is one of the v1 fixed strategy values.
func ValidStrategy(s string) bool {
	switch strings.TrimSpace(s) {
	case StrategyNetworkFirst, StrategyCacheFirst, StrategyStaleWhileRevalidate, StrategyNetworkOnly:
		return true
	default:
		return false
	}
}

func normalizePluginRoute(basePath, raw string) (string, bool) {
	route, ok := normalizeSameOriginPath(raw)
	if !ok {
		return "", false
	}
	base := strings.TrimRight(basePath, "/")
	if route == "/" {
		route = base + "/"
	} else {
		route = base + route
	}
	route = path.Clean(route)
	if strings.HasSuffix(raw, "/") && !strings.HasSuffix(route, "/") {
		route += "/"
	}
	if route == base {
		route = base + "/"
	}
	if !strings.HasPrefix(route, base+"/") {
		return "", false
	}
	return route, true
}

func normalizeSameOriginPath(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "..") {
		return "", false
	}
	if strings.HasPrefix(raw, "//") {
		return "", false
	}
	if u, err := url.Parse(raw); err != nil || u.IsAbs() || u.Host != "" {
		return "", false
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	cleaned := path.Clean(raw)
	if raw == "/" || cleaned == "." {
		return "/", true
	}
	if strings.HasSuffix(raw, "/") && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned, true
}

func dedupeRules(rules []RouteRule) []RouteRule {
	seen := make(map[string]bool, len(rules))
	out := make([]RouteRule, 0, len(rules))
	for _, rule := range rules {
		key := rule.Path + "\x00" + rule.Strategy
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, rule)
	}
	return out
}

func hashConfig(cfg Config) string {
	hashable := cfg
	hashable.Version = ""
	hashable.CacheName = ""
	data, _ := json.Marshal(hashable)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}
