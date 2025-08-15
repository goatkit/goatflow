package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigStructs(t *testing.T) {
	t.Run("Config struct has all expected fields", func(t *testing.T) {
		cfg := &Config{}
		assert.NotNil(t, cfg)
		
		// Verify main config sections exist
		assert.NotNil(t, &cfg.App)
		assert.NotNil(t, &cfg.Server)
		assert.NotNil(t, &cfg.Database)
		assert.NotNil(t, &cfg.Valkey)
		assert.NotNil(t, &cfg.Auth)
		assert.NotNil(t, &cfg.Email)
		assert.NotNil(t, &cfg.Storage)
		assert.NotNil(t, &cfg.Ticket)
		assert.NotNil(t, &cfg.Logging)
		assert.NotNil(t, &cfg.Metrics)
		assert.NotNil(t, &cfg.RateLimiting)
		assert.NotNil(t, &cfg.Features)
		assert.NotNil(t, &cfg.Maintenance)
		assert.NotNil(t, &cfg.Integrations)
	})
}

func TestDatabaseConfig(t *testing.T) {
	t.Run("GetDSN returns correct connection string", func(t *testing.T) {
		dbConfig := &DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "testuser",
			Password: "testpass",
			Name:     "testdb",
			SSLMode:  "disable",
		}

		expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
		assert.Equal(t, expected, dbConfig.GetDSN())
	})

	t.Run("GetDSN handles different configurations", func(t *testing.T) {
		testCases := []struct {
			name     string
			config   DatabaseConfig
			expected string
		}{
			{
				name: "with SSL",
				config: DatabaseConfig{
					Host:     "db.example.com",
					Port:     5432,
					User:     "admin",
					Password: "secret",
					Name:     "production",
					SSLMode:  "require",
				},
				expected: "host=db.example.com port=5432 user=admin password=secret dbname=production sslmode=require",
			},
			{
				name: "custom port",
				config: DatabaseConfig{
					Host:     "localhost",
					Port:     5433,
					User:     "user",
					Password: "pass",
					Name:     "mydb",
					SSLMode:  "disable",
				},
				expected: "host=localhost port=5433 user=user password=pass dbname=mydb sslmode=disable",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, tc.config.GetDSN())
			})
		}
	})
}

func TestValkeyConfig(t *testing.T) {
	t.Run("GetValkeyAddr returns correct address", func(t *testing.T) {
		valkeyConfig := &ValkeyConfig{
			Host: "localhost",
			Port: 6379,
		}

		assert.Equal(t, "localhost:6379", valkeyConfig.GetValkeyAddr())
	})

	t.Run("GetValkeyAddr handles different configurations", func(t *testing.T) {
		testCases := []struct {
			name     string
			config   ValkeyConfig
			expected string
		}{
			{
				name:     "default Redis port",
				config:   ValkeyConfig{Host: "127.0.0.1", Port: 6379},
				expected: "127.0.0.1:6379",
			},
			{
				name:     "custom port",
				config:   ValkeyConfig{Host: "valkey.example.com", Port: 6380},
				expected: "valkey.example.com:6380",
			},
			{
				name:     "IPv6 address",
				config:   ValkeyConfig{Host: "::1", Port: 6379},
				expected: "::1:6379",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, tc.config.GetValkeyAddr())
			})
		}
	})
}

func TestServerConfig(t *testing.T) {
	t.Run("GetServerAddr returns correct address", func(t *testing.T) {
		serverConfig := &ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		}

		assert.Equal(t, "0.0.0.0:8080", serverConfig.GetServerAddr())
	})

	t.Run("GetServerAddr handles different configurations", func(t *testing.T) {
		testCases := []struct {
			name     string
			config   ServerConfig
			expected string
		}{
			{
				name:     "localhost",
				config:   ServerConfig{Host: "localhost", Port: 3000},
				expected: "localhost:3000",
			},
			{
				name:     "all interfaces",
				config:   ServerConfig{Host: "0.0.0.0", Port: 8080},
				expected: "0.0.0.0:8080",
			},
			{
				name:     "specific IP",
				config:   ServerConfig{Host: "192.168.1.100", Port: 8090},
				expected: "192.168.1.100:8090",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, tc.config.GetServerAddr())
			})
		}
	})

	t.Run("CORS configuration", func(t *testing.T) {
		corsConfig := CORSConfig{
			Enabled: true,
			Origins: []string{"http://localhost:3000", "https://example.com"},
			Methods: []string{"GET", "POST", "PUT", "DELETE"},
			Headers: []string{"Content-Type", "Authorization"},
		}

		assert.True(t, corsConfig.Enabled)
		assert.Len(t, corsConfig.Origins, 2)
		assert.Contains(t, corsConfig.Origins, "http://localhost:3000")
		assert.Contains(t, corsConfig.Methods, "POST")
		assert.Contains(t, corsConfig.Headers, "Authorization")
	})
}

