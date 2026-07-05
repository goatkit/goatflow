package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/platform/marketplace"
	"github.com/goatkit/goatflow/internal/platform/plugin/packaging"
	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
	"github.com/goatkit/goatflow/pkg/plugin"
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
			if inst.Version == entry.LatestVersion {
				fmt.Printf("\n  Already installed at v%s — up to date.\n", inst.Version)
				return
			}
			fmt.Printf("\n  Already installed at v%s (marketplace has v%s)\n", inst.Version, entry.LatestVersion)
			fmt.Println("  Use 'gk update' to upgrade.")
			return
		}
	}

	// Install: download, verify, extract.
	fmt.Printf("\nInstalling %s v%s...\n", entry.Name, entry.LatestVersion)
	if err := client.Install(entry); err != nil {
		fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Installed %s v%s to %s/%s/\n", entry.Name, entry.LatestVersion, getPluginsDir(), entry.Name)
	fmt.Println("Restart GoatFlow to activate.")
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
		fmt.Printf("  Updating %s: %s → %s...\n", u.Name, u.CurrentVersion, u.LatestVersion)
		entry, err := client.FindPlugin(u.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    Error: %v\n", err)
			continue
		}
		if err := client.Update(entry); err != nil {
			fmt.Fprintf(os.Stderr, "    Update failed: %v\n", err)
			continue
		}
		fmt.Printf("    Updated to v%s\n", u.LatestVersion)
	}
	fmt.Println("\nRestart GoatFlow to activate updates.")
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

func marketplaceBuild(pluginDir string) {
	manifestPath := filepath.Join(pluginDir, "plugin.yaml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading plugin.yaml: %v\n", err)
		os.Exit(1)
	}

	var manifest plugin.PluginManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing plugin.yaml: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("dist", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dist/: %v\n", err)
		os.Exit(1)
	}

	outputPath := filepath.Join("dist", manifest.Name+"-"+manifest.Version+".zip")
	if err := packaging.PackagePlugin(pluginDir, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error building plugin: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Built %s\n", outputPath)
}

func marketplaceSign(filePath string, args []string) {
	keyHex := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--key" && i+1 < len(args) {
			keyHex = args[i+1]
			i++
		} else if args[i] == "--key-file" && i+1 < len(args) {
			data, err := os.ReadFile(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading key file: %v\n", err)
				os.Exit(1)
			}
			keyHex = strings.TrimSpace(string(data))
			i++
		}
	}

	if keyHex == "" {
		keyHex = os.Getenv("GOATFLOW_SIGNING_KEY")
	}
	if keyHex == "" {
		fmt.Fprintln(os.Stderr, "Error: no signing key provided. Use --key <hex>, --key-file <path>, or set GOATFLOW_SIGNING_KEY.")
		os.Exit(1)
	}

	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding key: %v\n", err)
		os.Exit(1)
	}

	privateKey := ed25519.PrivateKey(keyBytes)
	sigPath := filePath + ".sig"
	if err := signing.SignBinary(filePath, sigPath, privateKey); err != nil {
		fmt.Fprintf(os.Stderr, "Error signing: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Signed %s → %s\n", filePath, sigPath)
}

func marketplaceGenerateKeys() {
	pub, priv, err := signing.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key pair: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Private key: %s\n", hex.EncodeToString(priv))
	fmt.Printf("Public key:  %s\n", hex.EncodeToString(pub))
	fmt.Println("\nStore the private key securely. Share the public key with users for verification.")
	fmt.Println("Configure the public key in GoatFlow via GOATFLOW_TRUSTED_KEYS=<hex>")
}
