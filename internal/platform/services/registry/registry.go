// Package registry provides service registration and lifecycle management.
package registry

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServiceRegistry manages all registered services.
type ServiceRegistry struct {
	mu           sync.RWMutex
	services     map[string]ServiceInterface
	bindings     map[string][]*ServiceBinding
	factories    map[ServiceType]ServiceFactory
	healthChecks map[string]*time.Ticker
	migrations   map[string]*ServiceMigration

	// Event channels
	healthEvents chan ServiceHealth
	errorEvents  chan error

	// Context for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServiceRegistry creates a new service registry.
func NewServiceRegistry() *ServiceRegistry {
	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceRegistry{
		services:     make(map[string]ServiceInterface),
		bindings:     make(map[string][]*ServiceBinding),
		factories:    make(map[ServiceType]ServiceFactory),
		healthChecks: make(map[string]*time.Ticker),
		migrations:   make(map[string]*ServiceMigration),
		healthEvents: make(chan ServiceHealth, 100),
		errorEvents:  make(chan error, 100),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// RegisterFactory registers a service factory for a specific service type.
func (r *ServiceRegistry) RegisterFactory(serviceType ServiceType, factory ServiceFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[serviceType]; exists {
		return fmt.Errorf("factory for service type %s already registered", serviceType)
	}

	r.factories[serviceType] = factory
	return nil
}

// RegisterService registers a service instance.
func (r *ServiceRegistry) RegisterService(config *ServiceConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if service already exists
	if _, exists := r.services[config.ID]; exists {
		return fmt.Errorf("service %s already registered", config.ID)
	}

	// Get the appropriate factory
	factory, exists := r.factories[config.Type]
	if !exists {
		return fmt.Errorf("no factory registered for service type %s", config.Type)
	}

	// Create the service instance
	service, err := factory.CreateService(config)
	if err != nil {
		return fmt.Errorf("failed to create service %s: %w", config.ID, err)
	}

	// Try to connect to the service, but don't fail registration if it fails
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	if err := service.Connect(ctx); err != nil {
		// Log the connection error but don't fail registration
		// The service can be connected later when needed
		fmt.Printf("Warning: Failed to connect to service %s during registration: %v\n", config.ID, err)
	}
	cancel()

	// Register the service
	r.services[config.ID] = service

	// Start health monitoring
	r.startHealthMonitoring(config.ID, service)

	return nil
}

// UnregisterService removes a service from the registry.
func (r *ServiceRegistry) UnregisterService(serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, exists := r.services[serviceID]
	if !exists {
		return fmt.Errorf("service %s not found", serviceID)
	}

	// Stop health monitoring
	if ticker, exists := r.healthChecks[serviceID]; exists {
		ticker.Stop()
		delete(r.healthChecks, serviceID)
	}

	// Disconnect the service
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	if err := service.Disconnect(ctx); err != nil {
		// Log error but continue with unregistration
		r.errorEvents <- fmt.Errorf("error disconnecting service %s: %w", serviceID, err)
	}

	// Remove all bindings for this service
	delete(r.bindings, serviceID)

	// Remove the service
	delete(r.services, serviceID)

	return nil
}

// GetService retrieves a service by ID.
func (r *ServiceRegistry) GetService(serviceID string) (ServiceInterface, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[serviceID]
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceID)
	}

	return service, nil
}

// GetServicesByType retrieves all services of a specific type.
func (r *ServiceRegistry) GetServicesByType(serviceType ServiceType) []ServiceInterface {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var services []ServiceInterface
	for _, service := range r.services {
		if service.Type() == serviceType {
			services = append(services, service)
		}
	}

	return services
}

// CreateBinding creates a binding between an application and a service.
func (r *ServiceRegistry) CreateBinding(binding *ServiceBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Verify service exists
	if _, exists := r.services[binding.ServiceID]; !exists {
		return fmt.Errorf("service %s not found", binding.ServiceID)
	}

	// Add binding
	binding.CreatedAt = time.Now()
	binding.UpdatedAt = time.Now()

	r.bindings[binding.AppID] = append(r.bindings[binding.AppID], binding)

	return nil
}

// GetBindings retrieves all bindings for an application.
func (r *ServiceRegistry) GetBindings(appID string) []*ServiceBinding {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.bindings[appID]
}

// GetBoundService retrieves a service bound to an application by purpose.
func (r *ServiceRegistry) GetBoundService(appID string, purpose string) (ServiceInterface, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bindings := r.bindings[appID]

	// Find binding with highest priority for the given purpose
	var bestBinding *ServiceBinding
	for _, binding := range bindings {
		if binding.Purpose == purpose {
			if bestBinding == nil || binding.Priority > bestBinding.Priority {
				bestBinding = binding
			}
		}
	}

	if bestBinding == nil {
		return nil, fmt.Errorf("no service bound for app %s with purpose %s", appID, purpose)
	}

	service, exists := r.services[bestBinding.ServiceID]
	if !exists {
		return nil, fmt.Errorf("bound service %s not found", bestBinding.ServiceID)
	}

	return service, nil
}

// startHealthMonitoring starts periodic health checks for a service.
func (r *ServiceRegistry) startHealthMonitoring(serviceID string, service ServiceInterface) {
	ticker := time.NewTicker(30 * time.Second)
	r.healthChecks[serviceID] = ticker

	go func() {
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
				health, err := service.Health(ctx)
				cancel()

				if err != nil {
					r.errorEvents <- fmt.Errorf("health check failed for service %s: %w", serviceID, err)
					health = &ServiceHealth{
						ServiceID:   serviceID,
						Status:      StatusUnhealthy,
						LastChecked: time.Now(),
						Error:       err.Error(),
					}
				}

				// Send health event
				select {
				case r.healthEvents <- *health:
				default:
					// Channel full, skip this event
				}

			case <-r.ctx.Done():
				return
			}
		}
	}()
}

