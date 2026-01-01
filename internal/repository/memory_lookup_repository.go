package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// MemoryLookupRepository is an in-memory implementation of LookupRepository.
type MemoryLookupRepository struct {
	mu         sync.RWMutex
	queues     map[int]*models.QueueInfo
	priorities map[int]*models.LookupItem
	types      map[int]*models.LookupItem
	statuses   map[int]*models.LookupItem
	auditLogs  []LookupAuditLog
	nextID     map[string]int // Track next ID for each entity type
}

// NewMemoryLookupRepository creates a new in-memory lookup repository with default data.
func NewMemoryLookupRepository() *MemoryLookupRepository {
	repo := &MemoryLookupRepository{
		queues:     make(map[int]*models.QueueInfo),
		priorities: make(map[int]*models.LookupItem),
		types:      make(map[int]*models.LookupItem),
		statuses:   make(map[int]*models.LookupItem),
		auditLogs:  make([]LookupAuditLog, 0),
		nextID: map[string]int{
			"queue":    7,
			"priority": 5,
			"type":     6,
			"status":   6,
			"audit":    1,
		},
	}

	// Initialize with default data
	repo.initializeDefaults()
	return repo
}

func (r *MemoryLookupRepository) initializeDefaults() {
	// Default queues
	r.queues[1] = &models.QueueInfo{ID: 1, Name: "General Support", Description: "General customer inquiries", Active: true}
	r.queues[2] = &models.QueueInfo{ID: 2, Name: "Technical Issues", Description: "Technical problems and bugs", Active: true}
	r.queues[3] = &models.QueueInfo{ID: 3, Name: "Billing", Description: "Billing and payment issues", Active: true}
	r.queues[4] = &models.QueueInfo{ID: 4, Name: "Feature Requests", Description: "New feature suggestions", Active: true}
	r.queues[5] = &models.QueueInfo{ID: 5, Name: "Sales", Description: "Sales inquiries", Active: true}
	r.queues[6] = &models.QueueInfo{ID: 6, Name: "Documentation", Description: "Documentation requests", Active: true}

	// Default priorities (OTRS-compatible)
	r.priorities[1] = &models.LookupItem{ID: 1, Value: "low", Label: "Low", Order: 1, Active: true}
	r.priorities[2] = &models.LookupItem{ID: 2, Value: "normal", Label: "Normal", Order: 2, Active: true}
	r.priorities[3] = &models.LookupItem{ID: 3, Value: "high", Label: "High", Order: 3, Active: true}
	r.priorities[4] = &models.LookupItem{ID: 4, Value: "urgent", Label: "Urgent", Order: 4, Active: true}

	// Default types
	r.types[1] = &models.LookupItem{ID: 1, Value: "incident", Label: "Incident", Order: 1, Active: true}
	r.types[2] = &models.LookupItem{ID: 2, Value: "service_request", Label: "Service Request", Order: 2, Active: true}
	r.types[3] = &models.LookupItem{ID: 3, Value: "change_request", Label: "Change Request", Order: 3, Active: true}
	r.types[4] = &models.LookupItem{ID: 4, Value: "problem", Label: "Problem", Order: 4, Active: true}
	r.types[5] = &models.LookupItem{ID: 5, Value: "question", Label: "Question", Order: 5, Active: true}

	// Default statuses (workflow order)
	r.statuses[1] = &models.LookupItem{ID: 1, Value: "new", Label: "New", Order: 1, Active: true}
	r.statuses[2] = &models.LookupItem{ID: 2, Value: "open", Label: "Open", Order: 2, Active: true}
	r.statuses[3] = &models.LookupItem{ID: 3, Value: "pending", Label: "Pending", Order: 3, Active: true}
	r.statuses[4] = &models.LookupItem{ID: 4, Value: "resolved", Label: "Resolved", Order: 4, Active: true}
	r.statuses[5] = &models.LookupItem{ID: 5, Value: "closed", Label: "Closed", Order: 5, Active: true}
}

// Queue operations.
func (r *MemoryLookupRepository) GetQueues(ctx context.Context) ([]models.QueueInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	queues := make([]models.QueueInfo, 0, len(r.queues))
	for _, q := range r.queues {
		queues = append(queues, *q)
	}

	// Sort by ID for consistent ordering
	sort.Slice(queues, func(i, j int) bool {
		return queues[i].ID < queues[j].ID
	})

	return queues, nil
}

