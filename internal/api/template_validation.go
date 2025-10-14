package api

import (
    "os"
    "path/filepath"
    "strings"

    "github.com/flosch/pongo2/v6"
)

// ValidateAllTemplates walks templatesDir, parses every .pongo2 file, and returns a list of failures
func ValidateAllTemplates(templatesDir string) ([]string, error) {
    loader, err := pongo2.NewLocalFileSystemLoader(templatesDir)
    if err != nil {
        return nil, err
    }
    set := pongo2.NewSet("templates-validate", loader)

    var failures []string
    walkErr := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
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
    return failures, walkErr
}

// ValidateTemplatesReferencedInRoutes loads YAML route groups from routesDir and attempts to parse
// each route's Template against templatesDir. Returns a list of failures (missing/unparsable).
func ValidateTemplatesReferencedInRoutes(routesDir, templatesDir string) ([]string, error) {
    loader, err := pongo2.NewLocalFileSystemLoader(templatesDir)
    if err != nil {
        return nil, err
    }
    set := pongo2.NewSet("templates-from-yaml", loader)

    docs, err := loadYAMLRouteGroups(routesDir)
    if err != nil {
        return nil, err
    }

    var failures []string
    for _, doc := range docs {
        for _, rt := range doc.Spec.Routes {
            tpl := strings.TrimSpace(rt.Template)
            if tpl == "" { continue }
            if _, perr := set.FromFile(tpl); perr != nil {
                failures = append(failures, tpl+": "+perr.Error())
            }
        }
    }
    return failures, nil
}
