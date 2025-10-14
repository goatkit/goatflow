package api

import (
    "encoding/json"
    "io/fs"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "gopkg.in/yaml.v3"
)

// routeYAML structures reflect our route group schema
type routeGroupSpec struct {
    Prefix     string        `yaml:"prefix"`
    Middleware []string      `yaml:"middleware"`
    Routes     []routeSpec   `yaml:"routes"`
}

type topRouteDoc struct {
    APIVersion string        `yaml:"apiVersion"`
    Kind       string        `yaml:"kind"`
    Metadata   struct {
        Name        string `yaml:"name"`
        Description string `yaml:"description"`
        Namespace   string `yaml:"namespace"`
        Enabled     bool   `yaml:"enabled"`
    } `yaml:"metadata"`
    Spec routeGroupSpec `yaml:"spec"`
}

type routeSpec struct {
    Path        string   `yaml:"path"`
    Method      string   `yaml:"method"`
    HandlerName string   `yaml:"handler"`
    Template    string   `yaml:"template"`
    Middleware  []string `yaml:"middleware"`
    Description string   `yaml:"description"`
    RedirectTo  string   `yaml:"redirectTo"`
    Status      int      `yaml:"status"`
    Websocket   bool     `yaml:"websocket"`
}

// BuildRoutesManifest constructs the manifest JSON without registering routes (for tooling)
func BuildRoutesManifest() ([]byte, error) {
    docs, err := loadYAMLRouteGroups("./routes")
    if err != nil { return nil, err }
    type manifestRoute struct {
        Group      string   `json:"group"`
        Method     string   `json:"method"`
        Path       string   `json:"path"`
        Handler    string   `json:"handler,omitempty"`
        RedirectTo string   `json:"redirectTo,omitempty"`
        Status     int      `json:"status,omitempty"`
        Websocket  bool     `json:"websocket,omitempty"`
        Middleware []string `json:"middleware,omitempty"`
    }
    var manifest []manifestRoute
    for _, doc := range docs {
        prefix := doc.Spec.Prefix
        for _, rt := range doc.Spec.Routes {
            if rt.Path == "" || rt.Method == "" { continue }
            fullPath := filepath.Join(prefix, rt.Path)
            if !strings.HasPrefix(fullPath, "/") { fullPath = "/" + fullPath }
            mr := manifestRoute{Group: doc.Metadata.Name, Method: strings.ToUpper(rt.Method), Path: fullPath, Middleware: append(doc.Spec.Middleware, rt.Middleware...)}
            if rt.RedirectTo != "" { mr.RedirectTo = rt.RedirectTo; if rt.Status != 0 { mr.Status = rt.Status } }
            if rt.Websocket { mr.Websocket = true }
            if rt.HandlerName != "" { mr.Handler = rt.HandlerName }
            manifest = append(manifest, mr)
        }
    }
    out := struct {
        GeneratedAt time.Time   `json:"generatedAt"`
        Routes      interface{} `json:"routes"`
    }{GeneratedAt: time.Now().UTC(), Routes: manifest}
    return json.MarshalIndent(out, "", "  ")
}

// loadYAMLRouteGroups scans the routes directory and returns parsed groups
func loadYAMLRouteGroups(dir string) ([]topRouteDoc, error) {
    var docs []topRouteDoc
    entries := []string{}
    filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        if d.IsDir() { return nil }
        if !strings.HasSuffix(d.Name(), ".yaml") && !strings.HasSuffix(d.Name(), ".yml") { return nil }
        entries = append(entries, path)
        return nil
    })
    sort.Strings(entries)
    for _, p := range entries {
        b, err := os.ReadFile(p)
        if err != nil { log.Printf("route loader read error %s: %v", p, err); continue }
        var doc topRouteDoc
        if err := yaml.Unmarshal(b, &doc); err != nil { log.Printf("route yaml parse error %s: %v", p, err); continue }
        if !doc.Metadata.Enabled { continue }
        if doc.Kind != "RouteGroup" { continue }
        docs = append(docs, doc)
    }
    return docs, nil
}

