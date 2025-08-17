package service

import (
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// LookupService provides lookup data for forms and dropdowns
type LookupService struct {
	mu         sync.RWMutex
	cache      *models.TicketFormData
	cacheTime  time.Time
	cacheTTL   time.Duration
}

// NewLookupService creates a new lookup service
func NewLookupService() *LookupService {
	return &LookupService{
		cacheTTL: 5 * time.Minute, // Cache for 5 minutes
	}
}

// GetTicketFormData returns all data needed for ticket forms
func (s *LookupService) GetTicketFormData() *models.TicketFormData {
	s.mu.RLock()
	if s.cache != nil && time.Since(s.cacheTime) < s.cacheTTL {
		defer s.mu.RUnlock()
		return s.cache
	}
	s.mu.RUnlock()

	// Need write lock to update cache
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.cache != nil && time.Since(s.cacheTime) < s.cacheTTL {
		return s.cache
	}

	// Build fresh data
	s.cache = s.buildFormData()
	s.cacheTime = time.Now()
	return s.cache
}

// buildFormData creates the form data structure
func (s *LookupService) buildFormData() *models.TicketFormData {
	// In production, these would come from database queries
	// For now, we'll return the standard OTRS-compatible values
	
	return &models.TicketFormData{
		Queues: []models.QueueInfo{
			{ID: 1, Name: "General Support", Description: "General customer inquiries", Active: true},
			{ID: 2, Name: "Technical Issues", Description: "Technical problems and bugs", Active: true},
			{ID: 3, Name: "Billing", Description: "Billing and payment issues", Active: true},
			{ID: 4, Name: "Feature Requests", Description: "New feature suggestions", Active: true},
			{ID: 5, Name: "Sales", Description: "Sales inquiries", Active: true},
			{ID: 6, Name: "Documentation", Description: "Documentation requests", Active: true},
		},
		Priorities: []models.LookupItem{
			{ID: 1, Value: "low", Label: "Low", Order: 1, Active: true},
			{ID: 2, Value: "normal", Label: "Normal", Order: 2, Active: true},
			{ID: 3, Value: "high", Label: "High", Order: 3, Active: true},
			{ID: 4, Value: "urgent", Label: "Urgent", Order: 4, Active: true},
		},
		Types: []models.LookupItem{
			{ID: 1, Value: "incident", Label: "Incident", Order: 1, Active: true},
			{ID: 2, Value: "service_request", Label: "Service Request", Order: 2, Active: true},
			{ID: 3, Value: "change_request", Label: "Change Request", Order: 3, Active: true},
			{ID: 4, Value: "problem", Label: "Problem", Order: 4, Active: true},
			{ID: 5, Value: "question", Label: "Question", Order: 5, Active: true},
		},
		Statuses: []models.LookupItem{
			{ID: 1, Value: "new", Label: "New", Order: 1, Active: true},
			{ID: 2, Value: "open", Label: "Open", Order: 2, Active: true},
			{ID: 3, Value: "pending", Label: "Pending", Order: 3, Active: true},
			{ID: 4, Value: "resolved", Label: "Resolved", Order: 4, Active: true},
			{ID: 5, Value: "closed", Label: "Closed", Order: 5, Active: true},
		},
	}
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
	s.cache = nil
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