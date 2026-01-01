// Package main provides configuration management utilities.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/gotrs-io/gotrs-ce/internal/yamlmgmt"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Get storage directory from env or use default
	storageDir := os.Getenv("GOTRS_CONFIG_DIR")
	if storageDir == "" {
		storageDir = "./config"
	}

	// Initialize managers
	versionMgr := yamlmgmt.NewVersionManager(storageDir)
	schemaRegistry := yamlmgmt.NewSchemaRegistry()
	linter := yamlmgmt.NewUniversalLinter()

	// Parse command
	command := os.Args[1]

	switch command {
	case "list", "ls":
		handleList(versionMgr)

	case "show":
		handleShow(versionMgr)

	case "validate":
		handleValidate(schemaRegistry, linter)

	case "lint":
		handleLint(linter)

	case "version":
		handleVersion(versionMgr)

	case "rollback":
		handleRollback(versionMgr)

	case "diff":
		handleDiff(versionMgr)

	case "apply":
		handleApply(versionMgr, schemaRegistry, linter)

	case "watch":
		handleWatch(versionMgr, schemaRegistry)

	case "export":
		handleExport(versionMgr)

	case "import":
		handleImport(versionMgr)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("🔧 GOTRS Unified Configuration Manager")
	fmt.Println("======================================")
	fmt.Println("")
	fmt.Println("Manage all YAML configurations with version control, validation, and hot reload")
	fmt.Println("")
	fmt.Println("Usage: gotrs-config <command> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list [kind]              List all configurations or specific kind")
	fmt.Println("  show <kind> <name>       Show a specific configuration")
	fmt.Println("  validate <file>          Validate a YAML file against schema")
	fmt.Println("  lint <file|dir>          Lint YAML files for best practices")
	fmt.Println("  version <subcommand>     Version management (list, show, create)")
	fmt.Println("  rollback <kind> <name>   Rollback to a previous version")
	fmt.Println("  diff <kind> <name>       Show differences between versions")
	fmt.Println("  apply <file>             Apply a configuration file")
	fmt.Println("  watch                    Watch for configuration changes")
	fmt.Println("  export <kind>            Export configurations to files")
	fmt.Println("  import <dir>             Import configurations from directory")
	fmt.Println("")
	fmt.Println("Kinds:")
	fmt.Println("  route      - API route definitions")
	fmt.Println("  config     - System configuration settings")
	fmt.Println("  dashboard  - Dashboard layouts and widgets")
	fmt.Println("  compose    - Docker compose configurations")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  gotrs-config list config")
	fmt.Println("  gotrs-config show route health-checks")
	fmt.Println("  gotrs-config validate ./config/new-settings.yaml")
	fmt.Println("  gotrs-config lint ./routes/")
	fmt.Println("  gotrs-config version list config SystemID")
	fmt.Println("  gotrs-config rollback config SystemID v2025.1.15-1430")
	fmt.Println("  gotrs-config apply ./config/production.yaml")
	fmt.Println("")
	fmt.Println("Environment Variables:")
	fmt.Println("  GOTRS_CONFIG_DIR    Base directory for configurations (default: ./config)")
}

func handleList(versionMgr *yamlmgmt.VersionManager) {
	var kind yamlmgmt.YAMLKind

	if len(os.Args) > 2 {
		kind = parseKind(os.Args[2])
	}

	fmt.Printf("📋 Configuration List")
	if kind != "" {
		fmt.Printf(" (%s)", kind)
	}
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println()

	kinds := []yamlmgmt.YAMLKind{
		yamlmgmt.KindRoute,
		yamlmgmt.KindConfig,
		yamlmgmt.KindDashboard,
		yamlmgmt.KindCompose,
	}

	if kind != "" {
		kinds = []yamlmgmt.YAMLKind{kind}
	}

	totalCount := 0
	for _, k := range kinds {
		documents, err := versionMgr.ListAll(k)
		if err != nil {
			continue
		}

		if len(documents) > 0 {
			fmt.Printf("📁 %s (%d):\n", k, len(documents))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "  NAME\tVERSION\tMODIFIED\tDESCRIPTION")
			fmt.Fprintln(w, "  ----\t-------\t--------\t-----------")

			for _, doc := range documents {
				modified := "unknown"
				if !doc.Metadata.Modified.IsZero() {
					modified = doc.Metadata.Modified.Format("2006-01-02")
				}

				description := doc.Metadata.Description
				if len(description) > 40 {
					description = description[:37] + "..."
				}

				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
					doc.Metadata.Name,
					doc.Metadata.Version,
					modified,
					description)
			}
			w.Flush()
			fmt.Println()

			totalCount += len(documents)
		}
	}

	fmt.Printf("Total: %d configurations\n", totalCount)
}