func (r *MemoryLookupRepository) GetQueueByID(ctx context.Context, id int) (*models.QueueInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	queue, exists := r.queues[id]
	if !exists {
		return nil, fmt.Errorf("queue with ID %d not found", id)
	}

	result := *queue
	return &result, nil
}

func (r *MemoryLookupRepository) CreateQueue(ctx context.Context, queue *models.QueueInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	queue.ID = r.nextID["queue"]
	r.nextID["queue"]++

	r.queues[queue.ID] = queue
	return nil
}

func (r *MemoryLookupRepository) UpdateQueue(ctx context.Context, queue *models.QueueInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.queues[queue.ID]; !exists {
		return fmt.Errorf("queue with ID %d not found", queue.ID)
	}

	r.queues[queue.ID] = queue
	return nil
}

func (r *MemoryLookupRepository) DeleteQueue(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.queues[id]; !exists {
		return fmt.Errorf("queue with ID %d not found", id)
	}

	delete(r.queues, id)
	return nil
}

// Priority operations.
func (r *MemoryLookupRepository) GetPriorities(ctx context.Context) ([]models.LookupItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	priorities := make([]models.LookupItem, 0, len(r.priorities))
	for _, p := range r.priorities {
		priorities = append(priorities, *p)
	}

	// Sort by order for correct priority sequence
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].Order < priorities[j].Order
	})

	return priorities, nil
}

func (r *MemoryLookupRepository) GetPriorityByID(ctx context.Context, id int) (*models.LookupItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	priority, exists := r.priorities[id]
	if !exists {
		return nil, fmt.Errorf("priority with ID %d not found", id)
	}

	result := *priority
	return &result, nil
}

func (r *MemoryLookupRepository) UpdatePriority(ctx context.Context, priority *models.LookupItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.priorities[priority.ID]
	if !exists {
		return fmt.Errorf("priority with ID %d not found", priority.ID)
	}

	// Preserve the value (priorities are system-defined)
	priority.Value = existing.Value
	r.priorities[priority.ID] = priority
	return nil
}

// Type operations.
func (r *MemoryLookupRepository) GetTypes(ctx context.Context) ([]models.LookupItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]models.LookupItem, 0, len(r.types))
	for _, t := range r.types {
		types = append(types, *t)
	}

	// Sort by order
	sort.Slice(types, func(i, j int) bool {
		return types[i].Order < types[j].Order
	})

	return types, nil
}

func (r *MemoryLookupRepository) GetTypeByID(ctx context.Context, id int) (*models.LookupItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typ, exists := r.types[id]
	if !exists {
		return nil, fmt.Errorf("type with ID %d not found", id)
	}

	result := *typ
	return &result, nil
}

func (r *MemoryLookupRepository) CreateType(ctx context.Context, typ *models.LookupItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	typ.ID = r.nextID["type"]
	r.nextID["type"]++

	r.types[typ.ID] = typ
	return nil
}

func (r *MemoryLookupRepository) UpdateType(ctx context.Context, typ *models.LookupItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[typ.ID]; !exists {
		return fmt.Errorf("type with ID %d not found", typ.ID)
	}

	r.types[typ.ID] = typ
	return nil
}

func (r *MemoryLookupRepository) DeleteType(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[id]; !exists {
		return fmt.Errorf("type with ID %d not found", id)
	}

	delete(r.types, id)
	return nil
}

// Status operations.
func (r *MemoryLookupRepository) GetStatuses(ctx context.Context) ([]models.LookupItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make([]models.LookupItem, 0, len(r.statuses))
	for _, s := range r.statuses {
		statuses = append(statuses, *s)
	}

	// Sort by order for workflow sequence
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Order < statuses[j].Order
	})

	return statuses, nil
}

func (r *MemoryLookupRepository) GetStatusByID(ctx context.Context, id int) (*models.LookupItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status, exists := r.statuses[id]
	if !exists {
		return nil, fmt.Errorf("status with ID %d not found", id)
	}

	result := *status
	return &result, nil
}

