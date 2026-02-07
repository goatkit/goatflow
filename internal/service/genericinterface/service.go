// Package genericinterface provides the GenericInterface service for web service execution.
// This is the core execution engine for both inbound (Provider) and outbound (Requester) operations.
package genericinterface

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

// Transport defines the interface for HTTP transports (REST, SOAP).
type Transport interface {
	// Execute performs an HTTP request using the transport configuration.
	Execute(ctx context.Context, config models.TransportHTTPConfig, request *Request) (*Response, error)
	// Type returns the transport type (e.g., "HTTP::REST", "HTTP::SOAP").
	Type() string
}

// Request represents an outbound request to a remote system.
type Request struct {
	// Operation is the invoker/operation name.
	Operation string
	// Data contains the request payload.
	Data map[string]interface{}
	// Method is the HTTP method (GET, POST, PUT, DELETE).
	Method string
	// Path is the endpoint path (can contain placeholders like :id).
	Path string
}

// Response represents a response from a remote system.
type Response struct {
	// Success indicates if the request succeeded.
	Success bool
	// Data contains the response payload.
	Data map[string]interface{}
	// RawData contains the raw response body.
	RawData []byte
	// StatusCode is the HTTP status code.
	StatusCode int
	// Error contains any error message.
	Error string
	// Headers contains response headers.
	Headers map[string]string
}

// Service is the main GenericInterface execution service.
type Service struct {
	mu         sync.RWMutex
	repo       *repository.WebserviceRepository
	transports map[string]Transport
	cache      *webserviceCache
	debug      bool
}

// webserviceCache caches webservice configurations.
type webserviceCache struct {
	mu       sync.RWMutex
	configs  map[string]*models.WebserviceConfig // by name
	configsByID map[int]*models.WebserviceConfig
	expiry   time.Time
	ttl      time.Duration
}

// NewService creates a new GenericInterface service.
func NewService(db *sql.DB) *Service {
	s := &Service{
		repo:       repository.NewWebserviceRepository(db),
		transports: make(map[string]Transport),
		cache: &webserviceCache{
			configs:     make(map[string]*models.WebserviceConfig),
			configsByID: make(map[int]*models.WebserviceConfig),
			ttl:         5 * time.Minute,
		},
		debug: false,
	}

	// Register default transports
	s.RegisterTransport(NewRESTTransport())
	s.RegisterTransport(NewSOAPTransport())

	return s
}

// SetDebug enables or disables debug logging.
func (s *Service) SetDebug(debug bool) {
	s.debug = debug
}

// RegisterTransport registers a transport implementation.
func (s *Service) RegisterTransport(t Transport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transports[t.Type()] = t
	if s.debug {
		log.Printf("GenericInterface: Registered transport %s", t.Type())
	}
}

// GetTransport returns a transport by type.
func (s *Service) GetTransport(transportType string) (Transport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.transports[transportType]
	if !ok {
		return nil, fmt.Errorf("transport type %q not registered", transportType)
	}
	return t, nil
}

// Invoke executes an invoker on a webservice.
// This is the main method for making outbound requests.
func (s *Service) Invoke(ctx context.Context, webserviceName, invokerName string, data map[string]interface{}) (*Response, error) {
	// Get webservice config
	ws, err := s.getWebserviceByName(ctx, webserviceName)
	if err != nil {
		return nil, fmt.Errorf("webservice %q not found: %w", webserviceName, err)
	}

	if !ws.IsValid() {
		return nil, fmt.Errorf("webservice %q is not valid/active", webserviceName)
	}

	// Get invoker config
	invoker := ws.GetInvoker(invokerName)
	if invoker == nil {
		return nil, fmt.Errorf("invoker %q not found in webservice %q", invokerName, webserviceName)
	}

	// Get transport
	transportType := ws.Config.Requester.Transport.Type
	transport, err := s.GetTransport(transportType)
	if err != nil {
		return nil, fmt.Errorf("transport error: %w", err)
	}

	// Build request
	request := &Request{
		Operation: invokerName,
		Data:      data,
	}

	// Apply outbound mapping if configured
	if invoker.MappingOutbound.Type != "" {
		mappedData, err := s.applyMapping(invoker.MappingOutbound, data)
		if err != nil {
			return nil, fmt.Errorf("outbound mapping error: %w", err)
		}
		request.Data = mappedData
	}

	// Execute request
	if s.debug {
		log.Printf("GenericInterface: Invoking %s.%s with transport %s", webserviceName, invokerName, transportType)
	}

	response, err := transport.Execute(ctx, ws.Config.Requester.Transport.Config, request)
	if err != nil {
		return nil, fmt.Errorf("transport execution error: %w", err)
	}

	// Apply inbound mapping if configured
	if invoker.MappingInbound.Type != "" && response.Data != nil {
		mappedData, err := s.applyMapping(invoker.MappingInbound, response.Data)
		if err != nil {
			return nil, fmt.Errorf("inbound mapping error: %w", err)
		}
		response.Data = mappedData
	}

	return response, nil
}

