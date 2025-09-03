package service

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/data"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/i18n"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// LookupService provides lookup data for forms and dropdowns
type LookupService struct {
	mu         sync.RWMutex
	cache      map[string]*models.TicketFormData // Cache per language
	cacheTime  map[string]time.Time
	cacheTTL   time.Duration
	db         *sql.DB
	repo       *data.LookupsRepository
	i18n       *i18n.I18n
}

// NewLookupService creates a new lookup service
func NewLookupService() *LookupService {
	s := &LookupService{
		cache:     make(map[string]*models.TicketFormData),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  5 * time.Minute, // Cache for 5 minutes
		i18n:      i18n.GetInstance(),
	}
	
    // Try to connect to database
    db, err := database.GetDB()
    if err != nil || db == nil {
        if err != nil {
            log.Printf("Warning: Lookup service running without database: %v", err)
        } else {
            log.Printf("Warning: Lookup service running without database: nil connection")
        }
    } else {
        s.db = db
        s.repo = data.NewLookupsRepository(db)
        log.Printf("LookupService: Successfully connected to database")
    }
	
	return s
}

// GetTicketFormData returns all data needed for ticket forms (defaults to English)
func (s *LookupService) GetTicketFormData() *models.TicketFormData {
	return s.GetTicketFormDataWithLang("en")
}

// GetTicketFormDataWithLang returns all data needed for ticket forms with translation
func (s *LookupService) GetTicketFormDataWithLang(lang string) *models.TicketFormData {
	if lang == "" {
		lang = "en"
	}
	
	s.mu.RLock()
	if cached, ok := s.cache[lang]; ok && time.Since(s.cacheTime[lang]) < s.cacheTTL {
		defer s.mu.RUnlock()
		return cached
	}
	s.mu.RUnlock()

	// Need write lock to update cache
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := s.cache[lang]; ok && time.Since(s.cacheTime[lang]) < s.cacheTTL {
		return cached
	}

	// Build fresh data with translations
	s.cache[lang] = s.buildFormDataWithLang(lang)
	s.cacheTime[lang] = time.Now()
	return s.cache[lang]
}

// buildFormData creates the form data structure (backward compatibility)
func (s *LookupService) buildFormData() *models.TicketFormData {
	return s.buildFormDataWithLang("en")
}

// buildFormDataWithLang creates form data with translations
func (s *LookupService) buildFormDataWithLang(lang string) *models.TicketFormData {
	// Initialize with defaults that will be overridden if database is available
	result := &models.TicketFormData{
		Queues:     []models.QueueInfo{},
		Priorities: []models.LookupItem{},
		Statuses:   []models.LookupItem{},
		Types:      s.getDefaultTypes(lang),
	}
	
	// If we have database connection, get values from there
    if s.repo != nil {
		ctx := context.Background()
		log.Printf("LookupService: Fetching data from database for language: %s", lang)
		
		// Get states from database
		states, err := s.repo.GetTicketStates(ctx)
		if err == nil && len(states) > 0 {
			statuses := make([]models.LookupItem, len(states))
			for i, state := range states {
				label := s.getTranslation("ticket_states", state.Name, lang)
				statuses[i] = models.LookupItem{
					ID:     state.ID,
					Value:  state.Name,
					Label:  label,
					Order:  i + 1,
					Active: state.ValidID == 1,
				}
			}
			result.Statuses = statuses
			log.Printf("LookupService: Got %d states from database", len(statuses))
		}
		
		// Get priorities from database
		priorities, err := s.repo.GetTicketPriorities(ctx)
		if err == nil && len(priorities) > 0 {
			priorityItems := make([]models.LookupItem, len(priorities))
			for i, priority := range priorities {
				label := s.getTranslation("ticket_priorities", priority.Name, lang)
				priorityItems[i] = models.LookupItem{
					ID:     priority.ID,
					Value:  priority.Name,
					Label:  label,
					Order:  i + 1,
					Active: priority.ValidID == 1,
				}
			}
			result.Priorities = priorityItems
			log.Printf("LookupService: Got %d priorities from database", len(priorityItems))
		}
		
		// Get queues from database  
		queues, err := s.repo.GetQueues(ctx)
		if err == nil && len(queues) > 0 {
			queueItems := make([]models.QueueInfo, len(queues))
			for i, queue := range queues {
				// Apply translation to queue names
				translatedName := s.getTranslation("queues", queue.Name, lang)
				// Note: Queue descriptions would need to be fetched separately if needed
				// For now, just use empty description since LookupItem doesn't have Comment field
				queueItems[i] = models.QueueInfo{
					ID:          queue.ID,
					Name:        translatedName,
					Description: "",
					Active:      queue.ValidID == 1,
				}
			}
			result.Queues = queueItems
			log.Printf("LookupService: Got %d queues from database", len(queueItems))
		}
		
		// Return data from database
		return result
	}
	
    // Graceful fallback when DB unavailable: empty lists but valid structure
    log.Printf("WARNING: No lookup data available from database; returning empty defaults")
    return result
}

