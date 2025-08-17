package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/robfig/cron/v3"
)

// TriggerService handles workflow trigger events and scheduling
type TriggerService struct {
	workflowService *WorkflowService
	eventBus        *EventBus
	scheduler       *cron.Cron
	mu              sync.RWMutex
	scheduledJobs   map[int]cron.EntryID
}

// NewTriggerService creates a new trigger service
func NewTriggerService(workflowService *WorkflowService) *TriggerService {
	return &TriggerService{
		workflowService: workflowService,
		eventBus:        NewEventBus(),
		scheduler:       cron.New(cron.WithSeconds()),
		scheduledJobs:   make(map[int]cron.EntryID),
	}
}

// Start starts the trigger service
func (s *TriggerService) Start() error {
	// Register event handlers
	s.registerEventHandlers()
	
	// Start scheduler
	s.scheduler.Start()
	
	// Load scheduled workflows
	return s.loadScheduledWorkflows()
}

// Stop stops the trigger service
func (s *TriggerService) Stop() {
	s.scheduler.Stop()
	s.eventBus.Stop()
}

// registerEventHandlers registers all event handlers
func (s *TriggerService) registerEventHandlers() {
	// Ticket events
	s.eventBus.Subscribe("ticket.created", s.handleTicketCreated)
	s.eventBus.Subscribe("ticket.updated", s.handleTicketUpdated)
	s.eventBus.Subscribe("ticket.assigned", s.handleTicketAssigned)
	s.eventBus.Subscribe("ticket.status_changed", s.handleStatusChanged)
	s.eventBus.Subscribe("ticket.priority_changed", s.handlePriorityChanged)
	
	// Customer events
	s.eventBus.Subscribe("customer.replied", s.handleCustomerReply)
	
	// Agent events
	s.eventBus.Subscribe("agent.replied", s.handleAgentReply)
	
	// SLA events
	s.eventBus.Subscribe("sla.warning", s.handleSLAWarning)
	s.eventBus.Subscribe("sla.breach", s.handleSLABreach)
}

// Event handlers

func (s *TriggerService) handleTicketCreated(event Event) {
	context := s.buildContext(event)
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeTicketCreated, context)
}

func (s *TriggerService) handleTicketUpdated(event Event) {
	context := s.buildContext(event)
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeTicketUpdated, context)
}

func (s *TriggerService) handleTicketAssigned(event Event) {
	context := s.buildContext(event)
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeTicketAssigned, context)
}

func (s *TriggerService) handleStatusChanged(event Event) {
	context := s.buildContext(event)
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeStatusChanged, context)
}

func (s *TriggerService) handlePriorityChanged(event Event) {
	context := s.buildContext(event)
	_ = s.workflowService.ProcessTrigger(models.TriggerTypePriorityChanged, context)
}

func (s *TriggerService) handleCustomerReply(event Event) {
	context := s.buildContext(event)
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeCustomerReply, context)
}

func (s *TriggerService) handleAgentReply(event Event) {
	context := s.buildContext(event)
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeAgentReply, context)
}

func (s *TriggerService) handleSLAWarning(event Event) {
	context := s.buildContext(event)
	context["sla_type"] = "warning"
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeTimeBasedSLA, context)
}

func (s *TriggerService) handleSLABreach(event Event) {
	context := s.buildContext(event)
	context["sla_type"] = "breach"
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeTimeBasedSLA, context)
}

// buildContext builds execution context from event
func (s *TriggerService) buildContext(event Event) map[string]interface{} {
	context := make(map[string]interface{})
	
	// Copy all event data to context
	for k, v := range event.Data {
		context[k] = v
	}
	
	// Add metadata
	context["event_type"] = event.Type
	context["event_time"] = event.Timestamp
	
	return context
}

// Scheduled workflow management

// loadScheduledWorkflows loads all scheduled workflows
func (s *TriggerService) loadScheduledWorkflows() error {
	// Get active schedules from repository
	// This would load from database in real implementation
	schedules := s.getActiveSchedules()
	
	for _, schedule := range schedules {
		if err := s.scheduleWorkflow(schedule); err != nil {
			return fmt.Errorf("failed to schedule workflow %d: %w", schedule.WorkflowID, err)
		}
	}
	
	return nil
}