// InvokeWithController executes an invoker with specific controller/path settings.
// Used for REST APIs where the path may vary based on parameters.
func (s *Service) InvokeWithController(ctx context.Context, webserviceName, invokerName string, controller string, method string, data map[string]interface{}) (*Response, error) {
	// Get webservice config
	ws, err := s.getWebserviceByName(ctx, webserviceName)
	if err != nil {
		return nil, fmt.Errorf("webservice %q not found: %w", webserviceName, err)
	}

	if !ws.IsValid() {
		return nil, fmt.Errorf("webservice %q is not valid/active", webserviceName)
	}

	// Get invoker config
	invoker := ws.GetInvoker(invokerName)
	if invoker == nil {
		return nil, fmt.Errorf("invoker %q not found in webservice %q", invokerName, webserviceName)
	}

	// Get transport
	transportType := ws.Config.Requester.Transport.Type
	transport, err := s.GetTransport(transportType)
	if err != nil {
		return nil, fmt.Errorf("transport error: %w", err)
	}

	// Build request with custom controller
	request := &Request{
		Operation: invokerName,
		Data:      data,
		Method:    method,
		Path:      controller,
	}

	// Apply outbound mapping
	if invoker.MappingOutbound.Type != "" {
		mappedData, err := s.applyMapping(invoker.MappingOutbound, data)
		if err != nil {
			return nil, fmt.Errorf("outbound mapping error: %w", err)
		}
		request.Data = mappedData
	}

	// Execute request
	response, err := transport.Execute(ctx, ws.Config.Requester.Transport.Config, request)
	if err != nil {
		return nil, fmt.Errorf("transport execution error: %w", err)
	}

	// Apply inbound mapping
	if invoker.MappingInbound.Type != "" && response.Data != nil {
		mappedData, err := s.applyMapping(invoker.MappingInbound, response.Data)
		if err != nil {
			return nil, fmt.Errorf("inbound mapping error: %w", err)
		}
		response.Data = mappedData
	}

	return response, nil
}

// GetWebservice returns a webservice configuration by name.
func (s *Service) GetWebservice(ctx context.Context, name string) (*models.WebserviceConfig, error) {
	return s.getWebserviceByName(ctx, name)
}

// GetWebserviceByID returns a webservice configuration by ID.
func (s *Service) GetWebserviceByID(ctx context.Context, id int) (*models.WebserviceConfig, error) {
	return s.getWebserviceByID(ctx, id)
}

// ListWebservices returns all webservice configurations.
func (s *Service) ListWebservices(ctx context.Context) ([]*models.WebserviceConfig, error) {
	return s.repo.List(ctx)
}

// ListValidWebservices returns only valid (active) webservices.
func (s *Service) ListValidWebservices(ctx context.Context) ([]*models.WebserviceConfig, error) {
	return s.repo.ListValid(ctx)
}

// GetWebservicesForField returns webservices suitable for dynamic field configuration.
func (s *Service) GetWebservicesForField(ctx context.Context) ([]*models.WebserviceConfig, error) {
	return s.repo.GetValidWebservicesForField(ctx)
}

// CreateWebservice creates a new webservice configuration.
func (s *Service) CreateWebservice(ctx context.Context, ws *models.WebserviceConfig, userID int) (int, error) {
	id, err := s.repo.Create(ctx, ws, userID)
	if err != nil {
		return 0, err
	}
	s.invalidateCache()
	return id, nil
}

// UpdateWebservice updates an existing webservice configuration.
func (s *Service) UpdateWebservice(ctx context.Context, ws *models.WebserviceConfig, userID int) error {
	err := s.repo.Update(ctx, ws, userID)
	if err != nil {
		return err
	}
	s.invalidateCache()
	return nil
}

// DeleteWebservice deletes a webservice configuration.
func (s *Service) DeleteWebservice(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	s.invalidateCache()
	return nil
}

