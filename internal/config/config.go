package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	cfg  *Config
	once sync.Once
	mu   sync.RWMutex
)

// Config represents the application configuration
type Config struct {
	App          AppConfig          `mapstructure:"app"`
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Auth         AuthConfig         `mapstructure:"auth"`
	Email        EmailConfig        `mapstructure:"email"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Ticket       TicketConfig       `mapstructure:"ticket"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Metrics      MetricsConfig      `mapstructure:"metrics"`
	RateLimiting RateLimitingConfig `mapstructure:"rate_limiting"`
	Features     FeaturesConfig     `mapstructure:"features"`
	Maintenance  MaintenanceConfig  `mapstructure:"maintenance"`
	Integrations IntegrationsConfig `mapstructure:"integrations"`
}

type AppConfig struct {
	Name     string `mapstructure:"name"`
	Version  string `mapstructure:"version"`
	Env      string `mapstructure:"env"`
	Debug    bool   `mapstructure:"debug"`
	Timezone string `mapstructure:"timezone"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	CORS            CORSConfig    `mapstructure:"cors"`
}

type CORSConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Origins []string `mapstructure:"origins"`
	Methods []string `mapstructure:"methods"`
	Headers []string `mapstructure:"headers"`
}

type DatabaseConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	Name           string        `mapstructure:"name"`
	User           string        `mapstructure:"user"`
	Password       string        `mapstructure:"password"`
	SSLMode        string        `mapstructure:"ssl_mode"`
	MaxOpenConns   int           `mapstructure:"max_open_conns"`
	MaxIdleConns   int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	LogQueries     bool          `mapstructure:"log_queries"`
	Migrations     struct {
		AutoMigrate bool   `mapstructure:"auto_migrate"`
		Path        string `mapstructure:"path"`
	} `mapstructure:"migrations"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	MaxRetries   int    `mapstructure:"max_retries"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	Session      struct {
		Prefix string        `mapstructure:"prefix"`
		TTL    time.Duration `mapstructure:"ttl"`
	} `mapstructure:"session"`
	Cache struct {
		Prefix string        `mapstructure:"prefix"`
		TTL    time.Duration `mapstructure:"ttl"`
	} `mapstructure:"cache"`
}

type AuthConfig struct {
	JWT struct {
		Secret           string        `mapstructure:"secret"`
		Issuer           string        `mapstructure:"issuer"`
		Audience         string        `mapstructure:"audience"`
		AccessTokenTTL   time.Duration `mapstructure:"access_token_ttl"`
		RefreshTokenTTL  time.Duration `mapstructure:"refresh_token_ttl"`
	} `mapstructure:"jwt"`
	Session struct {
		CookieName string `mapstructure:"cookie_name"`
		Secure     bool   `mapstructure:"secure"`
		HTTPOnly   bool   `mapstructure:"http_only"`
		SameSite   string `mapstructure:"same_site"`
		MaxAge     int    `mapstructure:"max_age"`
	} `mapstructure:"session"`
	Password struct {
		MinLength        int  `mapstructure:"min_length"`
		RequireUppercase bool `mapstructure:"require_uppercase"`
		RequireLowercase bool `mapstructure:"require_lowercase"`
		RequireNumber    bool `mapstructure:"require_number"`
		RequireSpecial   bool `mapstructure:"require_special"`
		BcryptCost       int  `mapstructure:"bcrypt_cost"`
	} `mapstructure:"password"`
}

type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	From     string `mapstructure:"from"`
	FromName string `mapstructure:"from_name"`
	SMTP     struct {
		Host       string `mapstructure:"host"`
		Port       int    `mapstructure:"port"`
		User       string `mapstructure:"user"`
		Password   string `mapstructure:"password"`
		AuthType   string `mapstructure:"auth_type"`
		TLS        bool   `mapstructure:"tls"`
		SkipVerify bool   `mapstructure:"skip_verify"`
	} `mapstructure:"smtp"`
	Templates struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"templates"`
	Queue struct {
		Workers       int           `mapstructure:"workers"`
		BufferSize    int           `mapstructure:"buffer_size"`
		RetryAttempts int           `mapstructure:"retry_attempts"`
		RetryDelay    time.Duration `mapstructure:"retry_delay"`
	} `mapstructure:"queue"`
}

type StorageConfig struct {
	Type  string `mapstructure:"type"`
	Local struct {
		Path       string `mapstructure:"path"`
		PublicPath string `mapstructure:"public_path"`
	} `mapstructure:"local"`
	S3 struct {
		Bucket    string `mapstructure:"bucket"`
		Region    string `mapstructure:"region"`
		AccessKey string `mapstructure:"access_key"`
		SecretKey string `mapstructure:"secret_key"`
		Endpoint  string `mapstructure:"endpoint"`
	} `mapstructure:"s3"`
	Attachments struct {
		MaxSize      int64    `mapstructure:"max_size"`
		AllowedTypes []string `mapstructure:"allowed_types"`
	} `mapstructure:"attachments"`
}

type TicketConfig struct {
	IDPrefix       string `mapstructure:"id_prefix"`
	IDFormat       string `mapstructure:"id_format"`
	DefaultPriority string `mapstructure:"default_priority"`
	DefaultStatus  string `mapstructure:"default_status"`
	AutoAssign     bool   `mapstructure:"auto_assign"`
	SLA            struct {
		Enabled       bool          `mapstructure:"enabled"`
		FirstResponse time.Duration `mapstructure:"first_response"`
		Resolution    time.Duration `mapstructure:"resolution"`
	} `mapstructure:"sla"`
	Notifications struct {
		CustomerCreate bool `mapstructure:"customer_create"`
		CustomerUpdate bool `mapstructure:"customer_update"`
		AgentAssign    bool `mapstructure:"agent_assign"`
		AgentMention   bool `mapstructure:"agent_mention"`
	} `mapstructure:"notifications"`
}

