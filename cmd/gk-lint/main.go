package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const platformPrefix = "internal/platform"

type allowRule struct {
	Import string
	Reason string
}

var allowedProductImports = []allowRule{}

type violation struct {
	Kind   string
	File   string
	Line   int
	Import string
}

func main() {
	root, modulePath, err := moduleRoot()
	if err != nil {
		fatal(err)
	}

	productPackages, err := discoverProductPackages(root, modulePath)
	if err != nil {
		fatal(err)
	}

	violations, err := scanDirectImports(root, modulePath)
	if err != nil {
		fatal(err)
	}

	transitive, err := scanTransitiveImports(root, modulePath)
	if err != nil {
		fatal(err)
	}
	violations = append(violations, transitive...)

	if len(violations) > 0 {
		printViolations(violations)
		os.Exit(1)
	}

	fmt.Printf("platform boundary: OK (%d product packages checked)\n", len(productPackages))
}

func moduleRoot() (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	for dir := cwd; ; dir = filepath.Dir(dir) {
		goMod := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(goMod)
		if err == nil {
			modulePath, err := parseModulePath(data)
			if err != nil {
				return "", "", fmt.Errorf("parse %s: %w", goMod, err)
			}
			return dir, modulePath, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("go.mod not found from %s", cwd)
		}
	}
}

func parseModulePath(data []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if modulePath == "" {
				return "", errors.New("empty module path")
			}
			return modulePath, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("module directive not found")
}

func discoverProductPackages(root, modulePath string) (map[string]string, error) {
	packages := make(map[string]string)
	internalRoot := filepath.Join(root, "internal")

	err := filepath.WalkDir(internalRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != internalRoot && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == platformPrefix || strings.HasPrefix(rel, platformPrefix+"/") {
			return filepath.SkipDir
		}
		if rel == "internal" {
			return nil
		}

		hasGo, err := directoryHasGoFile(path)
		if err != nil {
			return err
		}
		if hasGo {
			importPath := modulePath + "/" + rel
			packages[importPath] = rel
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return packages, nil
}

func directoryHasGoFile(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") {
			return true, nil
		}
	}
	return false, nil
}

func scanDirectImports(root, modulePath string) ([]violation, error) {
	platformRoot := filepath.Join(root, platformPrefix)
	violations := make([]violation, 0)
	fset := token.NewFileSet()

	err := filepath.WalkDir(platformRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != platformRoot && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, "\"")
			if _, ok := allowedImport(importPath); ok {
				continue
			}
			if isProductPackage(importPath, modulePath) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				violations = append(violations, violation{
					Kind:   "direct",
					File:   filepath.ToSlash(rel),
					Line:   fset.Position(spec.Pos()).Line,
					Import: importPath,
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return violations, nil
}

func scanTransitiveImports(root, modulePath string) ([]violation, error) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./internal/platform/...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go list platform dependency closure: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	seen := make(map[string]struct{})
	violations := make([]violation, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		importPath := strings.TrimSpace(scanner.Text())
		if importPath == "" {
			continue
		}
		if _, ok := seen[importPath]; ok {
			continue
		}
		seen[importPath] = struct{}{}
		if _, ok := allowedImport(importPath); ok {
			continue
		}
		if isProductPackage(importPath, modulePath) {
			violations = append(violations, violation{
				Kind:   "transitive",
				Import: importPath,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return violations, nil
}

func isProductPackage(importPath, modulePath string) bool {
	internalPrefix := modulePath + "/internal/"
	platformImport := modulePath + "/" + platformPrefix
	return strings.HasPrefix(importPath, internalPrefix) &&
		importPath != platformImport &&
		!strings.HasPrefix(importPath, platformImport+"/")
}

func allowedImport(importPath string) (string, bool) {
	for _, rule := range allowedProductImports {
		if importPath == rule.Import || strings.HasPrefix(importPath, rule.Import+"/") {
			return rule.Reason, true
		}
	}
	return "", false
}

func printViolations(violations []violation) {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Kind != violations[j].Kind {
			return violations[i].Kind < violations[j].Kind
		}
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		return violations[i].Import < violations[j].Import
	})

	fmt.Fprintf(os.Stderr, "platform boundary violations: %d\n", len(violations))
	for _, v := range violations {
		switch v.Kind {
		case "direct":
			fmt.Fprintf(os.Stderr, "  direct: %s:%d imports product package %s\n", v.File, v.Line, v.Import)
		case "transitive":
			fmt.Fprintf(os.Stderr, "  transitive: internal/platform dependency closure includes product package %s\n", v.Import)
		default:
			fmt.Fprintf(os.Stderr, "  %s: %s %s\n", v.Kind, v.File, v.Import)
		}
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "gk-lint: %v\n", err)
	os.Exit(2)
}