// GetQueues returns available queues
func (s *LookupService) GetQueues() []models.QueueInfo {
	data := s.GetTicketFormData()
	return data.Queues
}

// GetPriorities returns available priorities
func (s *LookupService) GetPriorities() []models.LookupItem {
	data := s.GetTicketFormData()
	return data.Priorities
}

// GetTypes returns available ticket types
func (s *LookupService) GetTypes() []models.LookupItem {
	data := s.GetTicketFormData()
	return data.Types
}

// GetStatuses returns available ticket statuses
func (s *LookupService) GetStatuses() []models.LookupItem {
	data := s.GetTicketFormData()
	return data.Statuses
}

// InvalidateCache forces a cache refresh on next request
func (s *LookupService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*models.TicketFormData)
	s.cacheTime = make(map[string]time.Time)
}

// GetQueueByID returns a specific queue by ID
func (s *LookupService) GetQueueByID(id int) (*models.QueueInfo, bool) {
	queues := s.GetQueues()
	for _, q := range queues {
		if q.ID == id {
			return &q, true
		}
	}
	return nil, false
}

// GetPriorityByValue returns a priority by its value
func (s *LookupService) GetPriorityByValue(value string) (*models.LookupItem, bool) {
	priorities := s.GetPriorities()
	for _, p := range priorities {
		if p.Value == value {
			return &p, true
		}
	}
	return nil, false
}

// GetTypeByID returns a ticket type by ID
func (s *LookupService) GetTypeByID(id int) (*models.LookupItem, bool) {
	types := s.GetTypes()
	for _, t := range types {
		if t.ID == id {
			return &t, true
		}
	}
	return nil, false
}

// GetStatusByValue returns a status by its value
func (s *LookupService) GetStatusByValue(value string) (*models.LookupItem, bool) {
	statuses := s.GetStatuses()
	for _, s := range statuses {
		if s.Value == value {
			return &s, true
		}
	}
	return nil, false
}

// getTranslation gets a translation for a lookup value
func (s *LookupService) getTranslation(tableName, fieldValue, lang string) string {
	// First try database translation if available
	if s.repo != nil {
		if translation, err := s.repo.GetTranslation(context.Background(), tableName, fieldValue, lang); err == nil && translation != "" {
			return translation
		}
	}
	
	// Fallback to i18n files for common values
	// This is just a fallback - primary translations should be in DB
	if s.i18n != nil {
		// Try standard keys
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
		}
	}
	
	// If no translation found, return original value
	return fieldValue
}

// getDefaultTypes returns default ticket types with translations
func (s *LookupService) getDefaultTypes(lang string) []models.LookupItem {
	types := []models.LookupItem{
		{ID: 1, Value: "incident", Label: "Incident", Order: 1, Active: true},
		{ID: 2, Value: "service_request", Label: "Service Request", Order: 2, Active: true},
		{ID: 3, Value: "change_request", Label: "Change Request", Order: 3, Active: true},
		{ID: 4, Value: "problem", Label: "Problem", Order: 4, Active: true},
		{ID: 5, Value: "question", Label: "Question", Order: 5, Active: true},
	}
	
	// Apply translations if available
	for i := range types {
		if s.i18n != nil {
			if trans := s.i18n.T(lang, "tickets.type_"+types[i].Value); trans != "tickets.type_"+types[i].Value {
				types[i].Label = trans
			}
		}
	}
	
	return types
}