func (r *MemoryLookupRepository) UpdateStatus(ctx context.Context, status *models.LookupItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.statuses[status.ID]
	if !exists {
		return fmt.Errorf("status with ID %d not found", status.ID)
	}

	// Preserve the value (statuses are system-defined for workflow)
	status.Value = existing.Value
	r.statuses[status.ID] = status
	return nil
}

// Audit operations.
func (r *MemoryLookupRepository) LogChange(ctx context.Context, change *LookupAuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	change.ID = r.nextID["audit"]
	r.nextID["audit"]++

	if change.Timestamp.IsZero() {
		change.Timestamp = time.Now()
	}

	r.auditLogs = append(r.auditLogs, *change)
	return nil
}

func (r *MemoryLookupRepository) GetAuditLogs(ctx context.Context, entityType string, entityID int, limit int) ([]LookupAuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []LookupAuditLog
	for _, log := range r.auditLogs {
		if log.EntityType == entityType && log.EntityID == entityID {
			filtered = append(filtered, log)
		}
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// Export/Import operations.
func (r *MemoryLookupRepository) ExportConfiguration(ctx context.Context) (*LookupConfiguration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config := &LookupConfiguration{
		Version:    "1.0",
		ExportedAt: time.Now(),
		ExportedBy: "system", // In production, use actual user
	}

	// Export queues
	for _, q := range r.queues {
		config.Queues = append(config.Queues, *q)
	}
	sort.Slice(config.Queues, func(i, j int) bool {
		return config.Queues[i].ID < config.Queues[j].ID
	})

	// Export priorities
	for _, p := range r.priorities {
		config.Priorities = append(config.Priorities, *p)
	}
	sort.Slice(config.Priorities, func(i, j int) bool {
		return config.Priorities[i].Order < config.Priorities[j].Order
	})

	// Export types
	for _, t := range r.types {
		config.Types = append(config.Types, *t)
	}
	sort.Slice(config.Types, func(i, j int) bool {
		return config.Types[i].Order < config.Types[j].Order
	})

	// Export statuses
	for _, s := range r.statuses {
		config.Statuses = append(config.Statuses, *s)
	}
	sort.Slice(config.Statuses, func(i, j int) bool {
		return config.Statuses[i].Order < config.Statuses[j].Order
	})

	return config, nil
}

func (r *MemoryLookupRepository) ImportConfiguration(ctx context.Context, config *LookupConfiguration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing data
	r.queues = make(map[int]*models.QueueInfo)
	r.priorities = make(map[int]*models.LookupItem)
	r.types = make(map[int]*models.LookupItem)
	r.statuses = make(map[int]*models.LookupItem)

	// Import queues
	maxQueueID := 0
	for _, q := range config.Queues {
		queue := q
		r.queues[queue.ID] = &queue
		if queue.ID > maxQueueID {
			maxQueueID = queue.ID
		}
	}
	r.nextID["queue"] = maxQueueID + 1

	// Import priorities
	maxPriorityID := 0
	for _, p := range config.Priorities {
		priority := p
		r.priorities[priority.ID] = &priority
		if priority.ID > maxPriorityID {
			maxPriorityID = priority.ID
		}
	}
	r.nextID["priority"] = maxPriorityID + 1

	// Import types
	maxTypeID := 0
	for _, t := range config.Types {
		typ := t
		r.types[typ.ID] = &typ
		if typ.ID > maxTypeID {
			maxTypeID = typ.ID
		}
	}
	r.nextID["type"] = maxTypeID + 1

	// Import statuses
	maxStatusID := 0
	for _, s := range config.Statuses {
		status := s
		r.statuses[status.ID] = &status
		if status.ID > maxStatusID {
			maxStatusID = status.ID
		}
	}
	r.nextID["status"] = maxStatusID + 1

	// Log the import (directly append since we already hold the lock)
	log := LookupAuditLog{
		EntityType: "system",
		EntityID:   0,
		Action:     "import",
		NewValue:   fmt.Sprintf("Imported configuration v%s", config.Version),
		UserEmail:  config.ExportedBy,
		Timestamp:  time.Now(),
	}
	r.auditLogs = append(r.auditLogs, log)

	return nil
}