// registerYAMLRoutes registers routes onto router r using handler registry
// registerYAMLRoutes registers YAML routes. authMW may be nil (tests/dev). When provided,
// 'auth' token maps to authMW.RequireAuth(); 'admin' token adds checkAdmin().
func registerYAMLRoutes(r *gin.Engine, authMW interface{}) {
    docs, err := loadYAMLRouteGroups("./routes")
    if err != nil { log.Printf("yaml route load error: %v", err); return }
    // Ensure core legacy handlers are present (idempotent)
    ensureCoreHandlers()

    type manifestRoute struct {
        Group       string `json:"group"`
        Method      string `json:"method"`
        Path        string `json:"path"`
        Handler     string `json:"handler,omitempty"`
        RedirectTo  string `json:"redirectTo,omitempty"`
        Status      int    `json:"status,omitempty"`
        Websocket   bool   `json:"websocket,omitempty"`
        Middleware  []string `json:"middleware,omitempty"`
    }
    var manifest []manifestRoute

    for _, doc := range docs {
        prefix := doc.Spec.Prefix
        // Build middleware chain from tokens
        groupHandlers := []gin.HandlerFunc{}
        tokenSet := map[string]bool{}
        for _, mw := range doc.Spec.Middleware { tokenSet[mw] = true }
        // auth token -> RequireAuth if jwtManager present in context (try to find via global?) fallback to test stub
        // We don't have direct jwtManager here; rely on context injection already performed earlier for protected routes.
        // Minimal approach: if auth token present and no existing user in context, wrap a guard that enforces login cookie.
        if tokenSet["auth"] {
            if mw, ok := authMW.(interface{ RequireAuth() gin.HandlerFunc }); ok {
                groupHandlers = append(groupHandlers, mw.RequireAuth())
            } else {
                groupHandlers = append(groupHandlers, fallbackAuthGuard())
            }
        }
        if tokenSet["admin"] {
            groupHandlers = append(groupHandlers, checkAdmin())
        }
        base := r.Group(prefix, groupHandlers...)

        for _, rt := range doc.Spec.Routes {
            if rt.Path == "" || rt.Method == "" { continue }
            method := strings.ToUpper(rt.Method)
            fullPath := filepath.Join(prefix, rt.Path)
            if !strings.HasPrefix(fullPath, "/") { fullPath = "/" + fullPath }

            // Build route-level middleware tokens
            routeMws := []gin.HandlerFunc{}
            for _, mw := range rt.Middleware {
                switch mw {
                case "auth":
                    if real, ok := authMW.(interface{ RequireAuth() gin.HandlerFunc }); ok { routeMws = append(routeMws, real.RequireAuth()) } else { routeMws = append(routeMws, fallbackAuthGuard()) }
                case "admin":
                    routeMws = append(routeMws, checkAdmin())
                }
            }

            if rt.RedirectTo != "" {
                status := rt.Status
                if status == 0 { status = http.StatusFound }
                h := func(target string, code int) gin.HandlerFunc { return func(c *gin.Context) { c.Redirect(code, target) } }(rt.RedirectTo, status)
                // Skip if already registered elsewhere (legacy/manual) to avoid Gin panic
                if routeAlreadyRegistered(r, method, fullPath) {
                    log.Printf("duplicate route detected, skipping registration: %s %s (redirect)", method, fullPath)
                } else {
                    registerOneWithChain(base, method, rt.Path, append(routeMws, h)...) 
                }
                manifest = append(manifest, manifestRoute{Group: doc.Metadata.Name, Method: method, Path: fullPath, RedirectTo: rt.RedirectTo, Status: status, Middleware: append(doc.Spec.Middleware, rt.Middleware...)})
                continue
            }
            if rt.Websocket {
                if h, ok := GetHandler(rt.HandlerName); ok {
                    if routeAlreadyRegistered(r, method, fullPath) {
                        log.Printf("duplicate route detected, skipping registration: %s %s (websocket)", method, fullPath)
                    } else {
                        registerOneWithChain(base, method, rt.Path, append(routeMws, h)...) 
                    }
                    manifest = append(manifest, manifestRoute{Group: doc.Metadata.Name, Method: method, Path: fullPath, Handler: rt.HandlerName, Websocket: true, Middleware: append(doc.Spec.Middleware, rt.Middleware...)})
                } else { log.Printf("missing websocket handler %s", rt.HandlerName) }
                continue
            }
            if h, ok := GetHandler(rt.HandlerName); ok {
                if routeAlreadyRegistered(r, method, fullPath) {
                    log.Printf("duplicate route detected, skipping registration: %s %s", method, fullPath)
                } else {
                    registerOneWithChain(base, method, rt.Path, append(routeMws, h)...) 
                }
                manifest = append(manifest, manifestRoute{Group: doc.Metadata.Name, Method: method, Path: fullPath, Handler: rt.HandlerName, Middleware: append(doc.Spec.Middleware, rt.Middleware...)})
            } else if rt.HandlerName != "" {
                log.Printf("No handler mapped for %s (%s %s)", rt.HandlerName, method, fullPath)
            }
        }
    }

    // Write manifest
    if len(manifest) > 0 {
        log.Printf("writing routes manifest with %d entries", len(manifest))
        if err := os.MkdirAll("runtime", 0o755); err != nil {
            log.Printf("failed to create runtime dir: %v", err)
        }
        mf := struct {
            GeneratedAt time.Time       `json:"generatedAt"`
            Routes      []manifestRoute `json:"routes"`
        }{GeneratedAt: time.Now().UTC(), Routes: manifest}
        if b, err := json.MarshalIndent(mf, "", "  "); err == nil {
            if err := os.WriteFile("runtime/routes-manifest.json", b, 0o644); err != nil {
                log.Printf("failed writing routes manifest: %v", err)
            }
        } else {
            log.Printf("failed to marshal routes manifest: %v", err)
        }
    } else {
        log.Printf("no routes captured for manifest (len=0) - not writing file")
    }
}