func handleShow(versionMgr *yamlmgmt.VersionManager) {
	if len(os.Args) < 4 {
		fmt.Println("Error: kind and name required")
		fmt.Println("Usage: gotrs-config show <kind> <name>")
		os.Exit(1)
	}

	kind := parseKind(os.Args[2])
	name := os.Args[3]

	doc, err := versionMgr.GetCurrent(kind, name)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Output as YAML
	data, err := yaml.Marshal(doc)
	if err != nil {
		fmt.Printf("Error marshaling document: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}

func handleValidate(schemaRegistry *yamlmgmt.SchemaRegistry, linter *yamlmgmt.UniversalLinter) {
	if len(os.Args) < 3 {
		fmt.Println("Error: file path required")
		fmt.Println("Usage: gotrs-config validate <file>")
		os.Exit(1)
	}

	filename := os.Args[2]

	// Load file
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Parse YAML
	var doc yamlmgmt.YAMLDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔍 Validating: %s\n", filename)
	fmt.Println(strings.Repeat("-", 50))

	// Schema validation
	result, err := schemaRegistry.Validate(&doc)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		os.Exit(1)
	}

	if result.Valid {
		fmt.Println("✅ Schema validation: PASSED")
	} else {
		fmt.Println("❌ Schema validation: FAILED")
		for _, err := range result.Errors {
			fmt.Printf("  - %s: %s\n", err.Path, err.Message)
		}
	}

	// Linting
	issues, err := linter.Lint(&doc)
	if err != nil {
		fmt.Printf("Linting error: %v\n", err)
		os.Exit(1)
	}

	if len(issues) == 0 {
		fmt.Println("✅ Linting: No issues found")
	} else {
		fmt.Printf("⚠️  Linting: %d issues found\n", len(issues))
		for _, issue := range issues {
			icon := "ℹ️"
			if issue.Severity == "error" {
				icon = "❌"
			} else if issue.Severity == "warning" {
				icon = "⚠️"
			}
			fmt.Printf("  %s [%s] %s: %s\n", icon, issue.Rule, issue.Path, issue.Message)
		}
	}

	if !result.Valid || hasErrors(issues) {
		os.Exit(1)
	}
}

func handleLint(linter *yamlmgmt.UniversalLinter) {
	if len(os.Args) < 3 {
		fmt.Println("Error: file or directory required")
		fmt.Println("Usage: gotrs-config lint <file|dir>")
		os.Exit(1)
	}

	path := os.Args[2]
	files := []string{}

	// Check if path is file or directory
	info, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if info.IsDir() {
		// Walk directory
		filepath.Walk(path, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !i.IsDir() && (strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
				files = append(files, p)
			}
			return nil
		})
	} else {
		files = append(files, path)
	}

	fmt.Printf("🔍 Linting %d files\n", len(files))
	fmt.Println(strings.Repeat("=", 50))

	totalIssues := 0
	errorCount := 0
	warningCount := 0
	infoCount := 0

	for _, file := range files {
		// Load and parse file
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("❌ %s: Failed to read file\n", file)
			continue
		}

		var doc yamlmgmt.YAMLDocument
		if err := yaml.Unmarshal(data, &doc); err != nil {
			fmt.Printf("❌ %s: Invalid YAML\n", file)
			errorCount++
			continue
		}

		// Lint
		issues, err := linter.Lint(&doc)
		if err != nil {
			fmt.Printf("❌ %s: Linting error: %v\n", file, err)
			continue
		}

		if len(issues) > 0 {
			fmt.Printf("\n📄 %s (%d issues):\n", file, len(issues))
			for _, issue := range issues {
				icon := "ℹ️"
				switch issue.Severity {
				case "error":
					icon = "❌"
					errorCount++
				case "warning":
					icon = "⚠️"
					warningCount++
				default:
					infoCount++
				}
				fmt.Printf("  %s [%s] %s\n", icon, issue.Rule, issue.Message)
			}
			totalIssues += len(issues)
		} else {
			fmt.Printf("✅ %s: No issues\n", file)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("  Files scanned: %d\n", len(files))
	fmt.Printf("  Total issues:  %d\n", totalIssues)
	fmt.Printf("  Errors:        %d\n", errorCount)
	fmt.Printf("  Warnings:      %d\n", warningCount)
	fmt.Printf("  Info:          %d\n", infoCount)

	if errorCount > 0 {
		os.Exit(1)
	}
}

func handleVersion(versionMgr *yamlmgmt.VersionManager) {
	if len(os.Args) < 3 {
		fmt.Println("Error: subcommand required")
		fmt.Println("Usage: gotrs-config version <list|show|create> [options]")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "list", "ls":
		if len(os.Args) < 5 {
			fmt.Println("Error: kind and name required")
			fmt.Println("Usage: gotrs-config version list <kind> <name>")
			os.Exit(1)
		}

		kind := parseKind(os.Args[3])
		name := os.Args[4]

		versions, err := versionMgr.ListVersions(kind, name)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("📚 Version History: %s/%s\n", kind, name)
		fmt.Println(strings.Repeat("=", 50))

		if len(versions) == 0 {
			fmt.Println("No versions found")
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "VERSION\tHASH\tAUTHOR\tDATE\tMESSAGE")
			fmt.Fprintln(w, "-------\t----\t------\t----\t-------")

			for _, v := range versions {
				message := v.Message
				if len(message) > 30 {
					message = message[:27] + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					v.Number,
					v.Hash[:8],
					v.Author,
					v.Timestamp.Format("2006-01-02 15:04"),
					message)
			}
			w.Flush()
		}

	case "show":
		if len(os.Args) < 6 {
			fmt.Println("Error: kind, name, and version required")
			fmt.Println("Usage: gotrs-config version show <kind> <name> <version>")
			os.Exit(1)
		}

		kind := parseKind(os.Args[3])
		name := os.Args[4]
		versionID := os.Args[5]

		version, err := versionMgr.GetVersion(kind, name, versionID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Display version details
		fmt.Printf("📋 Version Details\n")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Printf("Version:    %s\n", version.Number)
		fmt.Printf("Hash:       %s\n", version.Hash)
		fmt.Printf("Author:     %s\n", version.Author)
		fmt.Printf("Timestamp:  %s\n", version.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("Message:    %s\n", version.Message)

		if version.ParentHash != "" {
			fmt.Printf("Parent:     %s\n", version.ParentHash[:8])
		}

		if version.Stats != nil {
			fmt.Printf("\nStatistics:\n")
			fmt.Printf("  Total fields: %d\n", version.Stats.TotalFields)
			if len(version.Stats.CustomStats) > 0 {
				for key, value := range version.Stats.CustomStats {
					title := cases.Title(language.English)
					fmt.Printf("  %s: %d\n", title.String(key), value)
				}
			}
		}

		if len(version.Changes) > 0 {
			fmt.Printf("\nChanges:\n")
			for _, change := range version.Changes {
				fmt.Printf("  %s %s: %s\n", change.Type, change.Path, change.Description)
			}
		}

	default:
		fmt.Printf("Unknown version subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleRollback(versionMgr *yamlmgmt.VersionManager) {
	if len(os.Args) < 5 {
		fmt.Println("Error: kind, name, and version required")
		fmt.Println("Usage: gotrs-config rollback <kind> <name> <version>")
		os.Exit(1)
	}

	kind := parseKind(os.Args[2])
	name := os.Args[3]
	versionID := os.Args[4]

	fmt.Printf("⚠️  Rolling back %s/%s to version %s\n", kind, name, versionID)
	fmt.Printf("Are you sure? (yes/no): ")

	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" {
		fmt.Println("Rollback cancelled")
		return
	}

	if err := versionMgr.Rollback(kind, name, versionID); err != nil {
		fmt.Printf("❌ Rollback failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Rollback successful")
}

func handleDiff(versionMgr *yamlmgmt.VersionManager) {
	if len(os.Args) < 4 {
		fmt.Println("Error: kind and name required")
		fmt.Println("Usage: gotrs-config diff <kind> <name> [from] [to]")
		os.Exit(1)
	}

	kind := parseKind(os.Args[2])
	name := os.Args[3]

	// Get versions to compare
	versions, err := versionMgr.ListVersions(kind, name)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if len(versions) < 2 {
		fmt.Println("Not enough versions to compare")
		os.Exit(1)
	}

	var fromID, toID string

	if len(os.Args) >= 6 {
		fromID = os.Args[4]
		toID = os.Args[5]
	} else {
		// Compare last two versions
		fromID = versions[1].ID
		toID = versions[0].ID
	}

	changes, err := versionMgr.DiffVersions(kind, name, fromID, toID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔀 Configuration Diff\n")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("From: %s\n", fromID)
	fmt.Printf("To:   %s\n\n", toID)

	if len(changes) == 0 {
		fmt.Println("No changes")
	} else {
		for _, change := range changes {
			icon := "~"
			if change.Type == yamlmgmt.ChangeTypeAdd {
				icon = "+"
			} else if change.Type == yamlmgmt.ChangeTypeDelete {
				icon = "-"
			}

			fmt.Printf("%s %s: %s\n", icon, change.Path, change.Description)
			if change.OldValue != nil || change.NewValue != nil {
				if change.OldValue != nil {
					fmt.Printf("  Old: %v\n", change.OldValue)
				}
				if change.NewValue != nil {
					fmt.Printf("  New: %v\n", change.NewValue)
				}
			}
		}
	}
}

func handleApply(versionMgr *yamlmgmt.VersionManager, schemaRegistry *yamlmgmt.SchemaRegistry, linter *yamlmgmt.UniversalLinter) {
	if len(os.Args) < 3 {
		fmt.Println("Error: file path required")
		fmt.Println("Usage: gotrs-config apply <file>")
		os.Exit(1)
	}

	filename := os.Args[2]

	// Load file
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Parse YAML
	var doc yamlmgmt.YAMLDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	// Validate
	result, err := schemaRegistry.Validate(&doc)
	if err != nil || !result.Valid {
		fmt.Println("❌ Validation failed")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		for _, e := range result.Errors {
			fmt.Printf("  - %s: %s\n", e.Path, e.Message)
		}
		os.Exit(1)
	}

	// Lint
	issues, err := linter.Lint(&doc)
	if err != nil {
		fmt.Printf("Linting error: %v\n", err)
		os.Exit(1)
	}

	if hasErrors(issues) {
		fmt.Println("❌ Linting errors found")
		for _, issue := range issues {
			if issue.Severity == "error" {
				fmt.Printf("  - [%s] %s\n", issue.Rule, issue.Message)
			}
		}
		os.Exit(1)
	}

	// Create version
	kind := yamlmgmt.YAMLKind(doc.Kind)
	name := doc.Metadata.Name
	message := fmt.Sprintf("Applied from %s", filepath.Base(filename))

	version, err := versionMgr.CreateVersion(kind, name, &doc, message)
	if err != nil {
		fmt.Printf("❌ Failed to apply: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Configuration applied successfully\n")
	fmt.Printf("  Kind:    %s\n", kind)
	fmt.Printf("  Name:    %s\n", name)
	fmt.Printf("  Version: %s\n", version.Number)
	fmt.Printf("  Hash:    %s\n", version.Hash[:8])
}

func handleWatch(versionMgr *yamlmgmt.VersionManager, schemaRegistry *yamlmgmt.SchemaRegistry) {
	hotReload, err := yamlmgmt.NewHotReloadManager(versionMgr, schemaRegistry)
	if err != nil {
		fmt.Printf("Error creating hot reload manager: %v\n", err)
		os.Exit(1)
	}

	// Watch directories
	hotReload.WatchDirectory("./routes", yamlmgmt.KindRoute)
	hotReload.WatchDirectory("./config", yamlmgmt.KindConfig)
	hotReload.WatchDirectory("./dashboards", yamlmgmt.KindDashboard)

	fmt.Println("👁️  Watching for configuration changes...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Listen for events
	for event := range hotReload.Events() {
		timestamp := event.Timestamp.Format("15:04:05")

		icon := "📝"
		switch event.Type {
		case yamlmgmt.EventTypeCreated:
			icon = "✨"
		case yamlmgmt.EventTypeModified:
			icon = "📝"
		case yamlmgmt.EventTypeDeleted:
			icon = "🗑️"
		case yamlmgmt.EventTypeError:
			icon = "❌"
		case yamlmgmt.EventTypeReloaded:
			icon = "🔄"
		}

		fmt.Printf("[%s] %s %s/%s", timestamp, icon, event.Kind, event.Name)

		if event.Version != nil {
			fmt.Printf(" (v%s)", event.Version.Hash[:8])
		}

		if event.Error != "" {
			fmt.Printf(" - Error: %s", event.Error)
		}

		fmt.Println()
	}
}

func handleExport(versionMgr *yamlmgmt.VersionManager) {
	if len(os.Args) < 3 {
		fmt.Println("Error: kind required")
		fmt.Println("Usage: gotrs-config export <kind> [output-dir]")
		os.Exit(1)
	}

	kind := parseKind(os.Args[2])
	outputDir := "./export"
	if len(os.Args) > 3 {
		outputDir = os.Args[3]
	}

	// Create output directory
	os.MkdirAll(outputDir, 0755)

	// Get all documents
	documents, err := versionMgr.ListAll(kind)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📤 Exporting %d %s configurations to %s\n", len(documents), kind, outputDir)

	for _, doc := range documents {
		filename := filepath.Join(outputDir, fmt.Sprintf("%s.yaml", doc.Metadata.Name))

		data, err := yaml.Marshal(doc)
		if err != nil {
			fmt.Printf("❌ Failed to marshal %s: %v\n", doc.Metadata.Name, err)
			continue
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Printf("❌ Failed to write %s: %v\n", filename, err)
			continue
		}

		fmt.Printf("✅ Exported %s\n", doc.Metadata.Name)
	}

	fmt.Printf("\nExport complete: %d files written to %s\n", len(documents), outputDir)
}

func handleImport(versionMgr *yamlmgmt.VersionManager) {
	if len(os.Args) < 3 {
		fmt.Println("Error: directory required")
		fmt.Println("Usage: gotrs-config import <dir>")
		os.Exit(1)
	}

	importDir := os.Args[2]

	fmt.Printf("📥 Importing configurations from %s\n", importDir)

	imported := 0
	failed := 0

	filepath.Walk(importDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		// Load file
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("❌ Failed to read %s: %v\n", path, err)
			failed++
			return nil
		}

		// Parse YAML
		var doc yamlmgmt.YAMLDocument
		if err := yaml.Unmarshal(data, &doc); err != nil {
			fmt.Printf("❌ Failed to parse %s: %v\n", path, err)
			failed++
			return nil
		}

		// Create version
		kind := yamlmgmt.YAMLKind(doc.Kind)
		name := doc.Metadata.Name
		message := fmt.Sprintf("Imported from %s", filepath.Base(path))

		if _, err := versionMgr.CreateVersion(kind, name, &doc, message); err != nil {
			fmt.Printf("❌ Failed to import %s: %v\n", name, err)
			failed++
		} else {
			fmt.Printf("✅ Imported %s/%s\n", kind, name)
			imported++
		}

		return nil
	})

	fmt.Printf("\nImport complete: %d succeeded, %d failed\n", imported, failed)

	if failed > 0 {
		os.Exit(1)
	}
}

// Helper functions

func parseKind(s string) yamlmgmt.YAMLKind {
	switch strings.ToLower(s) {
	case "route", "routes":
		return yamlmgmt.KindRoute
	case "config", "configuration":
		return yamlmgmt.KindConfig
	case "dashboard", "dashboards":
		return yamlmgmt.KindDashboard
	case "compose", "docker-compose":
		return yamlmgmt.KindCompose
	default:
		return yamlmgmt.YAMLKind(s)
	}
}

func hasErrors(issues []yamlmgmt.LintIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}