// scheduleWorkflow schedules a workflow for execution
func (s *TriggerService) scheduleWorkflow(schedule *models.WorkflowSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Remove existing schedule if any
	if existingID, exists := s.scheduledJobs[schedule.WorkflowID]; exists {
		s.scheduler.Remove(existingID)
	}
	
	// Add new schedule
	entryID, err := s.scheduler.AddFunc(schedule.CronExpr, func() {
		s.executeScheduledWorkflow(schedule.WorkflowID)
	})
	
	if err != nil {
		return err
	}
	
	s.scheduledJobs[schedule.WorkflowID] = entryID
	return nil
}

// unscheduleWorkflow removes a workflow from schedule
func (s *TriggerService) unscheduleWorkflow(workflowID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if entryID, exists := s.scheduledJobs[workflowID]; exists {
		s.scheduler.Remove(entryID)
		delete(s.scheduledJobs, workflowID)
	}
}

// executeScheduledWorkflow executes a scheduled workflow
func (s *TriggerService) executeScheduledWorkflow(workflowID int) {
	context := map[string]interface{}{
		"trigger_type": "scheduled",
		"workflow_id":  workflowID,
		"executed_at":  time.Now(),
	}
	
	_ = s.workflowService.ProcessTrigger(models.TriggerTypeScheduled, context)
}

// getActiveSchedules retrieves active schedules (placeholder)
func (s *TriggerService) getActiveSchedules() []*models.WorkflowSchedule {
	// This would fetch from repository in real implementation
	return []*models.WorkflowSchedule{}
}

// Public methods for managing triggers

// TriggerTicketEvent triggers a ticket-related event
func (s *TriggerService) TriggerTicketEvent(eventType string, ticketID int, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["ticket_id"] = ticketID
	
	s.eventBus.Publish(Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	})
}

// TriggerCustomEvent triggers a custom event
func (s *TriggerService) TriggerCustomEvent(eventType string, data map[string]interface{}) {
	s.eventBus.Publish(Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	})
}

// ScheduleWorkflow schedules a workflow for periodic execution
func (s *TriggerService) ScheduleWorkflow(workflowID int, cronExpr string) error {
	schedule := &models.WorkflowSchedule{
		WorkflowID: workflowID,
		CronExpr:   cronExpr,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}
	
	return s.scheduleWorkflow(schedule)
}

// UnscheduleWorkflow removes a workflow from schedule
func (s *TriggerService) UnscheduleWorkflow(workflowID int) {
	s.unscheduleWorkflow(workflowID)
}

// EventBus implementation

// Event represents a system event
type Event struct {
	Type      string
	Data      map[string]interface{}
	Timestamp time.Time
}

// EventHandler is a function that handles events
type EventHandler func(Event)

// EventBus manages event publishing and subscription
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]EventHandler
	eventChan   chan Event
	stopChan    chan bool
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	eb := &EventBus{
		subscribers: make(map[string][]EventHandler),
		eventChan:   make(chan Event, 1000),
		stopChan:    make(chan bool),
	}
	
	// Start event processor
	go eb.processEvents()
	
	return eb
}

// Subscribe registers a handler for an event type
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

// Unsubscribe removes a handler for an event type
func (eb *EventBus) Unsubscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	// Implementation would remove specific handler
	// For simplicity, clearing all handlers for the event type
	delete(eb.subscribers, eventType)
}

// Publish publishes an event
func (eb *EventBus) Publish(event Event) {
	select {
	case eb.eventChan <- event:
	default:
		// Event channel full, log warning
		fmt.Printf("Warning: Event channel full, dropping event: %s\n", event.Type)
	}
}

// processEvents processes events from the channel
func (eb *EventBus) processEvents() {
	for {
		select {
		case event := <-eb.eventChan:
			eb.handleEvent(event)
		case <-eb.stopChan:
			return
		}
	}
}

// handleEvent handles a single event
func (eb *EventBus) handleEvent(event Event) {
	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	eb.mu.RUnlock()
	
	// Execute handlers in goroutines for parallel processing
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			// Recover from panics in handlers
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Error in event handler: %v\n", r)
				}
			}()
			h(event)
		}(handler)
	}
	
	// Wait for all handlers to complete
	wg.Wait()
}

// Stop stops the event bus
func (eb *EventBus) Stop() {
	close(eb.stopChan)
}