func TestAppConfig(t *testing.T) {
	t.Run("IsProduction returns true for production env", func(t *testing.T) {
		appConfig := &AppConfig{
			Env: "production",
		}

		assert.True(t, appConfig.IsProduction())
		assert.False(t, appConfig.IsDevelopment())
	})

	t.Run("IsDevelopment returns true for development env", func(t *testing.T) {
		appConfig := &AppConfig{
			Env: "development",
		}

		assert.True(t, appConfig.IsDevelopment())
		assert.False(t, appConfig.IsProduction())
	})

	t.Run("Environment checks handle different values", func(t *testing.T) {
		testCases := []struct {
			env           string
			isProduction  bool
			isDevelopment bool
		}{
			{"production", true, false},
			{"development", false, true},
			{"staging", false, false},
			{"test", false, false},
			{"", false, false},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("env=%s", tc.env), func(t *testing.T) {
				appConfig := &AppConfig{Env: tc.env}
				assert.Equal(t, tc.isProduction, appConfig.IsProduction())
				assert.Equal(t, tc.isDevelopment, appConfig.IsDevelopment())
			})
		}
	})
}

func TestAuthConfig(t *testing.T) {
	t.Run("JWT configuration", func(t *testing.T) {
		authConfig := AuthConfig{}
		authConfig.JWT.Secret = "test-secret"
		authConfig.JWT.Issuer = "gotrs"
		authConfig.JWT.Audience = "gotrs-api"
		authConfig.JWT.AccessTokenTTL = 15 * time.Minute
		authConfig.JWT.RefreshTokenTTL = 7 * 24 * time.Hour

		assert.Equal(t, "test-secret", authConfig.JWT.Secret)
		assert.Equal(t, "gotrs", authConfig.JWT.Issuer)
		assert.Equal(t, 15*time.Minute, authConfig.JWT.AccessTokenTTL)
		assert.Equal(t, 7*24*time.Hour, authConfig.JWT.RefreshTokenTTL)
	})

	t.Run("Session configuration", func(t *testing.T) {
		authConfig := AuthConfig{}
		authConfig.Session.CookieName = "gotrs_session"
		authConfig.Session.Secure = true
		authConfig.Session.HTTPOnly = true
		authConfig.Session.SameSite = "strict"
		authConfig.Session.MaxAge = 86400

		assert.Equal(t, "gotrs_session", authConfig.Session.CookieName)
		assert.True(t, authConfig.Session.Secure)
		assert.True(t, authConfig.Session.HTTPOnly)
		assert.Equal(t, "strict", authConfig.Session.SameSite)
		assert.Equal(t, 86400, authConfig.Session.MaxAge)
	})

	t.Run("Password policy configuration", func(t *testing.T) {
		authConfig := AuthConfig{}
		authConfig.Password.MinLength = 8
		authConfig.Password.RequireUppercase = true
		authConfig.Password.RequireLowercase = true
		authConfig.Password.RequireNumber = true
		authConfig.Password.RequireSpecial = false
		authConfig.Password.BcryptCost = 12

		assert.Equal(t, 8, authConfig.Password.MinLength)
		assert.True(t, authConfig.Password.RequireUppercase)
		assert.True(t, authConfig.Password.RequireLowercase)
		assert.True(t, authConfig.Password.RequireNumber)
		assert.False(t, authConfig.Password.RequireSpecial)
		assert.Equal(t, 12, authConfig.Password.BcryptCost)
	})
}

