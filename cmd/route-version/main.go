// Package main provides route version management.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/routing"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	routesDir := "./routes"
	if envDir := os.Getenv("ROUTES_DIR"); envDir != "" {
		routesDir = envDir
	}

	// Initialize version manager
	vm := routing.NewRouteVersionManager(routesDir)

	switch command {
	case "list", "ls":
		listVersions(vm)

	case "show":
		if len(os.Args) < 3 {
			fmt.Println("Error: version or hash required")
			fmt.Println("Usage: route-version show <version|hash>")
			os.Exit(1)
		}
		showVersion(vm, os.Args[2])

	case "diff":
		if len(os.Args) < 4 {
			fmt.Println("Error: two versions required")
			fmt.Println("Usage: route-version diff <from> <to>")
			os.Exit(1)
		}
		diffVersions(vm, os.Args[2], os.Args[3])

	case "commit", "create":
		message := "Manual version created"
		if len(os.Args) > 2 {
			message = strings.Join(os.Args[2:], " ")
		}
		createVersion(vm, routesDir, message)

	case "rollback":
		if len(os.Args) < 3 {
			fmt.Println("Error: version or hash required")
			fmt.Println("Usage: route-version rollback <version|hash>")
			os.Exit(1)
		}
		rollbackVersion(vm, os.Args[2])

	case "stats":
		showStats(vm)

	case "validate":
		validateRoutes(routesDir)

	case "graph":
		showVersionGraph(vm)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("🔄 GoatFlow Route Version Manager")
	fmt.Println("")
	fmt.Println("Usage: route-version <command> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list, ls              List all versions")
	fmt.Println("  show <version>        Show details of a specific version")
	fmt.Println("  diff <from> <to>      Show differences between versions")
	fmt.Println("  commit [message]      Create a new version with optional message")
	fmt.Println("  rollback <version>    Rollback to a specific version")
	fmt.Println("  stats                 Show version statistics")
	fmt.Println("  validate              Validate current route files")
	fmt.Println("  graph                 Show version history graph")
	fmt.Println("")
	fmt.Println("Environment Variables:")
	fmt.Println("  ROUTES_DIR            Directory containing route files (default: ./routes)")
}

func listVersions(vm *routing.RouteVersionManager) {
	versions := vm.ListVersions()

	if len(versions) == 0 {
		fmt.Println("No versions found")
		return
	}

	fmt.Printf("📚 Found %d versions\n\n", len(versions))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VERSION\tHASH\tAUTHOR\tDATE\tROUTES\tMESSAGE")
	fmt.Fprintln(w, "-------\t----\t------\t----\t------\t-------")

	for _, v := range versions {
		date := v.Timestamp.Format("2006-01-02 15:04")
		routes := fmt.Sprintf("%d", v.Stats.TotalRoutes)
		message := v.Message
		if len(message) > 40 {
			message = message[:37] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			v.Version, v.Hash[:8], v.Author, date, routes, message)
	}

	w.Flush()
}

