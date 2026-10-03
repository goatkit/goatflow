package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/platform/marketplace"
	"github.com/goatkit/goatflow/internal/platform/plugin/packaging"
	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
	"github.com/goatkit/goatflow/internal/platform/version"
	"github.com/goatkit/goatflow/pkg/plugin"
)

func getPluginsDir() string {
	dir := os.Getenv("GOATFLOW_PLUGINS_DIR")
	if dir == "" {
		dir = "plugins"
	}
	return dir
}

// parsePluginRef splits "name@1.2.3" into name and version ("" when absent).
func parsePluginRef(ref string) (name, ver string) {
	name, ver, _ = strings.Cut(ref, "@")
	return name, strings.TrimPrefix(ver, "v")
}

func marketplaceInstall(ref string) {
	name, want := parsePluginRef(ref)
	client := marketplace.NewClient(getPluginsDir())

	fmt.Println("Fetching marketplace index...")
	entry, err := client.FindPlugin(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var target marketplace.VersionEntry
	if want != "" {
		target, err = entry.ResolveVersion(want)
	} else {
		target, err = entry.ResolveInstall()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found: %s v%s by %s (%s)\n", entry.Name, target.Version, entry.Author, entry.Licence)
	fmt.Printf("  Runtime: %s\n", entry.Runtime)
	if entry.Verified {
		fmt.Println("  Verified: ✓ (signed)")
	}
	if target.MinHostVersion != "" {
		fmt.Printf("  Requires GoatFlow >= %s\n", target.MinHostVersion)
	}
	if want == "" && marketplace.CompareVersions(target.Version, entry.LatestVersion) != 0 {
		fmt.Printf("  (v%s is the newest this GoatFlow %s can run; latest is v%s)\n", target.Version, version.Short(), entry.LatestVersion)
	}

	// Check dependencies.
	installed, _ := client.ListInstalled()
	missing, _ := marketplace.ResolveDependencies(name, &marketplace.Index{Plugins: []marketplace.PluginEntry{*entry}}, installed)
	if len(missing) > 0 {
		fmt.Printf("\n  Missing dependencies: %v\n", missing)
		fmt.Println("  Install dependencies first, then retry.")
		os.Exit(1)
	}

	// Check if already installed. An explicit name@version replaces any
	// installed version; a bare name leaves an existing install to 'gk update'.
	for _, inst := range installed {
		if inst.Name != name {
			continue
		}
		if marketplace.CompareVersions(inst.Version, target.Version) == 0 {
			fmt.Printf("\n  Already installed at v%s.\n", inst.Version)
			return
		}
		if want == "" {
			fmt.Printf("\n  Already installed at v%s (marketplace offers v%s)\n", inst.Version, target.Version)
			fmt.Println("  Use 'gk update' to upgrade, or 'gk install name@version' for a specific version.")
			return
		}
	}

	// Install: download, verify, extract.
	fmt.Printf("\nInstalling %s v%s...\n", entry.Name, target.Version)
	if _, err := client.Install(entry, target.Version); err != nil {
		fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Installed %s v%s to %s/%s/\n", entry.Name, target.Version, getPluginsDir(), entry.Name)
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
		if name != "" {
			explainNoUpdate(client, name)
		}
		return
	}

	for _, u := range updates {
		fmt.Printf("  Updating %s: %s → %s...\n", u.Name, u.CurrentVersion, u.LatestVersion)
		entry, err := client.FindPlugin(u.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    Error: %v\n", err)
			continue
		}
		if _, err := client.Update(entry, u.LatestVersion); err != nil {
			fmt.Fprintf(os.Stderr, "    Update failed: %v\n", err)
			continue
		}
		fmt.Printf("    Updated to v%s\n", u.LatestVersion)
	}
	fmt.Println("\nRestart GoatFlow to activate updates.")
}

// explainNoUpdate prints why an installed plugin has no update, e.g. when
// newer releases need a newer GoatFlow.
func explainNoUpdate(client *marketplace.Client, name string) {
	entry, err := client.FindPlugin(name)
	if err != nil {
		return
	}
	installed, _ := client.ListInstalled()
	for _, inst := range installed {
		if inst.Name != name {
			continue
		}
		if _, err := entry.ResolveUpdate(inst.Version); err != nil && !errors.Is(err, marketplace.ErrNoUpdate) {
			fmt.Printf("  Newer release not installable: %v\n", err)
		}
	}
}

func marketplaceInfo(name string) {
	client := marketplace.NewClient(getPluginsDir())
	entry, err := client.FindPlugin(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s — %s\n", entry.Name, entry.Description)
	fmt.Printf("  Author:  %s (%s)\n", entry.Author, entry.Licence)
	fmt.Printf("  Repo:    %s\n", entry.Repo)
	fmt.Printf("  Runtime: %s\n", entry.Runtime)
	installedVer := "not installed"
	installed, _ := client.ListInstalled()
	for _, inst := range installed {
		if inst.Name == entry.Name {
			installedVer = "v" + inst.Version
		}
	}
	fmt.Printf("  Installed: %s\n", installedVer)

	fmt.Printf("\n  Versions (this GoatFlow is %s):\n", version.Short())
	for _, v := range entry.AllVersions() {
		req := "any GoatFlow"
		if v.MinHostVersion != "" {
			req = "GoatFlow >= " + v.MinHostVersion
		}
		mark := "✓ compatible"
		if !v.Compatible() {
			mark = "✗ needs newer GoatFlow"
		}
		fmt.Printf("    v%-12s %-22s %s\n", v.Version, req, mark)
	}
	if target, err := entry.ResolveInstall(); err == nil {
		fmt.Printf("\n  'gk install %s' installs v%s\n", entry.Name, target.Version)
	} else {
		fmt.Printf("\n  Not installable: %v\n", err)
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

func marketplaceBuild(pluginDir string) {
	manifestPath := filepath.Join(pluginDir, "plugin.yaml")
	manifestData, err := os.ReadFile(manifestPath) // #nosec G304 G703 -- plugin dir given on the developer's own command line
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading plugin.yaml: %v\n", err)
		os.Exit(1)
	}

	var manifest plugin.PluginManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing plugin.yaml: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("dist", 0750); err != nil {
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
			data, err := os.ReadFile(args[i+1]) // #nosec G304 G703 -- --key-file path given on the developer's own command line
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