func TestEmailConfig(t *testing.T) {
	t.Run("SMTP configuration", func(t *testing.T) {
		emailConfig := EmailConfig{
			Enabled:  true,
			From:     "noreply@gotrs.io",
			FromName: "GOTRS System",
		}
		emailConfig.SMTP.Host = "smtp.example.com"
		emailConfig.SMTP.Port = 587
		emailConfig.SMTP.User = "smtp-user"
		emailConfig.SMTP.Password = "smtp-pass"
		emailConfig.SMTP.TLS = true

		assert.True(t, emailConfig.Enabled)
		assert.Equal(t, "noreply@gotrs.io", emailConfig.From)
		assert.Equal(t, "GOTRS System", emailConfig.FromName)
		assert.Equal(t, "smtp.example.com", emailConfig.SMTP.Host)
		assert.Equal(t, 587, emailConfig.SMTP.Port)
		assert.True(t, emailConfig.SMTP.TLS)
	})

	t.Run("Email queue configuration", func(t *testing.T) {
		emailConfig := EmailConfig{}
		emailConfig.Queue.Workers = 5
		emailConfig.Queue.BufferSize = 100
		emailConfig.Queue.RetryAttempts = 3
		emailConfig.Queue.RetryDelay = 30 * time.Second

		assert.Equal(t, 5, emailConfig.Queue.Workers)
		assert.Equal(t, 100, emailConfig.Queue.BufferSize)
		assert.Equal(t, 3, emailConfig.Queue.RetryAttempts)
		assert.Equal(t, 30*time.Second, emailConfig.Queue.RetryDelay)
	})
}

func TestStorageConfig(t *testing.T) {
	t.Run("Local storage configuration", func(t *testing.T) {
		storageConfig := StorageConfig{
			Type: "local",
		}
		storageConfig.Local.Path = "/var/lib/gotrs/uploads"
		storageConfig.Local.PublicPath = "/uploads"

		assert.Equal(t, "local", storageConfig.Type)
		assert.Equal(t, "/var/lib/gotrs/uploads", storageConfig.Local.Path)
		assert.Equal(t, "/uploads", storageConfig.Local.PublicPath)
	})

	t.Run("S3 storage configuration", func(t *testing.T) {
		storageConfig := StorageConfig{
			Type: "s3",
		}
		storageConfig.S3.Bucket = "gotrs-attachments"
		storageConfig.S3.Region = "us-east-1"
		storageConfig.S3.AccessKey = "access-key"
		storageConfig.S3.SecretKey = "secret-key"
		storageConfig.S3.Endpoint = "https://s3.amazonaws.com"

		assert.Equal(t, "s3", storageConfig.Type)
		assert.Equal(t, "gotrs-attachments", storageConfig.S3.Bucket)
		assert.Equal(t, "us-east-1", storageConfig.S3.Region)
		assert.Equal(t, "https://s3.amazonaws.com", storageConfig.S3.Endpoint)
	})

	t.Run("Attachment configuration", func(t *testing.T) {
		storageConfig := StorageConfig{}
		storageConfig.Attachments.MaxSize = 10 * 1024 * 1024 // 10MB
		storageConfig.Attachments.AllowedTypes = []string{
			"image/jpeg",
			"image/png",
			"application/pdf",
			"text/plain",
		}

		assert.Equal(t, int64(10*1024*1024), storageConfig.Attachments.MaxSize)
		assert.Len(t, storageConfig.Attachments.AllowedTypes, 4)
		assert.Contains(t, storageConfig.Attachments.AllowedTypes, "application/pdf")
	})
}

func TestTicketConfig(t *testing.T) {
	t.Run("Ticket ID configuration", func(t *testing.T) {
		ticketConfig := TicketConfig{
			IDPrefix:        "TKT",
			IDFormat:        "%s-%d",
			DefaultPriority: "normal",
			DefaultStatus:   "open",
			AutoAssign:      true,
		}

		assert.Equal(t, "TKT", ticketConfig.IDPrefix)
		assert.Equal(t, "%s-%d", ticketConfig.IDFormat)
		assert.Equal(t, "normal", ticketConfig.DefaultPriority)
		assert.Equal(t, "open", ticketConfig.DefaultStatus)
		assert.True(t, ticketConfig.AutoAssign)
	})

	t.Run("SLA configuration", func(t *testing.T) {
		ticketConfig := TicketConfig{}
		ticketConfig.SLA.Enabled = true
		ticketConfig.SLA.FirstResponse = 2 * time.Hour
		ticketConfig.SLA.Resolution = 24 * time.Hour

		assert.True(t, ticketConfig.SLA.Enabled)
		assert.Equal(t, 2*time.Hour, ticketConfig.SLA.FirstResponse)
		assert.Equal(t, 24*time.Hour, ticketConfig.SLA.Resolution)
	})

	t.Run("Notification preferences", func(t *testing.T) {
		ticketConfig := TicketConfig{}
		ticketConfig.Notifications.CustomerCreate = true
		ticketConfig.Notifications.CustomerUpdate = true
		ticketConfig.Notifications.AgentAssign = true
		ticketConfig.Notifications.AgentMention = false

		assert.True(t, ticketConfig.Notifications.CustomerCreate)
		assert.True(t, ticketConfig.Notifications.CustomerUpdate)
		assert.True(t, ticketConfig.Notifications.AgentAssign)
		assert.False(t, ticketConfig.Notifications.AgentMention)
	})
}