// WebserviceExists checks if a webservice with the given name exists.
func (s *Service) WebserviceExists(ctx context.Context, name string) (bool, error) {
	return s.repo.Exists(ctx, name)
}

// WebserviceExistsExcluding checks if a webservice name exists, excluding a specific ID.
func (s *Service) WebserviceExistsExcluding(ctx context.Context, name string, excludeID int) (bool, error) {
	return s.repo.ExistsExcluding(ctx, name, excludeID)
}

// GetHistory returns the configuration history for a webservice.
func (s *Service) GetHistory(ctx context.Context, configID int) ([]*models.WebserviceConfigHistory, error) {
	return s.repo.GetHistory(ctx, configID)
}

// RestoreFromHistory restores a webservice configuration from a history entry.
func (s *Service) RestoreFromHistory(ctx context.Context, historyID int64, userID int) error {
	err := s.repo.RestoreFromHistory(ctx, historyID, userID)
	if err != nil {
		return err
	}
	s.invalidateCache()
	return nil
}

// getWebserviceByName retrieves a webservice by name with caching.
func (s *Service) getWebserviceByName(ctx context.Context, name string) (*models.WebserviceConfig, error) {
	s.cache.mu.RLock()
	if time.Now().Before(s.cache.expiry) {
		if ws, ok := s.cache.configs[name]; ok {
			s.cache.mu.RUnlock()
			return ws, nil
		}
	}
	s.cache.mu.RUnlock()

	// Cache miss - load from database
	ws, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	s.cache.mu.Lock()
	s.cache.configs[name] = ws
	s.cache.configsByID[ws.ID] = ws
	if time.Now().After(s.cache.expiry) {
		s.cache.expiry = time.Now().Add(s.cache.ttl)
	}
	s.cache.mu.Unlock()

	return ws, nil
}

// getWebserviceByID retrieves a webservice by ID with caching.
func (s *Service) getWebserviceByID(ctx context.Context, id int) (*models.WebserviceConfig, error) {
	s.cache.mu.RLock()
	if time.Now().Before(s.cache.expiry) {
		if ws, ok := s.cache.configsByID[id]; ok {
			s.cache.mu.RUnlock()
			return ws, nil
		}
	}
	s.cache.mu.RUnlock()

	// Cache miss - load from database
	ws, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cache.mu.Lock()
	s.cache.configs[ws.Name] = ws
	s.cache.configsByID[ws.ID] = ws
	if time.Now().After(s.cache.expiry) {
		s.cache.expiry = time.Now().Add(s.cache.ttl)
	}
	s.cache.mu.Unlock()

	return ws, nil
}

// invalidateCache clears the webservice cache.
func (s *Service) invalidateCache() {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.configs = make(map[string]*models.WebserviceConfig)
	s.cache.configsByID = make(map[int]*models.WebserviceConfig)
	s.cache.expiry = time.Time{}
}

// applyMapping applies data transformation mapping.
func (s *Service) applyMapping(mapping models.MappingConfig, data map[string]interface{}) (map[string]interface{}, error) {
	switch mapping.Type {
	case "Simple":
		// Simple mapping - pass through with optional key renaming
		return s.applySimpleMapping(mapping, data)
	case "":
		// No mapping - pass through
		return data, nil
	default:
		// Unknown mapping type - log warning and pass through
		if s.debug {
			log.Printf("GenericInterface: Unknown mapping type %q, passing data through", mapping.Type)
		}
		return data, nil
	}
}

// applySimpleMapping applies simple key mapping.
func (s *Service) applySimpleMapping(mapping models.MappingConfig, data map[string]interface{}) (map[string]interface{}, error) {
	if mapping.Config == nil || len(mapping.Config) == 0 {
		return data, nil
	}

	result := make(map[string]interface{})

	// Check if there's a KeyMapDefault for pass-through
	if keyMapDefault, ok := mapping.Config["KeyMapDefault"].(map[string]interface{}); ok {
		if mapTo, ok := keyMapDefault["MapTo"].(string); ok && mapTo == "1" {
			// Copy all keys
			for k, v := range data {
				result[k] = v
			}
		}
	}

	// Apply specific key mappings
	if keyMap, ok := mapping.Config["KeyMap"].(map[string]interface{}); ok {
		for fromKey, toKeyIface := range keyMap {
			if toKey, ok := toKeyIface.(string); ok {
				if val, exists := data[fromKey]; exists {
					result[toKey] = val
				}
			}
		}
	}

	// If no mappings were applied, return original data
	if len(result) == 0 {
		return data, nil
	}

	return result, nil
}
