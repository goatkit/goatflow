package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "gotrs",
	Short: "GOTRS CLI - Modern ticketing system management tool",
	Long: `GOTRS Command Line Interface
	
A modern replacement for OTRS, built with Go and HTMX.
This CLI provides utilities for managing your GOTRS installation.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

var synthesizeCmd = &cobra.Command{
	Use:     "synthesize",
	Aliases: []string{"synth", "generate-env"},
	Short:   "Synthesize a new .env file with secure random secrets",
	Long: `Synthesize generates a new .env configuration file with cryptographically 
secure random values for all secrets and sensible defaults for other settings.

This ensures your installation starts with strong, unique credentials instead 
of default values that could be security risks.`,
	RunE: runSynthesize,
}

var (
	rotateSecretsFlag bool
	outputPathFlag    string
	forceFlag        bool
	testDataOnlyFlag  bool
)

func init() {
	synthesizeCmd.Flags().BoolVar(&rotateSecretsFlag, "rotate-secrets", false, "Rotate only secret values, keeping other settings")
	synthesizeCmd.Flags().StringVar(&outputPathFlag, "output", ".env", "Output path for the generated .env file")
	synthesizeCmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite existing .env without prompting")
	synthesizeCmd.Flags().BoolVar(&testDataOnlyFlag, "test-data-only", false, "Generate only test data SQL and CSV files")
	
	rootCmd.AddCommand(synthesizeCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GOTRS CLI %s\n", rootCmd.Version)
	},
}

func runSynthesize(cmd *cobra.Command, args []string) error {
	synth := config.NewSynthesizer(".env")
	
	// If only generating test data
	if testDataOnlyFlag {
		fmt.Println("🔬 Generating test data...")
		return synth.SynthesizeTestData()
	}
	
	outputPath := outputPathFlag
	if !filepath.IsAbs(outputPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		outputPath = filepath.Join(cwd, outputPath)
	}
	
	if _, err := os.Stat(outputPath); err == nil && !forceFlag && !rotateSecretsFlag {
		fmt.Printf("⚠️  File %s already exists. Use --force to overwrite or --rotate-secrets to update secrets only.\n", outputPathFlag)
		return nil
	}
	
	fmt.Println("🔬 Synthesizing secure configuration...")
	
	synth = config.NewSynthesizer(outputPath)
	
	if err := synth.SynthesizeEnv(rotateSecretsFlag); err != nil {
		return fmt.Errorf("failed to synthesize environment: %w", err)
	}
	
	generatedCount := synth.GetGeneratedCount()
	
	if rotateSecretsFlag {
		fmt.Printf("🔄 Rotated %d secret values\n", generatedCount)
		fmt.Printf("📝 Backup saved to %s.backup.*\n", outputPathFlag)
	} else {
		fmt.Printf("✅ Generated %d secure secrets\n", generatedCount)
		fmt.Printf("📝 Created %s with secure configuration\n", outputPathFlag)
		
		// Also generate test data when creating new .env
		fmt.Println("\n🔬 Generating test data...")
		if err := synth.SynthesizeTestData(); err != nil {
			return fmt.Errorf("failed to generate test data: %w", err)
		}
	}
	
	if !rotateSecretsFlag {
		// Check if we're running in a container (no .git directory accessible)
		if _, err := os.Stat(".git"); os.IsNotExist(err) {
			fmt.Println("\n📝 Note: Running in container - git hooks not installed")
			fmt.Println("   Run 'make scan-secrets-precommit' from host to install hooks")
		} else {
			fmt.Println("\n🛡️  Installing git hooks for secret scanning...")
			if err := installGitHooks(); err != nil {
				fmt.Printf("⚠️  Warning: Failed to install git hooks: %v\n", err)
				fmt.Println("   Run 'make scan-secrets-precommit' to install manually")
			} else {
				fmt.Println("✅ Git hooks installed successfully")
			}
		}
	}
	
	fmt.Println("\n🚀 Ready! Run 'make up' to start services")
	
	return nil
}

func installGitHooks() error {
	gitDir := ".git"
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not in a git repository")
	}
	
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}
	
	preCommitPath := filepath.Join(hooksDir, "pre-commit")
	
	preCommitContent := `#!/bin/bash
# GOTRS Pre-commit hook - Scan for secrets before committing

echo "🔍 Scanning for secrets..."

# Check if gitleaks is available
if command -v gitleaks &> /dev/null; then
    gitleaks detect --source . --verbose
    if [ $? -ne 0 ]; then
        echo "❌ Secrets detected! Commit aborted."
        echo "   Review the findings above and remove any secrets."
        echo "   Use 'gitleaks detect --source . --verbose' to re-scan."
        exit 1
    fi
else
    # Try with Docker/Podman
    CONTAINER_CMD=$(command -v podman 2> /dev/null || command -v docker 2> /dev/null)
    if [ -n "$CONTAINER_CMD" ]; then
        $CONTAINER_CMD run --rm -v "$(pwd):/workspace" -w /workspace \
            zricethezav/gitleaks:latest detect --source . --verbose
        if [ $? -ne 0 ]; then
            echo "❌ Secrets detected! Commit aborted."
            exit 1
        fi
    else
        echo "⚠️  Warning: gitleaks not found. Install it or use Docker/Podman."
        echo "   Skipping secret scan (not recommended)."
    fi
fi

echo "✅ No secrets detected"
exit 0
`
	
	if err := os.WriteFile(preCommitPath, []byte(preCommitContent), 0755); err != nil {
		return fmt.Errorf("failed to write pre-commit hook: %w", err)
	}
	
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}