func TestFeaturesConfig(t *testing.T) {
	t.Run("Feature flags", func(t *testing.T) {
		features := FeaturesConfig{
			Registration:            true,
			SocialLogin:            false,
			TwoFactorAuth:          true,
			APIKeys:                true,
			Webhooks:               true,
			LDAP:                   false,
			SAML:                   false,
			KnowledgeBase:          true,
			CustomerPortal:         true,
			AgentCollisionDetection: true,
		}

		assert.True(t, features.Registration)
		assert.False(t, features.SocialLogin)
		assert.True(t, features.TwoFactorAuth)
		assert.True(t, features.APIKeys)
		assert.True(t, features.Webhooks)
		assert.False(t, features.LDAP)
		assert.False(t, features.SAML)
		assert.True(t, features.KnowledgeBase)
		assert.True(t, features.CustomerPortal)
		assert.True(t, features.AgentCollisionDetection)
	})
}

func TestMaintenanceConfig(t *testing.T) {
	t.Run("Maintenance mode configuration", func(t *testing.T) {
		maintenance := MaintenanceConfig{
			Mode:    true,
			Message: "System is under maintenance. Please check back later.",
			AllowedIPs: []string{
				"127.0.0.1",
				"192.168.1.0/24",
			},
		}

		assert.True(t, maintenance.Mode)
		assert.Contains(t, maintenance.Message, "maintenance")
		assert.Len(t, maintenance.AllowedIPs, 2)
		assert.Contains(t, maintenance.AllowedIPs, "127.0.0.1")
	})
}

func TestIntegrationsConfig(t *testing.T) {
	t.Run("Slack integration", func(t *testing.T) {
		integrations := IntegrationsConfig{}
		integrations.Slack.Enabled = true
		integrations.Slack.WebhookURL = "https://hooks.slack.com/services/xxx"

		assert.True(t, integrations.Slack.Enabled)
		assert.Contains(t, integrations.Slack.WebhookURL, "slack.com")
	})

	t.Run("Teams integration", func(t *testing.T) {
		integrations := IntegrationsConfig{}
		integrations.Teams.Enabled = true
		integrations.Teams.WebhookURL = "https://outlook.office.com/webhook/xxx"

		assert.True(t, integrations.Teams.Enabled)
		assert.Contains(t, integrations.Teams.WebhookURL, "office.com")
	})

	t.Run("Generic webhook configuration", func(t *testing.T) {
		integrations := IntegrationsConfig{}
		integrations.Webhook.Enabled = true
		integrations.Webhook.Endpoints = []string{
			"https://api.example.com/webhook",
			"https://webhook.site/xxx",
		}
		integrations.Webhook.Timeout = 30 * time.Second
		integrations.Webhook.RetryAttempts = 3

		assert.True(t, integrations.Webhook.Enabled)
		assert.Len(t, integrations.Webhook.Endpoints, 2)
		assert.Equal(t, 30*time.Second, integrations.Webhook.Timeout)
		assert.Equal(t, 3, integrations.Webhook.RetryAttempts)
	})
}