// StartMigration initiates a migration from one service to another.
func (r *ServiceRegistry) StartMigration(fromServiceID, toServiceID string, strategy MigrationStrategy) (*ServiceMigration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Verify both services exist
	if _, exists := r.services[fromServiceID]; !exists {
		return nil, fmt.Errorf("source service %s not found", fromServiceID)
	}

	if _, exists := r.services[toServiceID]; !exists {
		return nil, fmt.Errorf("target service %s not found", toServiceID)
	}

	// Create migration
	migration := &ServiceMigration{
		ID:          fmt.Sprintf("migration-%d", time.Now().Unix()),
		FromService: fromServiceID,
		ToService:   toServiceID,
		Strategy:    strategy,
		Progress:    0.0,
		StartedAt:   time.Now(),
		Status:      "in_progress",
	}

	r.migrations[migration.ID] = migration

	// Start migration process (simplified for now)
	go r.executeMigration(migration)

	return migration, nil
}

// executeMigration performs the actual migration.
func (r *ServiceRegistry) executeMigration(migration *ServiceMigration) {
	// This is a simplified implementation
	// In a real system, this would handle data migration, traffic shifting, etc.

	switch migration.Strategy {
	case MigrationImmediate:
		// Immediately switch all bindings
		r.mu.Lock()
		for appID, bindings := range r.bindings {
			for _, binding := range bindings {
				if binding.ServiceID == migration.FromService {
					binding.ServiceID = migration.ToService
					binding.UpdatedAt = time.Now()
				}
			}
			r.bindings[appID] = bindings
		}
		migration.Progress = 1.0
		migration.Status = "completed"
		now := time.Now()
		migration.CompletedAt = &now
		r.mu.Unlock()

	case MigrationCanary:
		// Gradually shift traffic (simplified)
		steps := 10
		for i := 0; i <= steps; i++ {
			time.Sleep(5 * time.Second)
			migration.Progress = float64(i) / float64(steps)
		}
		migration.Status = "completed"
		now := time.Now()
		migration.CompletedAt = &now

	default:
		migration.Status = "failed"
		migration.Error = "unsupported migration strategy"
	}
}

// GetMigration retrieves a migration by ID.
func (r *ServiceRegistry) GetMigration(migrationID string) (*ServiceMigration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	migration, exists := r.migrations[migrationID]
	if !exists {
		return nil, fmt.Errorf("migration %s not found", migrationID)
	}

	return migration, nil
}

// HealthEvents returns a channel for receiving health events.
func (r *ServiceRegistry) HealthEvents() <-chan ServiceHealth {
	return r.healthEvents
}

// ErrorEvents returns a channel for receiving error events.
func (r *ServiceRegistry) ErrorEvents() <-chan error {
	return r.errorEvents
}

// Shutdown gracefully shuts down the registry.
func (r *ServiceRegistry) Shutdown(ctx context.Context) error {
	r.cancel()

	// Stop all health checks
	for _, ticker := range r.healthChecks {
		ticker.Stop()
	}

	// Disconnect all services
	var errors []error
	for id, service := range r.services {
		if err := service.Disconnect(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to disconnect service %s: %w", id, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}
