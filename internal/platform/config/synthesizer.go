package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SecretType string

const (
	SecretTypeHex      SecretType = "hex"
	SecretTypeAlphaNum SecretType = "alphanum"
	SecretTypeMixed    SecretType = "mixed"
	SecretTypeAPIKey   SecretType = "apikey"
	SecretTypePassword SecretType = "password"
)

type EnvVariable struct {
	Key         string
	Value       string
	Type        string
	Generated   bool
	Description string
}

type Synthesizer struct {
	// templatePath is reserved for future template-driven synthesis
	//nolint:unused
	templatePath string
	outputPath   string
	variables    []EnvVariable
}

func NewSynthesizer(outputPath string) *Synthesizer {
	return &Synthesizer{
		outputPath: outputPath,
		variables:  make([]EnvVariable, 0),
	}
}

func (s *Synthesizer) GenerateSecret(secretType SecretType, length int, keyType string, envPrefix string) (string, error) {
	switch secretType {
	case SecretTypeHex:
		return s.generateHex(length)
	case SecretTypeAlphaNum:
		return s.generateAlphaNum(length)
	case SecretTypeMixed:
		return s.generateMixed(length)
	case SecretTypeAPIKey:
		return s.generateAPIKey(keyType, envPrefix)
	case SecretTypePassword:
		return s.generatePassword(length)
	default:
		return "", fmt.Errorf("unknown secret type: %s", secretType)
	}
}

func (s *Synthesizer) generateHex(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Synthesizer) generateAlphaNum(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

func (s *Synthesizer) generateMixed(length int) (string, error) {
	// Use base64 for mixed secrets too - consistent and safe
	return s.generatePassword(length)
}

func (s *Synthesizer) generatePassword(length int) (string, error) {
	if length < 12 {
		length = 12
	}

	// Generate random bytes and base64 encode them
	// Base64 uses only: A-Z, a-z, 0-9, +, /, =
	// All safe for shells, URLs (when properly encoded), and SQL
	byteLength := (length * 3) / 4 // Account for base64 expansion
	if byteLength < 9 {
		byteLength = 9
	}

	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Use URL-safe base64 encoding (replaces + with -, / with _)
	// This gives us only: A-Z, a-z, 0-9, -, _
	result := base64.RawURLEncoding.EncodeToString(bytes)

	// Trim to requested length
	if len(result) > length {
		result = result[:length]
	}

	return result, nil
}

func (s *Synthesizer) generateAPIKey(keyType string, envPrefix string) (string, error) {
	random, err := s.generateAlphaNum(32)
	if err != nil {
		return "", err
	}
	// Add environment prefix if not production
	if envPrefix != "" {
		return fmt.Sprintf("%sgtr-%s-%s", envPrefix, keyType, strings.ToLower(random)), nil
	}
	return fmt.Sprintf("gtr-%s-%s", keyType, strings.ToLower(random)), nil
}

// randomInt returns a secure random integer in [0,max). Currently unused.
//
//nolint:unused
func (s *Synthesizer) randomInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

func (s *Synthesizer) SynthesizeEnv(rotateOnly bool) error {
	existingVars := make(map[string]string)

	if rotateOnly {
		existingVars = s.loadExistingEnv()
	}

	s.generateVariables(existingVars, rotateOnly)

	if err := s.writeEnvFile(); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	return nil
}

// SynthesizeTestData generates test data SQL and CSV files.
func (s *Synthesizer) SynthesizeTestData() error {
	generator := NewTestDataGenerator(s)

	if err := generator.Generate(); err != nil {
		return fmt.Errorf("failed to generate test data: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✅ Generated test data SQL: migrations/postgres/000004_generated_test_data.up.sql\n")
	fmt.Fprintf(os.Stderr, "📝 Credentials CSV written to stdout (redirect to save: make synthesize > credentials.csv)\n")

	return nil
}

func (s *Synthesizer) loadExistingEnv() map[string]string {
	vars := make(map[string]string)

	data, err := os.ReadFile(s.outputPath)
	if err != nil {
		return vars
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "\"'")
			vars[key] = value
		}
	}

	return vars
}

