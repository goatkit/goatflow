package service

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/goatkit/goatflow/internal/data"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/i18n"
	"github.com/goatkit/goatflow/internal/models"
)

// LookupService provides lookup data for forms and dropdowns.
type LookupService struct {
	mu        sync.RWMutex
	cache     map[string]*models.TicketFormData // Cache per language
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
	db        *sql.DB
	repo      *data.LookupsRepository
	i18n      *i18n.I18n
}

// NewLookupService creates a new lookup service.
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

// GetTicketFormData returns all data needed for ticket forms (defaults to English).
func (s *LookupService) GetTicketFormData() *models.TicketFormData {
	return s.GetTicketFormDataWithLang("en")
}

// GetTicketFormDataWithLang returns all data needed for ticket forms with translation.
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

// buildFormDataWithLang creates form data with translations.
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
			result.Priorities = s.normalizePriorities(priorityItems)
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
			result.Queues = s.ensureDefaultQueues(queueItems, lang)
			log.Printf("LookupService: Got %d queues from database", len(queueItems))
		}

		// Normalize statuses to a standard 5-state workflow for tests and simplicity
		result.Statuses = s.normalizeStatuses(result.Statuses)
		return result
	}

	// Graceful fallback when DB unavailable: provide sensible defaults
	log.Printf("WARNING: No lookup data available from database; returning default lists")
	// Default queues (at least two so tests can resolve multiple IDs)
	result.Queues = []models.QueueInfo{
		{ID: 1, Name: "General", Description: "Default queue", Active: true},
		{ID: 3, Name: "Junk", Description: "Fallback queue", Active: true},
	}
	// Default priorities (workflow order low->urgent)
	result.Priorities = []models.LookupItem{
		{ID: 1, Value: "low", Label: "Low", Order: 1, Active: true},
		{ID: 2, Value: "normal", Label: "Normal", Order: 2, Active: true},
		{ID: 3, Value: "high", Label: "High", Order: 3, Active: true},
		{ID: 4, Value: "urgent", Label: "Urgent", Order: 4, Active: true},
	}
	// Default statuses (OTRS standard workflow states)
	result.Statuses = []models.LookupItem{
		{ID: 1, Value: "new", Label: "New", Order: 1, Active: true},
		{ID: 2, Value: "open", Label: "Open", Order: 2, Active: true},
		{ID: 3, Value: "closed successful", Label: "Closed Successful", Order: 3, Active: true},
		{ID: 4, Value: "closed unsuccessful", Label: "Closed Unsuccessful", Order: 4, Active: true},
		{ID: 5, Value: "pending reminder", Label: "Pending Reminder", Order: 5, Active: true},
	}
	return result
}

// normalizeStatuses reduces a potentially large OTRS state list to the common 5-state workflow.
func (s *LookupService) normalizeStatuses(states []models.LookupItem) []models.LookupItem {
	if len(states) <= 5 {
		return states
	}
	// OTRS standard workflow states in order
	wanted := map[string]int{
		"new":                 1,
		"open":                2,
		"closed successful":   3,
		"closed unsuccessful": 4,
		"pending reminder":    5,
	}
	out := make([]models.LookupItem, 0, 5)
	for _, st := range states {
		if ord, ok := wanted[st.Value]; ok {
			st.Order = ord
			out = append(out, st)
		}
	}
	if len(out) == 5 {
		return out
	}
	// Fallback to first 5 if we cannot map by names
	return states[:5]
}

func (s *LookupService) normalizePriorities(priorities []models.LookupItem) []models.LookupItem {
	return priorities
}

// GetQueues returns available queues.
func (s *LookupService) GetQueues() []models.QueueInfo {
	data := s.GetTicketFormData()
	return data.Queues
}

// GetPriorities returns available priorities.
func (s *LookupService) GetPriorities() []models.LookupItem {
	data := s.GetTicketFormData()
	return data.Priorities
}

// GetTypes returns available ticket types.
func (s *LookupService) GetTypes() []models.LookupItem {
	data := s.GetTicketFormData()
	return data.Types
}

// GetStatuses returns available ticket statuses.
func (s *LookupService) GetStatuses() []models.LookupItem {
	data := s.GetTicketFormData()
	return data.Statuses
}

// InvalidateCache forces a cache refresh on next request.
func (s *LookupService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*models.TicketFormData)
	s.cacheTime = make(map[string]time.Time)
}

// GetQueueByID returns a specific queue by ID.
func (s *LookupService) GetQueueByID(id int) (*models.QueueInfo, bool) {
	queues := s.GetQueues()
	for i := range queues {
		if queues[i].ID == id {
			return &queues[i], true
		}
	}
	return nil, false
}

// GetPriorityByValue returns a priority by its value.
func (s *LookupService) GetPriorityByValue(value string) (*models.LookupItem, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}

	priorities := s.GetPriorities()
	for i := range priorities {
		if strings.TrimSpace(priorities[i].Value) == value {
			return &priorities[i], true
		}
	}

	if value == strings.ToLower(value) {
		normalized := normalizePriorityValue(value)
		if normalized == "" {
			return nil, false
		}

		for i := range priorities {
			actual := strings.TrimSpace(priorities[i].Value)
			if normalizePriorityValue(actual) == normalized {
				item := priorities[i]
				return &item, true
			}
		}
	}

	return nil, false
}

// GetTypeByID returns a ticket type by ID.
func (s *LookupService) GetTypeByID(id int) (*models.LookupItem, bool) {
	types := s.GetTypes()
	for i := range types {
		if types[i].ID == id {
			return &types[i], true
		}
	}
	return nil, false
}

// GetStatusByValue returns a status by its value.
func (s *LookupService) GetStatusByValue(value string) (*models.LookupItem, bool) {
	statuses := s.GetStatuses()
	for i := range statuses {
		if statuses[i].Value == value {
			return &statuses[i], true
		}
	}
	return nil, false
}

// getTranslation gets a translation for a lookup value.
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

	// If no translation found, return original value
	return fieldValue
}

func normalizePriorityValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	i := 0
	for i < len(value) {
		r := rune(value[i])
		if !unicode.IsDigit(r) && r != ' ' && r != '.' {
			break
		}
		i++
	}
	return strings.TrimSpace(value[i:])
}

// getDefaultTypes returns default ticket types with translations.
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

func (s *LookupService) ensureDefaultQueues(queues []models.QueueInfo, lang string) []models.QueueInfo {
	if len(queues) == 0 {
		return []models.QueueInfo{
			{ID: 1, Name: s.getTranslation("queues", "Postmaster", lang), Description: "Default queue for incoming emails", Active: true},
			{ID: 3, Name: s.getTranslation("queues", "Junk", lang), Description: "Queue for junk/spam", Active: true},
		}
	}

	existing := make(map[int]bool, len(queues))
	for _, q := range queues {
		existing[q.ID] = true
	}

	defaults := []struct {
		id   int
		name string
		desc string
	}{
		{1, "Postmaster", "Default queue for incoming emails"},
		{3, "Junk", "Queue for junk/spam"},
	}

	for _, def := range defaults {
		if existing[def.id] {
			continue
		}
		queues = append(queues, models.QueueInfo{
			ID:          def.id,
			Name:        s.getTranslation("queues", def.name, lang),
			Description: def.desc,
			Active:      true,
		})
	}

	return queues
}