type LoggingConfig struct {
	Level  string   `mapstructure:"level"`
	Format string   `mapstructure:"format"`
	Output string   `mapstructure:"output"`
	Fields []string `mapstructure:"fields"`
	File   struct {
		Path       string `mapstructure:"path"`
		Filename   string `mapstructure:"filename"`
		MaxSize    int    `mapstructure:"max_size"`
		MaxBackups int    `mapstructure:"max_backups"`
		MaxAge     int    `mapstructure:"max_age"`
		Compress   bool   `mapstructure:"compress"`
	} `mapstructure:"file"`
}

type MetricsConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	Prometheus struct {
		Enabled bool   `mapstructure:"enabled"`
		Port    int    `mapstructure:"port"`
		Path    string `mapstructure:"path"`
	} `mapstructure:"prometheus"`
	OpenTelemetry struct {
		Enabled     bool    `mapstructure:"enabled"`
		Endpoint    string  `mapstructure:"endpoint"`
		ServiceName string  `mapstructure:"service_name"`
		TraceRatio  float64 `mapstructure:"trace_ratio"`
	} `mapstructure:"opentelemetry"`
}

type RateLimitingConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	RequestsPerMinute  int      `mapstructure:"requests_per_minute"`
	Burst              int      `mapstructure:"burst"`
	ExcludePaths       []string `mapstructure:"exclude_paths"`
}

type FeaturesConfig struct {
	Registration           bool `mapstructure:"registration"`
	SocialLogin           bool `mapstructure:"social_login"`
	TwoFactorAuth         bool `mapstructure:"two_factor_auth"`
	APIKeys               bool `mapstructure:"api_keys"`
	Webhooks              bool `mapstructure:"webhooks"`
	LDAP                  bool `mapstructure:"ldap"`
	SAML                  bool `mapstructure:"saml"`
	KnowledgeBase         bool `mapstructure:"knowledge_base"`
	CustomerPortal        bool `mapstructure:"customer_portal"`
	AgentCollisionDetection bool `mapstructure:"agent_collision_detection"`
}

type MaintenanceConfig struct {
	Mode       bool     `mapstructure:"mode"`
	Message    string   `mapstructure:"message"`
	AllowedIPs []string `mapstructure:"allowed_ips"`
}

type IntegrationsConfig struct {
	Slack struct {
		Enabled    bool   `mapstructure:"enabled"`
		WebhookURL string `mapstructure:"webhook_url"`
	} `mapstructure:"slack"`
	Teams struct {
		Enabled    bool   `mapstructure:"enabled"`
		WebhookURL string `mapstructure:"webhook_url"`
	} `mapstructure:"teams"`
	Webhook struct {
		Enabled       bool          `mapstructure:"enabled"`
		Endpoints     []string      `mapstructure:"endpoints"`
		Timeout       time.Duration `mapstructure:"timeout"`
		RetryAttempts int           `mapstructure:"retry_attempts"`
	} `mapstructure:"webhook"`
}

// Load initializes the configuration with hot reload support
func Load(configPath string) error {
	var err error
	once.Do(func() {
		v := viper.New()

		// Set config type
		v.SetConfigType("yaml")

		// Load default configuration
		v.SetConfigName("default")
		v.AddConfigPath(configPath)
		if err = v.ReadInConfig(); err != nil {
			err = fmt.Errorf("failed to read default config: %w", err)
			return
		}

		// Load environment-specific config (optional)
		v.SetConfigName("config")
		if err = v.MergeInConfig(); err != nil {
			// It's OK if config.yaml doesn't exist
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				err = fmt.Errorf("failed to merge config: %w", err)
				return
			}
			err = nil
		}

		// Environment variable overrides
		v.SetEnvPrefix("GOTRS")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		// Unmarshal configuration
		cfg = &Config{}
		if err = v.Unmarshal(cfg); err != nil {
			err = fmt.Errorf("failed to unmarshal config: %w", err)
			return
		}

		// Watch for config changes
		v.WatchConfig()
		v.OnConfigChange(func(e fsnotify.Event) {
			fmt.Printf("Config file changed: %s\n", e.Name)
			mu.Lock()
			defer mu.Unlock()

			// Create new config instance
			newCfg := &Config{}
			if err := v.Unmarshal(newCfg); err != nil {
				fmt.Printf("Failed to reload config: %v\n", err)
				return
			}

			// Atomic swap
			cfg = newCfg
			fmt.Println("Configuration reloaded successfully")
		})
	})

	return err
}

// Get returns the current configuration (thread-safe)
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}

// GetDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// GetRedisAddr returns the Redis server address
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetServerAddr returns the server listen address
func (c *ServerConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// IsProduction returns true if running in production mode
func (c *AppConfig) IsProduction() bool {
	return c.Env == "production"
}

// IsDevelopment returns true if running in development mode
func (c *AppConfig) IsDevelopment() bool {
	return c.Env == "development"
}

// LoadFromFile loads configuration from a specific file (useful for testing)
func LoadFromFile(configFile string) error {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// MustLoad loads configuration and panics on error
func MustLoad(configPath string) {
	if err := Load(configPath); err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}
}