func (s *Synthesizer) generateVariables(existing map[string]string, rotateOnly bool) {
	// Determine environment for prefixing
	appEnv := s.getOrDefault(existing, "APP_ENV", "development")
	prefix := ""
	switch appEnv {
	case "development", "dev":
		prefix = "dev-"
	case "test", "testing":
		prefix = "test-"
	case "staging", "stage":
		prefix = "stage-"
		// production gets no prefix for cleaner keys
	}

	s.variables = []EnvVariable{
		{Key: "# GoatFlow Environment Configuration", Value: "", Type: "comment"},
		{Key: "# Generated by 'goatflow synthesize'", Value: "", Type: "comment"},
		{Key: "# Generated at: " + time.Now().Format(time.RFC3339), Value: "", Type: "comment"},
		{Key: fmt.Sprintf("# Environment: %s", appEnv), Value: "", Type: "comment"},
		{Key: "", Value: "", Type: "blank"},

		{Key: "# Application", Value: "", Type: "section"},
		{Key: "APP_ENV", Value: appEnv, Type: "static"},
		{Key: "APP_PORT", Value: s.getOrDefault(existing, "APP_PORT", "8080"), Type: "static"},
		{Key: "APP_URL", Value: s.getOrDefault(existing, "APP_URL", "http://localhost:8080"), Type: "static"},
		{Key: "", Value: "", Type: "blank"},

		{Key: "# Security Tokens (Auto-generated)", Value: "", Type: "section"},
	}

	jwtSecret, _ := s.GenerateSecret(SecretTypeHex, 64, "", "")
	if prefix != "" {
		jwtSecret = prefix + "jwt-" + jwtSecret
	}
	s.variables = append(s.variables, EnvVariable{
		Key:       "JWT_SECRET",
		Value:     s.getOrGenerate(existing, "JWT_SECRET", jwtSecret, rotateOnly),
		Type:      "secret",
		Generated: true,
	})

	sessionSecret, _ := s.GenerateSecret(SecretTypeHex, 48, "", "")
	if prefix != "" {
		sessionSecret = prefix + "session-" + sessionSecret
	}
	s.variables = append(s.variables, EnvVariable{
		Key:       "SESSION_SECRET",
		Value:     s.getOrGenerate(existing, "SESSION_SECRET", sessionSecret, rotateOnly),
		Type:      "secret",
		Generated: true,
	})

	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# Database", Value: "", Type: "section",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "DB_HOST", Value: s.getOrDefault(existing, "DB_HOST", "localhost"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "DB_PORT", Value: s.getOrDefault(existing, "DB_PORT", "5432"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "DB_NAME", Value: s.getOrDefault(existing, "DB_NAME", "goatflow"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "DB_USER", Value: s.getOrDefault(existing, "DB_USER", "goatflow_user"), Type: "static",
	})

	dbPassword, _ := s.GenerateSecret(SecretTypePassword, 24, "", "")
	s.variables = append(s.variables, EnvVariable{
		Key:       "DB_PASSWORD",
		Value:     s.getOrGenerate(existing, "DB_PASSWORD", dbPassword, rotateOnly),
		Type:      "secret",
		Generated: true,
	})

	s.variables = append(s.variables, EnvVariable{
		Key: "DB_SSL_MODE", Value: s.getOrDefault(existing, "DB_SSL_MODE", "disable"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# Valkey Cache", Value: "", Type: "section",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "VALKEY_HOST", Value: s.getOrDefault(existing, "VALKEY_HOST", "localhost"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "VALKEY_PORT", Value: s.getOrDefault(existing, "VALKEY_PORT", "6388"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "VALKEY_PASSWORD", Value: s.getOrDefault(existing, "VALKEY_PASSWORD", ""), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "VALKEY_DB", Value: s.getOrDefault(existing, "VALKEY_DB", "0"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# Email Configuration", Value: "", Type: "section",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "# MailHog (development) doesn't require authentication - leave USER/PASSWORD empty", Value: "", Type: "comment",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "# For production, update SMTP_HOST and add real credentials", Value: "", Type: "comment",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "SMTP_HOST", Value: s.getOrDefault(existing, "SMTP_HOST", "mailhog"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "SMTP_PORT", Value: s.getOrDefault(existing, "SMTP_PORT", "1025"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "SMTP_USER", Value: s.getOrDefault(existing, "SMTP_USER", ""), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "SMTP_PASSWORD", Value: s.getOrDefault(existing, "SMTP_PASSWORD", ""), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "SMTP_FROM_EMAIL", Value: s.getOrDefault(existing, "SMTP_FROM_EMAIL", "noreply@goatflow.local"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "SMTP_FROM_NAME", Value: s.getOrDefault(existing, "SMTP_FROM_NAME", "GoatFlow Support"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# Search Engine (Zinc)", Value: "", Type: "section",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "ZINC_USER", Value: s.getOrDefault(existing, "ZINC_USER", "zinc_admin"), Type: "static",
	})

	zincPassword, _ := s.GenerateSecret(SecretTypePassword, 20, "", "")
	s.variables = append(s.variables, EnvVariable{
		Key:       "ZINC_PASSWORD",
		Value:     s.getOrGenerate(existing, "ZINC_PASSWORD", zincPassword, rotateOnly),
		Type:      "secret",
		Generated: true,
	})
	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# LDAP Integration", Value: "", Type: "section",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "LDAP_HOST", Value: s.getOrDefault(existing, "LDAP_HOST", "openldap"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "LDAP_PORT", Value: s.getOrDefault(existing, "LDAP_PORT", "389"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "LDAP_BASE_DN", Value: s.getOrDefault(existing, "LDAP_BASE_DN", "dc=goatflow,dc=local"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "LDAP_BIND_DN", Value: s.getOrDefault(existing, "LDAP_BIND_DN", "cn=readonly,dc=goatflow,dc=local"), Type: "static",
	})

	ldapPassword, _ := s.GenerateSecret(SecretTypePassword, 16, "", "")
	s.variables = append(s.variables, EnvVariable{
		Key:       "LDAP_BIND_PASSWORD",
		Value:     s.getOrGenerate(existing, "LDAP_BIND_PASSWORD", ldapPassword, rotateOnly),
		Type:      "secret",
		Generated: true,
	})
	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# Internal API Keys", Value: "", Type: "section",
	})

	internalAPI, _ := s.GenerateSecret(SecretTypeAPIKey, 0, "internal", prefix)
	s.variables = append(s.variables, EnvVariable{
		Key:       "API_KEY_INTERNAL",
		Value:     s.getOrGenerate(existing, "API_KEY_INTERNAL", internalAPI, rotateOnly),
		Type:      "secret",
		Generated: true,
	})

	webhookSecret, _ := s.GenerateSecret(SecretTypeHex, 32, "", "")
	if prefix != "" {
		webhookSecret = prefix + "webhook-" + webhookSecret
	}
	s.variables = append(s.variables, EnvVariable{
		Key:       "WEBHOOK_SECRET",
		Value:     s.getOrGenerate(existing, "WEBHOOK_SECRET", webhookSecret, rotateOnly),
		Type:      "secret",
		Generated: true,
	})
	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# Feature Flags", Value: "", Type: "section",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "FEATURE_AI_SUGGESTIONS", Value: s.getOrDefault(existing, "FEATURE_AI_SUGGESTIONS", "false"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "FEATURE_WEBHOOKS", Value: s.getOrDefault(existing, "FEATURE_WEBHOOKS", "true"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{Key: "", Value: "", Type: "blank"})

	s.variables = append(s.variables, EnvVariable{
		Key: "# Logging", Value: "", Type: "section",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "LOG_LEVEL", Value: s.getOrDefault(existing, "LOG_LEVEL", "info"), Type: "static",
	})
	s.variables = append(s.variables, EnvVariable{
		Key: "LOG_FORMAT", Value: s.getOrDefault(existing, "LOG_FORMAT", "json"), Type: "static",
	})
}

