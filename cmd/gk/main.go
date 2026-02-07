// Package main provides the GoatKit CLI tool for plugin development.
package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "plugin":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gk plugin <command>")
			fmt.Println("Commands: init")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "init":
			pluginInit()
		default:
			fmt.Printf("Unknown plugin command: %s\n", os.Args[2])
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	case "version", "-v", "--version":
		fmt.Println("gk version 0.7.0")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("GoatKit CLI - Plugin Development Tool")
	fmt.Println()
	fmt.Println("Usage: gk <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  plugin init    Create a new plugin from template")
	fmt.Println("  help           Show this help message")
	fmt.Println("  version        Show version information")
}

func pluginInit() {
	var name, runtime string

	// Get plugin name
	if len(os.Args) > 3 {
		name = os.Args[3]
	} else {
		fmt.Print("Plugin name: ")
		fmt.Scanln(&name)
	}

	if name == "" {
		fmt.Println("Error: plugin name is required")
		os.Exit(1)
	}

	// Sanitize name
	name = strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	// Get runtime type
	if len(os.Args) > 4 {
		runtime = os.Args[4]
	} else {
		fmt.Print("Runtime (wasm/grpc) [wasm]: ")
		fmt.Scanln(&runtime)
		if runtime == "" {
			runtime = "wasm"
		}
	}

	switch runtime {
	case "wasm":
		createWASMPlugin(name)
	case "grpc":
		createGRPCPlugin(name)
	default:
		fmt.Printf("Unknown runtime: %s (use 'wasm' or 'grpc')\n", runtime)
		os.Exit(1)
	}
}

func createWASMPlugin(name string) {
	dir := filepath.Join("plugins", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}

	data := map[string]string{
		"Name":        name,
		"NameTitle":   toTitle(name),
		"NameSnake":   strings.ReplaceAll(name, "-", "_"),
		"Description": "A GoatKit WASM plugin",
	}

	// Create main.go
	writeTemplate(filepath.Join(dir, "main.go"), "templates/wasm_main.go.tmpl", data)

	// Create build.sh
	writeTemplate(filepath.Join(dir, "build.sh"), "templates/wasm_build.sh.tmpl", data)
	os.Chmod(filepath.Join(dir, "build.sh"), 0755)

	// Create README.md
	writeTemplate(filepath.Join(dir, "README.md"), "templates/wasm_readme.md.tmpl", data)

	fmt.Printf("✅ Created WASM plugin: %s/\n", dir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", dir)
	fmt.Println("  ./build.sh")
	fmt.Println()
	fmt.Println("The .wasm file will be built in place - ready for the plugin loader.")
}

func createGRPCPlugin(name string) {
	dir := filepath.Join("plugins", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}

	data := map[string]string{
		"Name":        name,
		"NameTitle":   toTitle(name),
		"NameSnake":   strings.ReplaceAll(name, "-", "_"),
		"Description": "A GoatKit gRPC plugin",
	}

	// Create main.go
	writeTemplate(filepath.Join(dir, "main.go"), "templates/grpc_main.go.tmpl", data)

	// Create build.sh
	writeTemplate(filepath.Join(dir, "build.sh"), "templates/grpc_build.sh.tmpl", data)
	os.Chmod(filepath.Join(dir, "build.sh"), 0755)

	// Create README.md
	writeTemplate(filepath.Join(dir, "README.md"), "templates/grpc_readme.md.tmpl", data)

	fmt.Printf("✅ Created gRPC plugin: %s/\n", dir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", dir)
	fmt.Println("  go build -o plugin")
	fmt.Println()
	fmt.Println("Configure GoatFlow to load the binary from this directory.")
}

func writeTemplate(path, tmplPath string, data any) {
	content, err := templateFS.ReadFile(tmplPath)
	if err != nil {
		fmt.Printf("Error reading template %s: %v\n", tmplPath, err)
		os.Exit(1)
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).Parse(string(content))
	if err != nil {
		fmt.Printf("Error parsing template %s: %v\n", tmplPath, err)
		os.Exit(1)
	}

	f, err := os.Create(path)
	if err != nil {
		fmt.Printf("Error creating file %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		fmt.Printf("Error executing template %s: %v\n", tmplPath, err)
		os.Exit(1)
	}
}

func toTitle(s string) string {
	words := strings.Split(s, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}