func TestLoadFromFile(t *testing.T) {
	t.Run("Load valid YAML config file", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "test-config.yaml")
		
		configContent := `
app:
  name: GOTRS Test
  version: 1.0.0
  env: test
  debug: true

server:
  host: localhost
  port: 8080

database:
  host: localhost
  port: 5432
  name: gotrs_test
  user: test_user
  password: test_pass
  ssl_mode: disable
`
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		// Reset the config singleton
		mu.Lock()
		cfg = nil
		once = sync.Once{}
		mu.Unlock()

		// Load the config
		err = LoadFromFile(configFile)
		require.NoError(t, err)

		// Verify the loaded config
		loadedCfg := Get()
		assert.NotNil(t, loadedCfg)
		assert.Equal(t, "GOTRS Test", loadedCfg.App.Name)
		assert.Equal(t, "1.0.0", loadedCfg.App.Version)
		assert.Equal(t, "test", loadedCfg.App.Env)
		assert.True(t, loadedCfg.App.Debug)
		assert.Equal(t, "localhost", loadedCfg.Server.Host)
		assert.Equal(t, 8080, loadedCfg.Server.Port)
		assert.Equal(t, "gotrs_test", loadedCfg.Database.Name)
	})

	t.Run("Error on non-existent file", func(t *testing.T) {
		err := LoadFromFile("/non/existent/config.yaml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

	t.Run("Error on invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "invalid-config.yaml")
		
		invalidContent := `
app:
  name: [this is invalid
  version: 1.0.0
`
		err := os.WriteFile(configFile, []byte(invalidContent), 0644)
		require.NoError(t, err)

		err = LoadFromFile(configFile)
		assert.Error(t, err)
		// The error comes from viper.ReadInConfig when YAML is invalid
		assert.Contains(t, err.Error(), "failed to read config file")
	})
}

func TestGet(t *testing.T) {
	t.Run("Get returns config instance", func(t *testing.T) {
		// Reset and set up a config
		mu.Lock()
		cfg = &Config{
			App: AppConfig{
				Name:    "Test App",
				Version: "1.0.0",
			},
		}
		mu.Unlock()

		retrieved := Get()
		assert.NotNil(t, retrieved)
		assert.Equal(t, "Test App", retrieved.App.Name)
		assert.Equal(t, "1.0.0", retrieved.App.Version)
	})

	t.Run("Get is thread-safe", func(t *testing.T) {
		// Reset and set up a config
		mu.Lock()
		cfg = &Config{
			App: AppConfig{
				Name: "Concurrent Test",
			},
		}
		mu.Unlock()

		// Run concurrent reads
		var wg sync.WaitGroup
		errors := make([]error, 100)
		
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				retrieved := Get()
				if retrieved == nil {
					errors[idx] = fmt.Errorf("config was nil")
				} else if retrieved.App.Name != "Concurrent Test" {
					errors[idx] = fmt.Errorf("unexpected app name: %s", retrieved.App.Name)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Check for errors
		for _, err := range errors {
			assert.NoError(t, err)
		}
	})
}

func TestMustLoad(t *testing.T) {
	t.Run("MustLoad panics on error", func(t *testing.T) {
		defer func() {
			r := recover()
			assert.NotNil(t, r)
			assert.Contains(t, r.(string), "Failed to load configuration")
		}()

		// Reset the singleton
		mu.Lock()
		cfg = nil
		once = sync.Once{}
		mu.Unlock()

		// This should panic
		MustLoad("/non/existent/path")
	})
}

func TestConcurrentConfigAccess(t *testing.T) {
	// Set up initial config
	mu.Lock()
	cfg = &Config{
		App: AppConfig{
			Name:    "Concurrent App",
			Version: "1.0.0",
		},
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}
	mu.Unlock()

	// Run concurrent operations
	var wg sync.WaitGroup
	
	// Readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c := Get()
				_ = c.App.IsProduction()
				_ = c.Server.GetServerAddr()
			}
		}()
	}
	
	// Writers (simulating config updates)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				mu.Lock()
				cfg = &Config{
					App: AppConfig{
						Name:    fmt.Sprintf("App %d", id),
						Version: fmt.Sprintf("%d.0.0", id),
					},
				}
				mu.Unlock()
				time.Sleep(time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state is consistent
	finalCfg := Get()
	assert.NotNil(t, finalCfg)
}

func BenchmarkGetConfig(b *testing.B) {
	// Set up config
	mu.Lock()
	cfg = &Config{
		App: AppConfig{
			Name:    "Benchmark App",
			Version: "1.0.0",
		},
	}
	mu.Unlock()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = Get()
		}
	})
}

func BenchmarkGetDSN(b *testing.B) {
	dbConfig := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Password: "pass",
		Name:     "db",
		SSLMode:  "disable",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dbConfig.GetDSN()
	}
}

func BenchmarkIsProduction(b *testing.B) {
	appConfig := &AppConfig{
		Env: "production",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = appConfig.IsProduction()
	}
}