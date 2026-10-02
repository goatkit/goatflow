package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/data"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/i18n"
)

// LookupService provides lookup data for forms and dropdowns.
type LookupService struct {
	mu        sync.RWMutex
	cache     map[string]*models.TicketFormData // Cache per language
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
	i18n      *i18n.I18n
}

// NewLookupService creates a new lookup service. The database is resolved on
// every cache rebuild, so the service never captures a stale or missing pool.
func NewLookupService() *LookupService {
	return &LookupService{
		cache:     make(map[string]*models.TicketFormData),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  5 * time.Minute, // Cache for 5 minutes
		i18n:      i18n.GetInstance(),
	}
}

// GetTicketFormDataWithLang returns all data needed for ticket forms with translation.
func (s *LookupService) GetTicketFormDataWithLang(lang string) (*models.TicketFormData, error) {
	if lang == "" {
		lang = "en"
	}

	s.mu.RLock()
	if cached, ok := s.cache[lang]; ok && time.Since(s.cacheTime[lang]) < s.cacheTTL {
		defer s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Need write lock to update cache
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := s.cache[lang]; ok && time.Since(s.cacheTime[lang]) < s.cacheTTL {
		return cached, nil
	}

	formData, err := s.buildFormDataWithLang(lang)
	if err != nil {
		return nil, err
	}
	s.cache[lang] = formData
	s.cacheTime[lang] = time.Now()
	return formData, nil
}

// buildFormDataWithLang loads states, priorities, queues and types from the
// database and applies translations.
func (s *LookupService) buildFormDataWithLang(lang string) (*models.TicketFormData, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, fmt.Errorf("lookup data: database unavailable: %w", err)
	}
	if db == nil {
		return nil, fmt.Errorf("lookup data: database unavailable")
	}
	repo := data.NewLookupsRepository(db)
	ctx := context.Background()

	result := &models.TicketFormData{
		Queues:     []models.QueueInfo{},
		Priorities: []models.LookupItem{},
		Statuses:   []models.LookupItem{},
		Types:      []models.LookupItem{},
	}

	states, err := repo.GetTicketStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("lookup data: ticket states: %w", err)
	}
	for i, state := range states {
		result.Statuses = append(result.Statuses, models.LookupItem{
			ID:     state.ID,
			Value:  state.Name,
			Label:  s.getTranslation("ticket_states", state.Name, lang),
			Order:  i + 1,
			Active: state.ValidID == 1,
		})
	}

	priorities, err := repo.GetTicketPriorities(ctx)
	if err != nil {
		return nil, fmt.Errorf("lookup data: ticket priorities: %w", err)
	}
	for i, priority := range priorities {
		result.Priorities = append(result.Priorities, models.LookupItem{
			ID:     priority.ID,
			Value:  priority.Name,
			Label:  s.getTranslation("ticket_priorities", priority.Name, lang),
			Order:  i + 1,
			Active: priority.ValidID == 1,
		})
	}

	queues, err := repo.GetQueues(ctx)
	if err != nil {
		return nil, fmt.Errorf("lookup data: queues: %w", err)
	}
	for _, queue := range queues {
		result.Queues = append(result.Queues, models.QueueInfo{
			ID:     queue.ID,
			Name:   s.getTranslation("queues", queue.Name, lang),
			Active: queue.ValidID == 1,
		})
	}

	// Ticket types: ticket_type names are admin-defined and shown verbatim.
	types, err := repo.GetTicketTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("lookup data: ticket types: %w", err)
	}
	for i, typ := range types {
		result.Types = append(result.Types, models.LookupItem{
			ID:     typ.ID,
			Value:  typ.Name,
			Label:  typ.Name,
			Order:  i + 1,
			Active: typ.ValidID == 1,
		})
	}

	return result, nil
}

// InvalidateCache forces a cache refresh on next request.
func (s *LookupService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*models.TicketFormData)
	s.cacheTime = make(map[string]time.Time)
}

// getTranslation translates well-known OTRS state and priority names through
// the i18n catalogue (status.<name>, priority.<name>); anything else, including
// admin-defined names, is returned unchanged.
func (s *LookupService) getTranslation(tableName, fieldValue, lang string) string {
	if s.i18n != nil {
		if tableName == "ticket_states" {
			if trans := s.i18n.T(lang, "status."+fieldValue); trans != "status."+fieldValue {
				return trans
			}
		} else if tableName == "ticket_priorities" {
			// Handle OTRS format "3 normal" -> try "normal"
			simplified := fieldValue
			if len(fieldValue) > 2 && fieldValue[1] == ' ' {
				simplified = fieldValue[2:]
			}
			if trans := s.i18n.T(lang, "priority."+simplified); trans != "priority."+simplified {
				return trans
			}
			// Fallback mapping for extremes when translations are missing
			switch simplified {
			case "very low":
				if lang == "de" {
					return "Sehr niedrig"
				}
				return "Very Low"
			case "very high":
				if lang == "de" {
					return "Sehr hoch"
				}
				return "Very High"
			}
		}
	}

	return fieldValue
}
