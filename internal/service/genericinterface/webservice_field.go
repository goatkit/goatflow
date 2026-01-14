package genericinterface

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// WebserviceFieldService handles autocomplete and display value retrieval for
// WebserviceDropdown and WebserviceMultiselect dynamic fields.
type WebserviceFieldService struct {
	giService *Service
	cache     *fieldCache
}

// fieldCache provides caching for webservice field results.
type fieldCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	data    []AutocompleteResult
	expiry  time.Time
}

// AutocompleteResult represents a single autocomplete suggestion.
// JSON field names match OTRS's expected format for compatibility.
type AutocompleteResult struct {
	StoredValue  string                 `json:"StoredValue"`            // The stored value (e.g., ID) - OTRS compatible
	DisplayValue string                 `json:"DisplayValue"`           // The displayed text - OTRS compatible
	Data         map[string]interface{} `json:"Data,omitempty"`         // Additional data for autofill
	// Legacy aliases for frontend compatibility (same values, different keys)
	Value string `json:"value,omitempty"` // Alias for StoredValue
	Label string `json:"label,omitempty"` // Alias for DisplayValue
}

// FieldConfig holds the configuration needed to query a webservice field.
type FieldConfig struct {
	Webservice               string
	InvokerSearch            string
	InvokerGet               string
	StoredValue              string
	DisplayedValues          []string
	DisplayedValuesSeparator string
	SearchKeys               []string
	AutocompleteMinLength    int
	Limit                    int
	CacheTTL                 int
}

// NewWebserviceFieldService creates a new webservice field service.
func NewWebserviceFieldService(db *sql.DB) *WebserviceFieldService {
	return &WebserviceFieldService{
		giService: NewService(db),
		cache: &fieldCache{
			entries: make(map[string]*cacheEntry),
			ttl:     60 * time.Second,
		},
	}
}

// NewWebserviceFieldServiceWithGI creates a service using an existing GI service.
func NewWebserviceFieldServiceWithGI(giService *Service) *WebserviceFieldService {
	return &WebserviceFieldService{
		giService: giService,
		cache: &fieldCache{
			entries: make(map[string]*cacheEntry),
			ttl:     60 * time.Second,
		},
	}
}

// Search performs a search/autocomplete query against the configured webservice.
func (s *WebserviceFieldService) Search(ctx context.Context, config FieldConfig, searchTerm string) ([]AutocompleteResult, error) {
	// Check minimum length
	if len(searchTerm) < config.AutocompleteMinLength {
		return nil, nil
	}

	// Check cache
	cacheKey := s.buildCacheKey(config.Webservice, config.InvokerSearch, searchTerm)
	if cached := s.getFromCache(cacheKey); cached != nil {
		return cached, nil
	}

	// Build request data
	requestData := map[string]interface{}{
		"SearchTerms": searchTerm,
		"Limit":       config.Limit,
	}

	// Execute webservice call
	response, err := s.giService.Invoke(ctx, config.Webservice, config.InvokerSearch, requestData)
	if err != nil {
		return nil, fmt.Errorf("webservice search failed: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("webservice returned error: %s", response.Error)
	}

	// Parse results
	results := s.parseSearchResults(response.Data, config)

	// Cache results
	s.addToCache(cacheKey, results, time.Duration(config.CacheTTL)*time.Second)

	return results, nil
}

// GetDisplayValue retrieves the display value for a stored value.
// This is used to show the human-readable label when loading existing data.
func (s *WebserviceFieldService) GetDisplayValue(ctx context.Context, config FieldConfig, storedValue string) (string, error) {
	if storedValue == "" {
		return "", nil
	}

	// If no InvokerGet, we can't retrieve display value
	if config.InvokerGet == "" {
		return storedValue, nil
	}

	// Build request data
	requestData := map[string]interface{}{
		config.StoredValue: storedValue,
	}

	// Execute webservice call
	response, err := s.giService.Invoke(ctx, config.Webservice, config.InvokerGet, requestData)
	if err != nil {
		return storedValue, nil // Return stored value on error
	}

	if !response.Success || response.Data == nil {
		return storedValue, nil
	}

	// Build display value from response
	return s.buildDisplayValue(response.Data, config), nil
}

// GetMultipleDisplayValues retrieves display values for multiple stored values.
// Used for WebserviceMultiselect fields.
func (s *WebserviceFieldService) GetMultipleDisplayValues(ctx context.Context, config FieldConfig, storedValues []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, value := range storedValues {
		displayValue, err := s.GetDisplayValue(ctx, config, value)
		if err != nil {
			result[value] = value // Fallback to stored value
		} else {
			result[value] = displayValue
		}
	}

	return result, nil
}

// parseSearchResults converts webservice response to autocomplete results.
func (s *WebserviceFieldService) parseSearchResults(data map[string]interface{}, config FieldConfig) []AutocompleteResult {
	var results []AutocompleteResult

	// Response could be an array at "items" key or at root level
	var items []interface{}

	if itemsArray, ok := data["items"].([]interface{}); ok {
		items = itemsArray
	} else if itemsArray, ok := data["Items"].([]interface{}); ok {
		items = itemsArray
	} else if itemsArray, ok := data["results"].([]interface{}); ok {
		items = itemsArray
	} else if itemsArray, ok := data["Results"].([]interface{}); ok {
		items = itemsArray
	} else if itemsArray, ok := data["data"].([]interface{}); ok {
		items = itemsArray
	} else if itemsArray, ok := data["Data"].([]interface{}); ok {
		items = itemsArray
	} else {
		// Try treating the entire response as a single item
		if len(data) > 0 {
			items = []interface{}{data}
		}
	}

	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		result := AutocompleteResult{
			Data: itemMap,
		}

		// Extract stored value
		if val, ok := itemMap[config.StoredValue]; ok {
			storedVal := fmt.Sprintf("%v", val)
			result.StoredValue = storedVal
			result.Value = storedVal // Legacy alias
		}

		// Build display label
		displayVal := s.buildDisplayValue(itemMap, config)
		result.DisplayValue = displayVal
		result.Label = displayVal // Legacy alias

		if result.StoredValue != "" {
			results = append(results, result)
		}
	}

	// Apply limit
	if config.Limit > 0 && len(results) > config.Limit {
		results = results[:config.Limit]
	}

	return results
}