func (s *Synthesizer) getOrDefault(existing map[string]string, key, defaultValue string) string {
	if val, ok := existing[key]; ok {
		return val
	}
	return defaultValue
}

func (s *Synthesizer) getOrGenerate(existing map[string]string, key, newValue string, rotateOnly bool) string {
	if rotateOnly {
		return newValue
	}
	if val, ok := existing[key]; ok && val != "" {
		return val
	}
	return newValue
}

func (s *Synthesizer) writeEnvFile() error {
	if _, err := os.Stat(s.outputPath); err == nil {
		backupPath := fmt.Sprintf("%s.backup.%s", s.outputPath, time.Now().Format("20060102_150405"))
		if err := s.copyFile(s.outputPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing .env: %w", err)
		}
	}

	file, err := os.Create(s.outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	for _, v := range s.variables {
		switch v.Type {
		case "comment":
			fmt.Fprintln(file, v.Key)
		case "section":
			fmt.Fprintln(file, v.Key)
		case "blank":
			fmt.Fprintln(file)
		default:
			fmt.Fprintf(file, "%s=%s\n", v.Key, v.Value)
		}
	}

	return nil
}

func (s *Synthesizer) copyFile(src, dst string) error {
	sourceFile, err := os.ReadFile(src) //nolint:gosec // G304 false positive - config copy
	if err != nil {
		return err
	}

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	return os.WriteFile(dst, sourceFile, 0644)
}

func (s *Synthesizer) GetGeneratedCount() int {
	count := 0
	for _, v := range s.variables {
		if v.Generated {
			count++
		}
	}
	return count
}
