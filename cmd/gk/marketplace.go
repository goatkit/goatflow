package main

import (
	"fmt"
	"os"

	"github.com/goatkit/goatflow/internal/platform/marketplace"
)

func getPluginsDir() string {
	dir := os.Getenv("GOATFLOW_PLUGINS_DIR")
	if dir == "" {
		dir = "plugins"
	}
	return dir
}

func marketplaceInstall(name string) {
	client := marketplace.NewClient(getPluginsDir())

	fmt.Println("Fetching marketplace index...")
	entry, err := client.FindPlugin(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found: %s v%s by %s (%s)\n", entry.Name, entry.LatestVersion, entry.Author, entry.Licence)
	fmt.Printf("  Runtime: %s\n", entry.Runtime)
	if entry.Verified {
		fmt.Println("  Verified: ✓ (signed)")
	}
	if entry.MinHostVersion != "" {
		fmt.Printf("  Requires GoatFlow >= %s\n", entry.MinHostVersion)
	}

	// Check dependencies.
	installed, _ := client.ListInstalled()
	missing, _ := marketplace.ResolveDependencies(name, &marketplace.Index{Plugins: []marketplace.PluginEntry{*entry}}, installed)
	if len(missing) > 0 {
		fmt.Printf("\n  Missing dependencies: %v\n", missing)
		fmt.Println("  Install dependencies first, then retry.")
		os.Exit(1)
	}

	// Check if already installed.
	for _, inst := range installed {
		if inst.Name == name {
			fmt.Printf("\n  Already installed: v%s (marketplace has v%s)\n", inst.Version, entry.LatestVersion)
			if inst.Version == entry.LatestVersion {
				fmt.Println("  Up to date.")
				return
			}
			fmt.Println("  Use 'gk update' to upgrade.")
			return
		}
	}

	downloadURL := marketplace.DownloadURL(entry.Repo, entry.LatestVersion, entry.Name)
	fmt.Printf("\nDownload: %s\n", downloadURL)
	fmt.Println("  To install manually:")
	fmt.Printf("    curl -L -o %s.zip %s\n", name, downloadURL)
	fmt.Printf("    unzip %s.zip -d %s/%s/\n", name, getPluginsDir(), name)
	fmt.Println("    Restart GoatFlow to activate.")
}

func marketplaceUpdate(name string) {
	client := marketplace.NewClient(getPluginsDir())

	fmt.Println("Checking for updates...")
	updates, err := client.CheckUpdates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if name != "" {
		// Filter to specific plugin.
		var filtered []marketplace.UpdateAvailable
		for _, u := range updates {
			if u.Name == name {
				filtered = append(filtered, u)
			}
		}
		updates = filtered
	}

	if len(updates) == 0 {
		fmt.Println("All plugins are up to date.")
		return
	}

	for _, u := range updates {
		fmt.Printf("  %s: %s → %s (update available)\n", u.Name, u.CurrentVersion, u.LatestVersion)
		downloadURL := marketplace.DownloadURL(u.Repo, u.LatestVersion, u.Name)
		fmt.Printf("    Download: %s\n", downloadURL)
	}
}

func marketplaceSearch(query string) {
	client := marketplace.NewClient(getPluginsDir())

	results, err := client.Search(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Printf("No plugins found for %q\n", query)
		return
	}

	fmt.Printf("Results for %q:\n\n", query)
	fmt.Printf("  %-15s %-35s %-10s %-12s %s\n", "NAME", "DESCRIPTION", "VERSION", "AUTHOR", "LICENCE")
	for _, p := range results {
		desc := p.Description
		if len(desc) > 33 {
			desc = desc[:30] + "..."
		}
		fmt.Printf("  %-15s %-35s %-10s %-12s %s\n", p.Name, desc, p.LatestVersion, p.Author, p.Licence)
	}
}