// buildDisplayValue creates the display string from response data.
func (s *WebserviceFieldService) buildDisplayValue(data map[string]interface{}, config FieldConfig) string {
	var parts []string

	for _, field := range config.DisplayedValues {
		if val, ok := data[field]; ok && val != nil {
			valStr := fmt.Sprintf("%v", val)
			if valStr != "" {
				parts = append(parts, valStr)
			}
		}
	}

	separator := config.DisplayedValuesSeparator
	if separator == "" {
		separator = " - "
	}

	return strings.Join(parts, separator)
}

// buildCacheKey creates a unique cache key for a search request.
func (s *WebserviceFieldService) buildCacheKey(webservice, invoker, term string) string {
	return fmt.Sprintf("%s:%s:%s", webservice, invoker, strings.ToLower(term))
}

// getFromCache retrieves cached results if still valid.
func (s *WebserviceFieldService) getFromCache(key string) []AutocompleteResult {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	entry, ok := s.cache.entries[key]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiry) {
		return nil
	}

	return entry.data
}

// addToCache stores results in cache.
func (s *WebserviceFieldService) addToCache(key string, data []AutocompleteResult, ttl time.Duration) {
	if ttl == 0 {
		ttl = s.cache.ttl
	}

	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	s.cache.entries[key] = &cacheEntry{
		data:   data,
		expiry: time.Now().Add(ttl),
	}
}

// ClearCache clears the cache for a specific webservice or all caches.
func (s *WebserviceFieldService) ClearCache(webservice string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	if webservice == "" {
		s.cache.entries = make(map[string]*cacheEntry)
		return
	}

	prefix := webservice + ":"
	for key := range s.cache.entries {
		if strings.HasPrefix(key, prefix) {
			delete(s.cache.entries, key)
		}
	}
}

// ParseFieldConfigFromMap parses a DynamicFieldConfig-like map into FieldConfig.
func ParseFieldConfigFromMap(configMap map[string]interface{}) FieldConfig {
	config := FieldConfig{
		AutocompleteMinLength:    3,
		Limit:                    20,
		CacheTTL:                 60,
		DisplayedValuesSeparator: " - ",
	}

	if ws, ok := configMap["Webservice"].(string); ok {
		config.Webservice = ws
	}
	if inv, ok := configMap["InvokerSearch"].(string); ok {
		config.InvokerSearch = inv
	}
	if inv, ok := configMap["InvokerGet"].(string); ok {
		config.InvokerGet = inv
	}
	if sv, ok := configMap["StoredValue"].(string); ok {
		config.StoredValue = sv
	}
	if dv, ok := configMap["DisplayedValues"].(string); ok {
		config.DisplayedValues = strings.Split(dv, ",")
		for i := range config.DisplayedValues {
			config.DisplayedValues[i] = strings.TrimSpace(config.DisplayedValues[i])
		}
	}
	if sep, ok := configMap["DisplayedValuesSeparator"].(string); ok && sep != "" {
		config.DisplayedValuesSeparator = sep
	}
	if sk, ok := configMap["SearchKeys"].(string); ok {
		config.SearchKeys = strings.Split(sk, ",")
		for i := range config.SearchKeys {
			config.SearchKeys[i] = strings.TrimSpace(config.SearchKeys[i])
		}
	}
	if min, ok := configMap["AutocompleteMinLength"].(int); ok && min > 0 {
		config.AutocompleteMinLength = min
	}
	if limit, ok := configMap["Limit"].(int); ok && limit > 0 {
		config.Limit = limit
	}
	if ttl, ok := configMap["CacheTTL"].(int); ok && ttl > 0 {
		config.CacheTTL = ttl
	}

	return config
}
