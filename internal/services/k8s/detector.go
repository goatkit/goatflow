// Package k8s provides Kubernetes environment detection and integration.
package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/goatkit/goatflow/internal/services/registry"
)

// EnvironmentType represents the deployment environment.
type EnvironmentType string

const (
	EnvLocal      EnvironmentType = "local"
	EnvDocker     EnvironmentType = "docker"
	EnvKubernetes EnvironmentType = "kubernetes"
	EnvKnative    EnvironmentType = "knative"
	EnvOpenShift  EnvironmentType = "openshift"
)

// Detector detects the runtime environment and adapts service configuration.
type Detector struct {
	environment EnvironmentType
	namespace   string
	cluster     string
}

// NewDetector creates a new environment detector.
func NewDetector() *Detector {
	d := &Detector{
		environment: detectEnvironment(),
		namespace:   os.Getenv("POD_NAMESPACE"),
		cluster:     os.Getenv("CLUSTER_NAME"),
	}

	if d.namespace == "" {
		d.namespace = "default"
	}

	return d
}

// detectEnvironment determines the current runtime environment.
func detectEnvironment() EnvironmentType {
	// Check for Knative
	if os.Getenv("K_SERVICE") != "" {
		return EnvKnative
	}

	// Check for OpenShift
	if _, err := os.Stat("/var/run/secrets/openshift.io"); err == nil {
		return EnvOpenShift
	}

	// Check for Kubernetes
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
		return EnvKubernetes
	}

	// Check for Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return EnvDocker
	}

	// Default to local
	return EnvLocal
}

// Environment returns the detected environment type.
func (d *Detector) Environment() EnvironmentType {
	return d.environment
}

// IsKubernetes returns true if running in any Kubernetes environment.
func (d *Detector) IsKubernetes() bool {
	return d.environment == EnvKubernetes ||
		d.environment == EnvKnative ||
		d.environment == EnvOpenShift
}

// AdaptServiceConfig adapts service configuration based on environment.
func (d *Detector) AdaptServiceConfig(config *registry.ServiceConfig) *registry.ServiceConfig {
	if !d.IsKubernetes() {
		return config
	}

	// In Kubernetes, adapt service discovery
	adapted := *config

	// Convert service names to Kubernetes DNS names
	if config.Host != "" && !strings.Contains(config.Host, ".") {
		// Assume it's a service name, convert to FQDN
		adapted.Host = fmt.Sprintf("%s.%s.svc.cluster.local", config.Host, d.namespace)
	}

	// Add Kubernetes-specific labels
	if adapted.Labels == nil {
		adapted.Labels = make(map[string]string)
	}
	adapted.Labels["kubernetes.io/namespace"] = d.namespace
	adapted.Labels["kubernetes.io/environment"] = string(d.environment)

	// Use Kubernetes secrets for credentials if available
	if d.environment == EnvKubernetes || d.environment == EnvOpenShift {
		d.loadFromSecrets(&adapted)
	}

	// For Knative, adapt for serverless
	if d.environment == EnvKnative {
		d.adaptForKnative(&adapted)
	}

	return &adapted
}

// loadFromSecrets loads credentials from Kubernetes secrets.
func (d *Detector) loadFromSecrets(config *registry.ServiceConfig) {
	// Check for mounted secrets
	secretPath := fmt.Sprintf("/var/run/secrets/%s", config.ID)

	// Try to load username from secret
	if data, err := os.ReadFile(fmt.Sprintf("%s/username", secretPath)); err == nil {
		config.Username = strings.TrimSpace(string(data))
	}

	// Try to load password from secret
	if data, err := os.ReadFile(fmt.Sprintf("%s/password", secretPath)); err == nil {
		config.Password = strings.TrimSpace(string(data))
	}

	// Try to load additional config from ConfigMap
	configMapPath := fmt.Sprintf("/etc/config/%s", config.ID)
	if data, err := os.ReadFile(fmt.Sprintf("%s/host", configMapPath)); err == nil {
		config.Host = strings.TrimSpace(string(data))
	}
}

// adaptForKnative adapts configuration for Knative Serving.
func (d *Detector) adaptForKnative(config *registry.ServiceConfig) {
	// Knative specific adaptations
	if config.Options == nil {
		config.Options = make(map[string]interface{})
	}

	// Use Knative service discovery
	config.Options["service_discovery"] = "knative"

	// Adapt connection pools for serverless
	if config.MaxConns > 5 {
		config.MaxConns = 5 // Lower max connections for serverless
	}
	if config.MinConns > 1 {
		config.MinConns = 1 // Minimal idle connections
	}

	// Add Knative annotations
	if config.Annotations == nil {
		config.Annotations = make(map[string]string)
	}
	config.Annotations["serving.knative.dev/service"] = os.Getenv("K_SERVICE")
	config.Annotations["serving.knative.dev/revision"] = os.Getenv("K_REVISION")
}

// DiscoverServices discovers services from Kubernetes API.
func (d *Detector) DiscoverServices(ctx context.Context, serviceType registry.ServiceType) ([]*registry.ServiceConfig, error) {
	if !d.IsKubernetes() {
		return nil, fmt.Errorf("service discovery only available in Kubernetes")
	}

	// This would use the Kubernetes client-go library to discover services
	// For now, return a simple implementation
	var services []*registry.ServiceConfig

	// Check for well-known service environment variables (Kubernetes service discovery)
	// Format: <SERVICE>_SERVICE_HOST and <SERVICE>_SERVICE_PORT

	// Example: Check for PostgreSQL service
	if host := os.Getenv("POSTGRES_SERVICE_HOST"); host != "" {
		port := os.Getenv("POSTGRES_SERVICE_PORT")
		if port == "" {
			port = "5432"
		}

		services = append(services, &registry.ServiceConfig{
			ID:       "discovered-postgres",
			Name:     "PostgreSQL (Discovered)",
			Type:     registry.ServiceTypeDatabase,
			Provider: registry.ProviderPostgres,
			Host:     host,
			Port:     atoi(port),
			Labels: map[string]string{
				"discovered": "true",
				"source":     "kubernetes",
			},
		})
	}

	// Check for Redis/Valkey service
	if host := os.Getenv("VALKEY_SERVICE_HOST"); host != "" {
		port := os.Getenv("VALKEY_SERVICE_PORT")
		if port == "" {
			port = "6379"
		}

		services = append(services, &registry.ServiceConfig{
			ID:       "discovered-valkey",
			Name:     "Valkey Cache (Discovered)",
			Type:     registry.ServiceTypeCache,
			Provider: registry.ProviderValkey,
			Host:     host,
			Port:     atoi(port),
			Labels: map[string]string{
				"discovered": "true",
				"source":     "kubernetes",
			},
		})
	}

	return services, nil
}

// WatchServices watches for service changes in Kubernetes.
func (d *Detector) WatchServices(ctx context.Context, callback func(*registry.ServiceConfig, string)) error {
	if !d.IsKubernetes() {
		return fmt.Errorf("service watching only available in Kubernetes")
	}

	// This would use the Kubernetes client-go library to watch for service changes
	// For now, return not implemented
	return fmt.Errorf("service watching not yet implemented")
}

// Helper function to convert string to int.
func atoi(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}
