package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/data"
	"github.com/gotrs-io/gotrs-ce/internal/i18n"
)

// LookupOption represents a single option in a dropdown/select field.
type LookupOption struct {
	ID          int    `json:"id"`
	Value       string `json:"value"`        // Internal database value
	DisplayText string `json:"display_text"` // Translated display text
	IsSystem    bool   `json:"is_system"`    // Whether this is a system-defined value
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
}

// LookupService provides translated lookup values for dropdowns.
type LookupService struct {
	repo  *data.LookupsRepository
	i18n  *i18n.I18n
	cache map[string][]LookupOption
	mu    sync.RWMutex
}

// NewLookupService creates a new lookup service.
func NewLookupService(repo *data.LookupsRepository) *LookupService {
	return &LookupService{
		repo:  repo,
		i18n:  i18n.GetInstance(),
		cache: make(map[string][]LookupOption),
	}
}

// GetTicketStates returns all available ticket states with translations.
func (s *LookupService) GetTicketStates(ctx context.Context, lang string) ([]LookupOption, error) {
	cacheKey := fmt.Sprintf("ticket_states_%s", lang)

	// Check cache
	s.mu.RLock()
	if cached, ok := s.cache[cacheKey]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Fetch from database
	states, err := s.repo.GetTicketStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket states: %w", err)
	}

	// Apply translations
	options := make([]LookupOption, len(states))
	for i, state := range states {
		options[i] = LookupOption{
			ID:          state.ID,
			Value:       state.Name,
			DisplayText: s.getTranslation("ticket_states", state.Name, lang),
			IsSystem:    state.IsSystem,
			SortOrder:   i,
			IsActive:    state.ValidID == 1,
		}
	}

	// Cache the result
	s.mu.Lock()
	s.cache[cacheKey] = options
	s.mu.Unlock()

	// Expire cache after 5 minutes
	go func() {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		delete(s.cache, cacheKey)
		s.mu.Unlock()
	}()

	return options, nil
}

// GetTicketPriorities returns all available ticket priorities with translations.
func (s *LookupService) GetTicketPriorities(ctx context.Context, lang string) ([]LookupOption, error) {
	cacheKey := fmt.Sprintf("ticket_priorities_%s", lang)

	// Check cache
	s.mu.RLock()
	if cached, ok := s.cache[cacheKey]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Fetch from database
	priorities, err := s.repo.GetTicketPriorities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket priorities: %w", err)
	}

	// Apply translations
	options := make([]LookupOption, len(priorities))
	for i, priority := range priorities {
		options[i] = LookupOption{
			ID:          priority.ID,
			Value:       priority.Name,
			DisplayText: s.getTranslation("ticket_priorities", priority.Name, lang),
			IsSystem:    priority.IsSystem,
			SortOrder:   i,
			IsActive:    priority.ValidID == 1,
		}
	}

	// Cache the result
	s.mu.Lock()
	s.cache[cacheKey] = options
	s.mu.Unlock()

	// Expire cache after 5 minutes
	go func() {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		delete(s.cache, cacheKey)
		s.mu.Unlock()
	}()

	return options, nil
}

// GetQueues returns all available queues with optional translations.
func (s *LookupService) GetQueues(ctx context.Context, lang string) ([]LookupOption, error) {
	cacheKey := fmt.Sprintf("queues_%s", lang)

	// Check cache
	s.mu.RLock()
	if cached, ok := s.cache[cacheKey]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Fetch from database
	queues, err := s.repo.GetQueues(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queues: %w", err)
	}

	// Apply translations (queues might not always be translated)
	options := make([]LookupOption, len(queues))
	for i, queue := range queues {
		displayText := s.getTranslation("queues", queue.Name, lang)
		// If no translation, use the queue name as-is
		if displayText == queue.Name {
			displayText = queue.Name
		}

		options[i] = LookupOption{
			ID:          queue.ID,
			Value:       queue.Name,
			DisplayText: displayText,
			IsSystem:    queue.IsSystem,
			SortOrder:   i,
			IsActive:    queue.ValidID == 1,
		}
	}

	// Cache the result
	s.mu.Lock()
	s.cache[cacheKey] = options
	s.mu.Unlock()

	// Expire cache after 5 minutes
	go func() {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		delete(s.cache, cacheKey)
		s.mu.Unlock()
	}()

	return options, nil
}

// GetTicketTypes returns all available ticket types with translations.
func (s *LookupService) GetTicketTypes(ctx context.Context, lang string) ([]LookupOption, error) {
	cacheKey := fmt.Sprintf("ticket_types_%s", lang)

	// Check cache
	s.mu.RLock()
	if cached, ok := s.cache[cacheKey]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Fetch from database
	types, err := s.repo.GetTicketTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket types: %w", err)
	}

	// Apply translations
	options := make([]LookupOption, len(types))
	for i, ticketType := range types {
		options[i] = LookupOption{
			ID:          ticketType.ID,
			Value:       ticketType.Name,
			DisplayText: s.getTranslation("ticket_types", ticketType.Name, lang),
			IsSystem:    ticketType.IsSystem,
			SortOrder:   i,
			IsActive:    ticketType.ValidID == 1,
		}
	}

	// Cache the result
	s.mu.Lock()
	s.cache[cacheKey] = options
	s.mu.Unlock()

	// Expire cache after 5 minutes
	go func() {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		delete(s.cache, cacheKey)
		s.mu.Unlock()
	}()

	return options, nil
}

// getTranslation attempts to get a translation from the database or i18n files.
func (s *LookupService) getTranslation(tableName, fieldValue, lang string) string {
	// First, try to get translation from database
	if translation, err := s.repo.GetTranslation(context.Background(), tableName, fieldValue, lang); err == nil && translation != "" {
		return translation
	}

	// Fallback to i18n file translations for common values
	// This allows for quick setup without database entries
	switch tableName {
	case "ticket_states":
		// Try i18n key like "status.new", "status.open", etc.
		if translated := s.i18n.T(lang, fmt.Sprintf("status.%s", fieldValue)); translated != fmt.Sprintf("status.%s", fieldValue) {
			return translated
		}
	case "ticket_priorities":
		// Try i18n key like "priority.low", "priority.high", etc.
		// Handle OTRS format like "3 normal" -> "normal"
		simplifiedValue := fieldValue
		if len(fieldValue) > 2 && fieldValue[1] == ' ' {
			simplifiedValue = fieldValue[2:]
		}
		if translated := s.i18n.T(lang, fmt.Sprintf("priority.%s", simplifiedValue)); translated != fmt.Sprintf("priority.%s", simplifiedValue) {
			return translated
		}
	}

	// If no translation found, return the original value
	// This ensures OTRS compatibility for custom values
	return fieldValue
}

// ClearCache clears the lookup cache.
func (s *LookupService) ClearCache() {
	s.mu.Lock()
	s.cache = make(map[string][]LookupOption)
	s.mu.Unlock()
}

// AddCustomTranslation allows admins to add translations for custom values.
func (s *LookupService) AddCustomTranslation(ctx context.Context, tableName, fieldValue, lang, translation string) error {
	err := s.repo.AddTranslation(ctx, tableName, fieldValue, lang, translation, false)
	if err != nil {
		return fmt.Errorf("failed to add custom translation: %w", err)
	}

	// Clear cache to reflect changes
	s.ClearCache()

	return nil
}