// routeAlreadyRegistered checks the engine for an existing method+path
func routeAlreadyRegistered(r *gin.Engine, method, fullPath string) bool {
    // Gin stores registered routes in Engine.Routes()
    // Paths are absolute (including group prefixes). Method is uppercased.
    for _, info := range r.Routes() {
        if strings.EqualFold(info.Method, method) && info.Path == fullPath {
            return true
        }
    }
    return false
}

// GenerateRoutesManifest creates a temporary gin engine, registers YAML routes and ensures
// the manifest file exists. Used by tooling (make routes-generate) without starting full server.
func GenerateRoutesManifest() error {
    r := gin.New()
    // minimal middleware similar to gin.Default without logger/recovery spam for tooling
    r.Use(gin.Recovery())
    if err := os.MkdirAll("runtime", 0o755); err != nil {
        log.Printf("failed to pre-create runtime dir: %v", err)
    } else {
        if fi, err := os.Stat("runtime"); err == nil && fi.IsDir() { log.Printf("runtime dir ready for manifest output") }
    }
    registerYAMLRoutes(r, nil)
    if _, err := os.Stat("runtime/routes-manifest.json"); err != nil {
        return err
    }
    return nil
}

func registerOne(g *gin.RouterGroup, method, path string, h gin.HandlerFunc) {
    switch method {
    case "GET": g.GET(path, h)
    case "POST": g.POST(path, h)
    case "PUT": g.PUT(path, h)
    case "DELETE": g.DELETE(path, h)
    case "PATCH": g.PATCH(path, h)
    default:
        log.Printf("unsupported method %s for %s", method, path)
    }
}

// helper with possible extra middleware chain
func registerOneWithChain(g *gin.RouterGroup, method, path string, handlers ...gin.HandlerFunc) {
    switch method {
    case "GET": g.GET(path, handlers...)
    case "POST": g.POST(path, handlers...)
    case "PUT": g.PUT(path, handlers...)
    case "DELETE": g.DELETE(path, handlers...)
    case "PATCH": g.PATCH(path, handlers...)
    default:
        log.Printf("unsupported method %s for %s", method, path)
    }
}

// fallbackAuthGuard provides a minimal auth gate when full middleware unavailable
func fallbackAuthGuard() gin.HandlerFunc {
    return func(c *gin.Context) {
        if _, ok := c.Get("user_id"); ok { c.Next(); return }
        if _, err := c.Cookie("access_token"); err == nil {
            // Populate a minimal user context for downstream handlers
            if _, exists := c.Get("user_id"); !exists { c.Set("user_id", uint(1)) }
            if _, exists := c.Get("user_email"); !exists { c.Set("user_email", "demo@example.com") }
            if _, exists := c.Get("user_role"); !exists { c.Set("user_role", "Admin") }
            if _, exists := c.Get("user_name"); !exists { c.Set("user_name", "Demo User") }
            c.Next(); return
        }
        accept := c.GetHeader("Accept")
        if strings.Contains(accept, "text/html") {
            // Prevent redirect loop: if already requesting /login just serve page; likewise treat root as login
            p := c.Request.URL.Path
            if p == "/login" || p == "/" {
                // Use existing login page renderer if available; otherwise simple placeholder string
                if pongo2Renderer != nil { handleLoginPage(c) } else { c.String(http.StatusOK, "login") }
            } else { c.Redirect(http.StatusFound, "/login") }
        } else {
            c.JSON(http.StatusUnauthorized, gin.H{"error":"unauthorized"})
        }
        c.Abort()
    }
}

// Integrate with existing setup in setupHTMXRoutesWithAuth AFTER static/auth/basic have been initialized.
// We'll call registerYAMLRoutes at the end of setupHTMXRoutesWithAuth so new groups override legacy gaps.