func showVersion(vm *routing.RouteVersionManager, versionOrHash string) {
	v, err := vm.GetVersion(versionOrHash)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📋 Version Details\n")
	fmt.Printf("==================\n\n")
	fmt.Printf("Version:    %s\n", v.Version)
	fmt.Printf("Hash:       %s\n", v.Hash)
	fmt.Printf("Author:     %s\n", v.Author)
	fmt.Printf("Date:       %s\n", v.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Message:    %s\n", v.Message)

	if v.ParentHash != "" {
		fmt.Printf("Parent:     %s\n", v.ParentHash[:8])
	}

	fmt.Printf("\n📊 Statistics\n")
	fmt.Printf("-------------\n")
	fmt.Printf("Total Routes:    %d\n", v.Stats.TotalRoutes)
	fmt.Printf("Total Endpoints: %d\n", v.Stats.TotalEndpoints)
	fmt.Printf("Enabled:         %d\n", v.Stats.EnabledRoutes)
	fmt.Printf("Disabled:        %d\n", v.Stats.DisabledRoutes)

	fmt.Printf("\n🔧 Methods:\n")
	for method, count := range v.Stats.MethodBreakdown {
		fmt.Printf("  %s: %d\n", method, count)
	}

	fmt.Printf("\n📁 Namespaces:\n")
	for ns, count := range v.Stats.NamespaceCount {
		fmt.Printf("  %s: %d\n", ns, count)
	}

	fmt.Printf("\n📝 Routes:\n")
	for name, route := range v.Routes {
		status := "✅"
		if !route.Metadata.Enabled {
			status = "⏸️"
		}
		fmt.Printf("  %s %s (%s)\n", status, name, route.Metadata.Namespace)
	}
}

func diffVersions(vm *routing.RouteVersionManager, from, to string) {
	diff, err := vm.DiffVersions(from, to)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔀 Version Diff\n")
	fmt.Printf("===============\n\n")
	fmt.Printf("From: %s\n", diff.FromVersion)
	fmt.Printf("To:   %s\n\n", diff.ToVersion)

	if len(diff.Added) > 0 {
		fmt.Printf("➕ Added (%d):\n", len(diff.Added))
		for _, name := range diff.Added {
			fmt.Printf("   + %s\n", name)
		}
		fmt.Println()
	}

	if len(diff.Modified) > 0 {
		fmt.Printf("✏️  Modified (%d):\n", len(diff.Modified))
		for _, name := range diff.Modified {
			fmt.Printf("   ~ %s\n", name)
			if changes, ok := diff.Changes[name]; ok && len(changes) > 0 {
				for _, change := range changes {
					fmt.Printf("     - %s\n", change)
				}
			}
		}
		fmt.Println()
	}

	if len(diff.Deleted) > 0 {
		fmt.Printf("➖ Deleted (%d):\n", len(diff.Deleted))
		for _, name := range diff.Deleted {
			fmt.Printf("   - %s\n", name)
		}
		fmt.Println()
	}

	if len(diff.Added) == 0 && len(diff.Modified) == 0 && len(diff.Deleted) == 0 {
		fmt.Println("No changes between versions")
	}
}

func createVersion(vm *routing.RouteVersionManager, routesDir, message string) {
	// Load current routes from filesystem
	routes, err := loadRoutesFromDir(routesDir)
	if err != nil {
		fmt.Printf("Error loading routes: %v\n", err)
		os.Exit(1)
	}

	// Create version
	v, err := vm.CreateVersion(routes, message)
	if err != nil {
		fmt.Printf("Error creating version: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Version created successfully\n")
	fmt.Printf("   Version: %s\n", v.Version)
	fmt.Printf("   Hash:    %s\n", v.Hash[:8])
	fmt.Printf("   Routes:  %d\n", v.Stats.TotalRoutes)
	fmt.Printf("   Message: %s\n", v.Message)
}

func rollbackVersion(vm *routing.RouteVersionManager, versionOrHash string) {
	// Confirm rollback
	fmt.Printf("⚠️  Are you sure you want to rollback to version %s?\n", versionOrHash)
	fmt.Printf("This will overwrite current route files.\n")
	fmt.Printf("Type 'yes' to confirm: ")

	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" {
		fmt.Println("Rollback cancelled")
		return
	}

	// Perform rollback
	if err := vm.Rollback(versionOrHash); err != nil {
		fmt.Printf("Error during rollback: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully rolled back to version %s\n", versionOrHash)
	fmt.Println("   Route files have been updated")
	fmt.Println("   Restart your services to apply changes")
}

func showStats(vm *routing.RouteVersionManager) {
	versions := vm.ListVersions()

	if len(versions) == 0 {
		fmt.Println("No versions found")
		return
	}

	fmt.Println("📊 Version Statistics")
	fmt.Println("====================")
	fmt.Printf("\nTotal Versions: %d\n", len(versions))

	// Find version with most routes
	maxRoutes := 0
	var maxRoutesVersion *routing.RouteVersion

	// Find version with most changes
	authors := make(map[string]int)

	for _, v := range versions {
		if v.Stats.TotalRoutes > maxRoutes {
			maxRoutes = v.Stats.TotalRoutes
			maxRoutesVersion = v
		}
		authors[v.Author]++
	}

	if maxRoutesVersion != nil {
		fmt.Printf("\nLargest Version:\n")
		fmt.Printf("  Version: %s\n", maxRoutesVersion.Version)
		fmt.Printf("  Routes:  %d\n", maxRoutesVersion.Stats.TotalRoutes)
		fmt.Printf("  Date:    %s\n", maxRoutesVersion.Timestamp.Format("2006-01-02"))
	}

	fmt.Printf("\nAuthors:\n")
	for author, count := range authors {
		fmt.Printf("  %s: %d versions\n", author, count)
	}

	// Show version frequency
	if len(versions) > 1 {
		oldest := versions[len(versions)-1].Timestamp
		newest := versions[0].Timestamp
		duration := newest.Sub(oldest)

		fmt.Printf("\nVersion History:\n")
		fmt.Printf("  First Version: %s\n", oldest.Format("2006-01-02 15:04"))
		fmt.Printf("  Latest Version: %s\n", newest.Format("2006-01-02 15:04"))
		fmt.Printf("  Time Span: %s\n", formatDuration(duration))

		avgTime := duration / time.Duration(len(versions)-1)
		fmt.Printf("  Avg Time Between Versions: %s\n", formatDuration(avgTime))
	}
}

func validateRoutes(routesDir string) {
	fmt.Println("🔍 Validating Route Files")
	fmt.Println("========================")

	errorCount := 0
	warningCount := 0
	validCount := 0

	err := filepath.Walk(routesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		relPath, _ := filepath.Rel(routesDir, path)

		// Read and parse file
		data, err := os.ReadFile(path) //nolint:gosec // G304 CLI tool
		if err != nil {
			fmt.Printf("❌ %s: Failed to read file\n", relPath)
			errorCount++
			return nil //nolint:nilerr // continue walking on error
		}

		var config routing.RouteConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			fmt.Printf("❌ %s: Invalid YAML: %v\n", relPath, err)
			errorCount++
			return nil
		}

		// Validate configuration
		issues := validateRouteConfig(&config)

		if len(issues) == 0 {
			fmt.Printf("✅ %s: Valid\n", relPath)
			validCount++
		} else {
			for _, issue := range issues {
				if strings.HasPrefix(issue, "Warning") {
					fmt.Printf("⚠️  %s: %s\n", relPath, issue)
					warningCount++
				} else {
					fmt.Printf("❌ %s: %s\n", relPath, issue)
					errorCount++
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("   Valid:    %d files\n", validCount)
	fmt.Printf("   Warnings: %d issues\n", warningCount)
	fmt.Printf("   Errors:   %d issues\n", errorCount)

	if errorCount > 0 {
		os.Exit(1)
	}
}

func showVersionGraph(vm *routing.RouteVersionManager) {
	versions := vm.ListVersions()

	if len(versions) == 0 {
		fmt.Println("No versions found")
		return
	}

	fmt.Println("📈 Version History Graph")
	fmt.Println("=======================")
	fmt.Println()

	// Create simple ASCII graph
	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]

		// Format line
		date := v.Timestamp.Format("2006-01-02")
		msg := v.Message
		if len(msg) > 40 {
			msg = msg[:37] + "..."
		}

		marker := "●"
		if i == 0 {
			marker = "◉" // Current version
		}

		fmt.Printf("%s─%s─[%s] %s (%d routes) - %s\n",
			marker, "─", v.Hash[:8], date, v.Stats.TotalRoutes, msg)

		if i > 0 {
			fmt.Println("│")
		}
	}
}

// Helper functions

func loadRoutesFromDir(dir string) (map[string]*routing.RouteConfig, error) {
	routes := make(map[string]*routing.RouteConfig)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		data, err := os.ReadFile(path) //nolint:gosec // G304 CLI tool
		if err != nil {
			return err
		}

		var config routing.RouteConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		// Use filename without extension as key
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		routes[name] = &config

		return nil
	})

	return routes, err
}

func validateRouteConfig(config *routing.RouteConfig) []string {
	issues := []string{}

	// Check required fields
	if config.APIVersion == "" {
		issues = append(issues, "Missing apiVersion")
	}

	if config.Kind != "Route" && config.Kind != "RouteGroup" {
		issues = append(issues, fmt.Sprintf("Invalid kind: %s", config.Kind))
	}

	if config.Metadata.Name == "" {
		issues = append(issues, "Missing metadata.name")
	}

	if len(config.Spec.Routes) == 0 {
		issues = append(issues, "Warning: No routes defined")
	}

	// Validate routes
	for i, route := range config.Spec.Routes {
		if route.Path == "" {
			issues = append(issues, fmt.Sprintf("Route %d: missing path", i))
		}

		if route.Method == nil {
			issues = append(issues, fmt.Sprintf("Route %d: missing method", i))
		}

		if route.Handler == "" && len(route.Handlers) == 0 {
			issues = append(issues, fmt.Sprintf("Route %d: no handler defined", i))
		}
	}

	return issues
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d minutes", int(d.Minutes()))
}
