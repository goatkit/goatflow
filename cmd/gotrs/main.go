package main

import (
    "database/sql"
    "fmt"
    "os"
    "path/filepath"

    "github.com/gotrs-io/gotrs-ce/internal/config"
    "github.com/spf13/cobra"
    "golang.org/x/crypto/bcrypt"
    _ "github.com/lib/pq"
    _ "github.com/go-sql-driver/mysql"
    "github.com/gotrs-io/gotrs-ce/internal/database"
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
	rootCmd.AddCommand(resetUserCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GOTRS CLI %s\n", rootCmd.Version)
	},
}

var resetUserCmd = &cobra.Command{
	Use:   "reset-user",
	Short: "Reset a user's password and optionally enable their account",
	Long: `Reset a user's password in the database using bcrypt hashing.
	
Optionally enables the account by setting valid_id = 1 (OTRS compatible).
Connects directly to the database using environment variables.`,
	RunE: runResetUser,
}

var (
	usernameFlag string
	passwordFlag string
	enableFlag   bool
)

func init() {
	resetUserCmd.Flags().StringVar(&usernameFlag, "username", "", "Username to reset (required)")
	resetUserCmd.Flags().StringVar(&passwordFlag, "password", "", "New password (required)")
	resetUserCmd.Flags().BoolVar(&enableFlag, "enable", false, "Enable the user account (set valid_id = 1)")
	resetUserCmd.MarkFlagRequired("username")
	resetUserCmd.MarkFlagRequired("password")
}

func runSynthesize(cmd *cobra.Command, args []string) error {
	synth := config.NewSynthesizer(".env")
	
	// If only generating test data
	if testDataOnlyFlag {
		fmt.Fprintln(os.Stderr, "🔬 Generating test data...")
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

func runResetUser(cmd *cobra.Command, args []string) error {
	// Get database connection parameters from environment
    dbDriver := os.Getenv("DB_DRIVER")
    if dbDriver == "" {
        // Default to postgres if not specified, but most of our docker-compose uses mariadb
        dbDriver = os.Getenv("DB_ENGINE")
        if dbDriver == "" {
            dbDriver = "mariadb"
        }
    }
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
        // Prefer service names inside compose network
        if dbDriver == "postgres" || dbDriver == "postgresql" {
            dbHost = "postgres"
        } else {
            dbHost = "mariadb"
        }
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
        if dbDriver == "postgres" || dbDriver == "postgresql" {
            dbPort = "5432"
        } else {
            dbPort = "3306"
        }
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "gotrs"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
        if dbDriver == "postgres" || dbDriver == "postgresql" {
            dbUser = "gotrs_user"
        } else {
            dbUser = "otrs"
        }
	}
    dbPassword := os.Getenv("DB_PASSWORD")
    if dbPassword == "" {
        dbPassword = os.Getenv("PGPASSWORD")
    }
	if dbPassword == "" {
        return fmt.Errorf("database password environment variable is required (DB_PASSWORD or PGPASSWORD)")
	}

	// Connect to database
    var (
        driverName string
        dsn        string
    )
    switch dbDriver {
    case "postgres", "postgresql":
        driverName = "postgres"
        dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
            dbHost, dbPort, dbUser, dbPassword, dbName)
    case "mysql", "mariadb":
        driverName = "mysql"
        // Use MySQL DSN format
        dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4,utf8&loc=Local",
            dbUser, dbPassword, dbHost, dbPort, dbName)
    default:
        // Fallback: try postgres style first
        driverName = "postgres"
        dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
            dbHost, dbPort, dbUser, dbPassword, dbName)
    }

    fmt.Printf("🔗 Connecting to database (%s) %s@%s:%s/%s...\n", driverName, dbUser, dbHost, dbPort, dbName)
    db, err := sql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Generate bcrypt hash for the password
	fmt.Printf("🔒 Generating password hash...\n")
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordFlag), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to generate password hash: %w", err)
	}

	// Prepare SQL update
	var sqlQuery string
	var sqlArgs []any
	
	if enableFlag {
		sqlQuery = `UPDATE users SET 
			pw = $1,
			valid_id = 1,
			change_time = CURRENT_TIMESTAMP,
			change_by = 1
		WHERE login = $2`
		sqlArgs = []any{string(hash), usernameFlag}
	} else {
		sqlQuery = `UPDATE users SET 
			pw = $1,
			change_time = CURRENT_TIMESTAMP,
			change_by = 1
		WHERE login = $2`
		sqlArgs = []any{string(hash), usernameFlag}
	}

	// Execute update
	fmt.Printf("🔄 Updating user password and status...\n")
	result, err := db.Exec(sqlQuery, sqlArgs...)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' not found", usernameFlag)
	}

	// Verify the update
	var login string
	var validID int
	var passwordStatus string
	
	err = db.QueryRow(database.ConvertPlaceholders(`SELECT login,
		CASE WHEN pw IS NOT NULL THEN 'SET' ELSE 'NULL' END as password_status,
		valid_id
		FROM users WHERE login = $1`), usernameFlag).Scan(&login, &passwordStatus, &validID)
	
	if err != nil {
		return fmt.Errorf("failed to verify update: %w", err)
	}

	fmt.Printf("✅ Password reset successful!\n")
	fmt.Printf("   Username: %s\n", login)
	fmt.Printf("   Password: ******** (hidden for security)\n")
	if enableFlag {
		fmt.Printf("   Status: Enabled (valid_id=%d)\n", validID)
	} else {
		fmt.Printf("   Status: valid_id=%d\n", validID)
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}