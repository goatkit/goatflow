package api

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/flosch/pongo2/v6"
)

// TestPongo2TemplatesParse walks the templates directory and ensures all .pongo2 files parse.
func TestPongo2TemplatesParse(t *testing.T) {
    // Try to locate the templates directory relative to this package
    candidates := []string{"../../templates", "../templates", "templates"}
    var templatesDir string
    for _, c := range candidates {
        if st, err := os.Stat(c); err == nil && st.IsDir() {
            templatesDir = c
            break
        }
    }
    if templatesDir == "" {
        t.Fatal("templates directory not found from internal/api; tried ../../templates, ../templates, templates")
    }

    loader, lerr := pongo2.NewLocalFileSystemLoader(templatesDir)
    if lerr != nil {
        t.Fatalf("failed to create template loader for %s: %v", templatesDir, lerr)
    }
    set := pongo2.NewSet("test-templates", loader)

    var failures []string
    err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        if info.IsDir() { return nil }
        if !strings.HasSuffix(info.Name(), ".pongo2") { return nil }

        rel, rerr := filepath.Rel(templatesDir, path)
        if rerr != nil {
            failures = append(failures, path+": relpath error: "+rerr.Error())
            return nil
        }
        if _, perr := set.FromFile(rel); perr != nil {
            failures = append(failures, rel+": "+perr.Error())
        }
        return nil
    })
    if err != nil {
        t.Fatalf("error walking templates: %v", err)
    }
    if len(failures) > 0 {
        for _, f := range failures {
            t.Errorf("template parse error: %s", f)
        }
        t.Fatalf("%d template(s) failed to parse", len(failures))
    }
}
