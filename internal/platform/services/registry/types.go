package registry

import (
	"context"
	"time"
)

// ServiceType represents the type of service.
type ServiceType string

const (
	ServiceTypeDatabase   ServiceType = "database"
	ServiceTypeCache      ServiceType = "cache"
	ServiceTypeQueue      ServiceType = "queue"
	ServiceTypeStorage    ServiceType = "storage"
	ServiceTypeSearch     ServiceType = "search"
	ServiceTypeAuth       ServiceType = "auth"
	ServiceTypeEmail      ServiceType = "email"
	ServiceTypeMonitoring ServiceType = "monitoring"
	ServiceTypeWorkflow   ServiceType = "workflow"
	ServiceTypeSecrets    ServiceType = "secrets"
	ServiceTypeRouter     ServiceType = "router"
)

// ServiceStatus represents the current status of a service.
type ServiceStatus string

const (
	StatusHealthy      ServiceStatus = "healthy"
	StatusDegraded     ServiceStatus = "degraded"
	StatusUnhealthy    ServiceStatus = "unhealthy"
	StatusInitializing ServiceStatus = "initializing"
	StatusUnknown      ServiceStatus = "unknown"
)

// ServiceProvider represents a backend provider (e.g., postgres, mysql, redis).
type ServiceProvider string

const (
	// Database providers.
	ProviderPostgres  ServiceProvider = "postgres"
	ProviderMySQL     ServiceProvider = "mysql"
	ProviderSQLite    ServiceProvider = "sqlite"
	ProviderMongoDB   ServiceProvider = "mongodb"
	ProviderCockroach ServiceProvider = "cockroachdb"

	// Cache providers.
	ProviderRedis     ServiceProvider = "redis"
	ProviderValkey    ServiceProvider = "valkey"
	ProviderMemcache  ServiceProvider = "memcache"
	ProviderHazelcast ServiceProvider = "hazelcast"

	// Queue providers.
	ProviderRabbitMQ ServiceProvider = "rabbitmq"
	ProviderKafka    ServiceProvider = "kafka"
	ProviderNATS     ServiceProvider = "nats"
	ProviderSQS      ServiceProvider = "aws-sqs"
	ProviderPubSub   ServiceProvider = "gcp-pubsub"

	// Storage providers.
	ProviderS3        ServiceProvider = "s3"
	ProviderGCS       ServiceProvider = "gcs"
	ProviderAzureBlob ServiceProvider = "azure-blob"
	ProviderMinIO     ServiceProvider = "minio"
	ProviderLocal     ServiceProvider = "local-fs"

	// Search providers.
	ProviderElasticsearch ServiceProvider = "elasticsearch"
	ProviderOpenSearch    ServiceProvider = "opensearch"
	ProviderZinc          ServiceProvider = "zinc"
	ProviderMeilisearch   ServiceProvider = "meilisearch"
	ProviderTypesense     ServiceProvider = "typesense"
)

// ServiceConfig holds the configuration for a service instance.
type ServiceConfig struct {
	// Basic identification
	ID       string          `yaml:"id" json:"id"`
	Name     string          `yaml:"name" json:"name"`
	Type     ServiceType     `yaml:"type" json:"type"`
	Provider ServiceProvider `yaml:"provider" json:"provider"`

	// Connection details
	Host     string `yaml:"host,omitempty" json:"host,omitempty"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty"`
	Database string `yaml:"database,omitempty" json:"database,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Advanced options
	MaxConns   int           `yaml:"max_conns,omitempty" json:"max_conns,omitempty"`
	MinConns   int           `yaml:"min_conns,omitempty" json:"min_conns,omitempty"`
	Timeout    time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RetryCount int           `yaml:"retry_count,omitempty" json:"retry_count,omitempty"`
	TLS        bool          `yaml:"tls,omitempty" json:"tls,omitempty"`

	// Provider-specific settings
	Options map[string]interface{} `yaml:"options,omitempty" json:"options,omitempty"`

	// Metadata
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// ServiceBinding represents a binding between an application and a service.
type ServiceBinding struct {
	ID        string    `yaml:"id" json:"id"`
	AppID     string    `yaml:"app_id" json:"app_id"`
	ServiceID string    `yaml:"service_id" json:"service_id"`
	Name      string    `yaml:"name" json:"name"`
	Purpose   string    `yaml:"purpose" json:"purpose"` // e.g., "primary", "replica", "analytics"
	Priority  int       `yaml:"priority" json:"priority"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`
}

// ServiceHealth represents the health status of a service.
type ServiceHealth struct {
	ServiceID   string                 `json:"service_id"`
	Status      ServiceStatus          `json:"status"`
	Latency     time.Duration          `json:"latency"`
	LastChecked time.Time              `json:"last_checked"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceMetrics represents metrics for a service.
type ServiceMetrics struct {
	ServiceID     string                 `json:"service_id"`
	Connections   int                    `json:"connections"`
	Requests      int64                  `json:"requests"`
	Errors        int64                  `json:"errors"`
	Latency       time.Duration          `json:"latency"`
	Throughput    float64                `json:"throughput"`
	CustomMetrics map[string]interface{} `json:"custom_metrics,omitempty"`
}

// ServiceInterface defines the common interface all services must implement.
type ServiceInterface interface {
	// Lifecycle methods
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Ping(ctx context.Context) error

	// Health and monitoring
	Health(ctx context.Context) (*ServiceHealth, error)
	Metrics(ctx context.Context) (*ServiceMetrics, error)

	// Configuration
	GetConfig() *ServiceConfig
	UpdateConfig(config *ServiceConfig) error

	// Type information
	Type() ServiceType
	Provider() ServiceProvider
	ID() string
}

// ServiceFactory creates service instances based on configuration.
type ServiceFactory interface {
	CreateService(config *ServiceConfig) (ServiceInterface, error)
	SupportedProviders() []ServiceProvider
}

// MigrationStrategy defines how to migrate between services.
type MigrationStrategy string

const (
	MigrationBlueGreen MigrationStrategy = "blue-green"
	MigrationCanary    MigrationStrategy = "canary"
	MigrationGradual   MigrationStrategy = "gradual"
	MigrationImmediate MigrationStrategy = "immediate"
)

// ServiceMigration represents a migration from one service to another.
type ServiceMigration struct {
	ID          string            `json:"id"`
	FromService string            `json:"from_service"`
	ToService   string            `json:"to_service"`
	Strategy    MigrationStrategy `json:"strategy"`
	Progress    float64           `json:"progress"` // 0.0 to 1.0
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Status      string            `json:"status"`
	Error       string            `json:"error,omitempty"`
}
