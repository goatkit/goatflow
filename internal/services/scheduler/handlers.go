// Package scheduler provides task scheduling and job management.
package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/email/inbound/adapter"
	"github.com/goatkit/goatflow/internal/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/notifications"
	"github.com/goatkit/goatflow/internal/services/escalation"
	"github.com/goatkit/goatflow/internal/services/genericagent"
)

func (s *Service) registerBuiltinHandlers() {
	s.RegisterHandler("ticket.autoClose", s.handleAutoClose)
	s.RegisterHandler("ticket.pendingReminder", s.handlePendingReminder)
	s.RegisterHandler("email.poll", s.handleEmailPoll)
	s.RegisterHandler("scheduler.housekeeping", s.handleHousekeeping)
	s.RegisterHandler("genericAgent.execute", s.handleGenericAgentExecute)
	s.RegisterHandler("escalation.check", s.handleEscalationCheck)
	s.RegisterHandler("metrics.ticketActivity", s.handleMetricsTicketActivity)
}

func (s *Service) handleAutoClose(ctx context.Context, job *models.ScheduledJob) error {
	if s.ticketRepo == nil {
		s.logger.Printf("scheduler: ticket repository unavailable, skipping autoClose")
		return nil
	}
	transitions := map[string]string{
		"pending auto close+": "closed successful",
		"pending auto close-": "closed unsuccessful",
	}
	if cfg := transitionsFromConfig(job.Config); len(cfg) > 0 {
		transitions = cfg
	}

	systemUserID := intFromConfig(job.Config, "system_user_id", 1)
	result, err := s.ticketRepo.AutoClosePendingTickets(ctx, s.now(), transitions, systemUserID)
	if err != nil {
		return err
	}
	if result != nil {
		s.logger.Printf("scheduler: autoClose transitioned %d ticket(s) %+v", result.Total, result.Transitions)
	}
	return nil
}

func (s *Service) handleEmailPoll(ctx context.Context, job *models.ScheduledJob) error {
	s.logger.Printf("scheduler: email poll starting")
	if s.emailRepo == nil {
		s.logger.Printf("scheduler: email repository unavailable, skipping poll")
		return nil
	}
	if s.connectorFactory == nil {
		s.logger.Printf("scheduler: connector factory unavailable, skipping poll")
		return nil
	}
	if s.emailHandler == nil {
		s.logger.Printf("scheduler: email handler unavailable, skipping poll")
		return nil
	}
	accounts, err := s.emailRepo.GetActiveAccounts()
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		s.logger.Printf("scheduler: email poll found no active accounts")
		return nil
	}
	stopTimer := s.startEmailPollRun(len(accounts))
	defer stopTimer()

	limit := intFromConfig(job.Config, "max_accounts", 5)
	count := len(accounts)
	if limit > 0 && count > limit {
		count = limit
	}
	now := s.now()
	targets := s.selectEmailPollAccounts(accounts, count, now)
	if len(targets) == 0 {
		s.logger.Printf("scheduler: email poll skipped (no eligible accounts among %d)", len(accounts))
		return nil
	}
	workers := intFromConfig(job.Config, "worker_count", 2)
	if workers <= 0 {
		workers = 1
	}
	s.logger.Printf("scheduler: email poll dispatching %d of %d account(s) with %d worker(s)", len(targets), len(accounts), workers)

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var fetchErrs []error
	for _, model := range targets {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(model *models.EmailAccount) {
			defer wg.Done()
			defer func() { <-sem }()
			account := adapter.AccountFromModel(model)
			fetcher, ferr := s.connectorFactory.FetcherFor(account)
			if ferr != nil {
				s.logger.Printf("scheduler: no fetcher for account %s (%s): %v", model.Login, model.AccountType, ferr)
				errMu.Lock()
				fetchErrs = append(fetchErrs, ferr)
				errMu.Unlock()
				s.recordEmailPollResult(ctx, account, 0, false, ferr)
				return
			}
			if err := fetcher.Fetch(ctx, account, s.emailHandler); err != nil {
				s.logger.Printf("scheduler: fetch error for account %s: %v", model.Login, err)
				errMu.Lock()
				fetchErrs = append(fetchErrs, err)
				errMu.Unlock()
				s.recordEmailPollResult(ctx, account, 0, false, err)
				return
			}
			s.recordEmailPollResult(ctx, account, 0, true, nil)
		}(model)
	}

	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(fetchErrs) > 0 {
		return errors.Join(fetchErrs...)
	}

	s.logger.Printf("scheduler: email poll processed %d account(s)", len(targets))
	return nil
}

func (s *Service) startEmailPollRun(activeAccounts int) func() {
	if s.metrics == nil {
		return func() {}
	}
	return s.metrics.recordRun(activeAccounts)
}

//nolint:unparam // processed is passed but currently always 0, used in status map
func (s *Service) recordEmailPollResult(ctx context.Context, account connector.Account, processed int, success bool, err error) {
	s.markAccountPolled(account.ID, s.now())
	if s.metrics != nil {
		s.metrics.recordAccount(account, success)
	}
	// Persist lightweight status to Valkey so UI can surface poll health.
	if s.valkey == nil {
		return
	}
	status := map[string]interface{}{
		"account_id":       account.ID,
		"last_poll_at":     s.now().UTC(),
		"last_status":      boolToStatus(success),
		"last_error":       "",
		"messages_fetched": processed,
		"next_poll_eta":    nextPollETA(account, s.now()),
	}
	if err != nil {
		status["last_error"] = err.Error()
	}
	key := s.valkeyStatusKey(account.ID)
	// best effort: ignore errors
	_ = s.valkey.Set(ctx, key, status, 24*time.Hour)
}

func boolToStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

func nextPollETA(account connector.Account, now time.Time) time.Time {
	interval := account.PollInterval
	if interval <= 0 {
		return time.Time{}
	}
	return now.Add(interval).UTC()
}

func (s *Service) valkeyStatusKey(accountID int) string {
	return fmt.Sprintf("mail_poll_status:%d", accountID)
}

func (s *Service) selectEmailPollAccounts(accounts []*models.EmailAccount, count int, now time.Time) []*models.EmailAccount {
	if len(accounts) == 0 || count <= 0 {
		return nil
	}
	total := len(accounts)
	if count > total {
		count = total
	}
	s.emailPollState.mu.Lock()
	defer s.emailPollState.mu.Unlock()

	start := s.emailPollState.nextIdx % total
	selected := make([]*models.EmailAccount, 0, count)
	idx := start
	scanned := 0
	for scanned < total && len(selected) < count {
		account := accounts[idx]
		if s.accountReadyLocked(account, now) {
			selected = append(selected, account)
		}
		idx = (idx + 1) % total
		scanned++
	}
	s.emailPollState.nextIdx = idx
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func (s *Service) accountReadyLocked(account *models.EmailAccount, now time.Time) bool {
	if account == nil {
		return false
	}
	interval := time.Duration(account.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		return true
	}
	if s.emailPollState.lastPoll == nil {
		return true
	}
	last, ok := s.emailPollState.lastPoll[account.ID]
	if !ok {
		return true
	}
	return now.Sub(last) >= interval
}

func (s *Service) markAccountPolled(accountID int, when time.Time) {
	s.emailPollState.mu.Lock()
	if s.emailPollState.lastPoll == nil {
		s.emailPollState.lastPoll = make(map[int]time.Time)
	}
	s.emailPollState.lastPoll[accountID] = when
	s.emailPollState.mu.Unlock()
}

func (s *Service) handleHousekeeping(ctx context.Context, job *models.ScheduledJob) error {
	s.logger.Printf("scheduler: housekeeping placeholder running")
	return nil
}

func (s *Service) handleGenericAgentExecute(ctx context.Context, job *models.ScheduledJob) error {
	if s.db == nil {
		s.logger.Printf("scheduler: database unavailable, skipping genericAgent")
		return nil
	}

	svc := genericagent.NewService(s.db, genericagent.WithLogger(s.logger))
	return svc.ExecuteAllDueJobs(ctx)
}

func (s *Service) handleEscalationCheck(ctx context.Context, job *models.ScheduledJob) error {
	if s.db == nil {
		s.logger.Printf("scheduler: database unavailable, skipping escalation check")
		return nil
	}

	// Initialize escalation service
	escService := escalation.NewService(s.db, s.logger)
	if err := escService.Initialize(ctx); err != nil {
		s.logger.Printf("scheduler: failed to initialize escalation service: %v", err)
		return err
	}

	// Create check service using the calendar service from escalation service
	calService := escalation.NewCalendarService(s.db)
	if err := calService.LoadCalendars(ctx); err != nil {
		s.logger.Printf("scheduler: failed to load calendars: %v", err)
		return err
	}

	checkService := escalation.NewCheckService(s.db, calService, s.logger)

	// Set decay time from config if provided
	if decayTime := intFromConfig(job.Config, "decay_time_minutes", 0); decayTime > 0 {
		checkService.SetDecayTime(decayTime)
	}

	// Check escalations and trigger events
	events, err := checkService.CheckEscalations(ctx)
	if err != nil {
		return err
	}

	if len(events) > 0 {
		s.logger.Printf("scheduler: escalation check triggered %d event(s)", len(events))
		for _, evt := range events {
			s.logger.Printf("scheduler: escalation event %s for ticket %d", evt.EventName, evt.TicketID)
			// TODO: Integrate with event/notification system when available
		}
	}

	return nil
}

// handleMetricsTicketActivity calculates ticket activity counts
// for various time periods and caches them in Valkey.
func (s *Service) handleMetricsTicketActivity(ctx context.Context, job *models.ScheduledJob) error {
	if s.db == nil {
		s.logger.Printf("scheduler: database unavailable, skipping ticket activity metrics")
		return nil
	}

	metrics := calculateTicketActivityMetrics(s.db)

	if s.valkey != nil {
		if err := s.valkey.SetObject(ctx, "metrics:ticket_activity", metrics, 2*time.Minute); err != nil {
			s.logger.Printf("scheduler: failed to cache ticket activity metrics: %v", err)
		}
	}

	s.logger.Printf("scheduler: ticket activity updated: closed_day=%d closed_week=%d created_day=%d created_week=%d open=%d",
		metrics["closed_day"], metrics["closed_week"], metrics["created_day"], metrics["created_week"], metrics["open"])
	return nil
}

// calculateTicketActivityMetrics computes ticket counts for dashboard display.
func calculateTicketActivityMetrics(db *sql.DB) map[string]int {
	metrics := make(map[string]int)

	// Tickets closed in last 24 hours
	metrics["closed_day"] = getTicketCount(db, "closed", 1)
	// Tickets closed in last 7 days
	metrics["closed_week"] = getTicketCount(db, "closed", 7)
	// Tickets closed in last 30 days
	metrics["closed_month"] = getTicketCount(db, "closed", 30)

	// Tickets created in last 24 hours
	metrics["created_day"] = getTicketCount(db, "created", 1)
	// Tickets created in last 7 days
	metrics["created_week"] = getTicketCount(db, "created", 7)
	// Tickets created in last 30 days
	metrics["created_month"] = getTicketCount(db, "created", 30)

	// Currently open tickets
	metrics["open"] = getOpenTicketCount(db)

	return metrics
}

// getTicketCount returns the count of tickets closed or created within the specified days.
func getTicketCount(db *sql.DB, countType string, days int) int {
	var query string
	if countType == "closed" {
		query = database.ConvertPlaceholders(`
			SELECT COUNT(*)
			FROM ticket
			WHERE ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)
			  AND change_time >= DATE_SUB(NOW(), INTERVAL ? DAY)
		`)
	} else {
		query = database.ConvertPlaceholders(`
			SELECT COUNT(*)
			FROM ticket
			WHERE create_time >= DATE_SUB(NOW(), INTERVAL ? DAY)
		`)
	}
	var count int
	_ = db.QueryRow(query, days).Scan(&count)
	return count
}

// getOpenTicketCount returns the count of currently open tickets.
func getOpenTicketCount(db *sql.DB) int {
	query := `
		SELECT COUNT(*)
		FROM ticket
		WHERE ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2, 4))
	`
	var count int
	_ = db.QueryRow(query).Scan(&count)
	return count
}

func defaultJobs() []*models.ScheduledJob {
	return []*models.ScheduledJob{
		{
			Name:           "Pending Reminder Notifications",
			Slug:           "pending-reminder",
			Handler:        "ticket.pendingReminder",
			Schedule:       "*/1 * * * *",
			TimeoutSeconds: 60,
			Config: map[string]any{
				"limit": 100,
			},
		},
		{
			Name:           "Auto-close Pending Tickets",
			Slug:           "pending-auto-close",
			Handler:        "ticket.autoClose",
			Schedule:       "*/5 * * * *",
			TimeoutSeconds: 120,
			Config: map[string]any{
				"transitions": map[string]string{
					"pending auto close+": "closed successful",
					"pending auto close-": "closed unsuccessful",
				},
				"system_user_id": 1,
			},
		},
		{
			Name:           "Email Account Poller",
			Slug:           "email-ingest",
			Handler:        "email.poll",
			Schedule:       "*/2 * * * *",
			TimeoutSeconds: 300,
			Config: map[string]any{
				"max_accounts": 5,
				"worker_count": 2,
			},
		},
		{
			Name:           "Scheduler Housekeeping",
			Slug:           "scheduler-housekeeping",
			Handler:        "scheduler.housekeeping",
			Schedule:       "0 3 * * *",
			TimeoutSeconds: 600,
			Config: map[string]any{
				"retention_days": 30,
			},
		},
		{
			Name:           "Generic Agent Job Executor",
			Slug:           "generic-agent",
			Handler:        "genericAgent.execute",
			Schedule:       "* * * * *",
			TimeoutSeconds: 300,
			Config:         map[string]any{},
		},
		{
			Name:           "Escalation Check",
			Slug:           "escalation-check",
			Handler:        "escalation.check",
			Schedule:       "* * * * *",
			TimeoutSeconds: 120,
			Config: map[string]any{
				"decay_time_minutes": 0, // 0 = no decay, events triggered every run
			},
		},
		{
			Name:           "Ticket Activity Metrics",
			Slug:           "metrics-ticket-activity",
			Handler:        "metrics.ticketActivity",
			Schedule:       "@every 1m",
			TimeoutSeconds: 30,
			RunOnStartup:   true, // Populate cache immediately so dashboard has data
			Config:         map[string]any{},
		},
	}
}

// DefaultJobs returns a cloned copy of the built-in scheduled jobs.
func DefaultJobs() []*models.ScheduledJob {
	jobs := defaultJobs()
	out := make([]*models.ScheduledJob, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		out = append(out, job.Clone())
	}
	return out
}

func (s *Service) handlePendingReminder(ctx context.Context, job *models.ScheduledJob) error {
	if s.ticketRepo == nil {
		s.logger.Printf("scheduler: ticket repository unavailable, skipping pendingReminder")
		return nil
	}
	if s.reminderHub == nil {
		s.logger.Printf("scheduler: reminder hub unavailable, skipping pendingReminder")
		return nil
	}

	limit := intFromConfig(job.Config, "limit", 50)
	reminders, err := s.ticketRepo.FindDuePendingReminders(ctx, s.now(), limit)
	if err != nil {
		return err
	}
	if len(reminders) == 0 {
		return nil
	}

	dispatched := 0
	for _, reminder := range reminders {
		recipients := recipientsForReminder(reminder)
		if len(recipients) == 0 {
			continue
		}
		payload := convertReminder(reminder)
		if err := s.reminderHub.Dispatch(ctx, recipients, payload); err != nil {
			s.logger.Printf("scheduler: failed to dispatch pending reminder for ticket %s: %v", reminder.TicketNumber, err)
			continue
		}
		dispatched++
	}

	if dispatched > 0 {
		s.logger.Printf("scheduler: pending reminder dispatched %d ticket(s)", dispatched)
	}
	return nil
}

func recipientsForReminder(reminder *models.PendingReminder) []int {
	if reminder == nil {
		return nil
	}
	var out []int
	if reminder.ResponsibleUserID != nil && *reminder.ResponsibleUserID > 0 {
		out = append(out, *reminder.ResponsibleUserID)
	}
	if len(out) == 0 && reminder.OwnerUserID != nil && *reminder.OwnerUserID > 0 {
		out = append(out, *reminder.OwnerUserID)
	}
	return out
}

func convertReminder(reminder *models.PendingReminder) notifications.PendingReminder {
	if reminder == nil {
		return notifications.PendingReminder{}
	}
	return notifications.PendingReminder{
		TicketID:     reminder.TicketID,
		TicketNumber: reminder.TicketNumber,
		Title:        reminder.Title,
		QueueID:      reminder.QueueID,
		QueueName:    reminder.QueueName,
		PendingUntil: reminder.PendingUntil,
		StateName:    reminder.StateName,
	}
}

func intFromConfig(cfg map[string]any, key string, def int) int {
	if cfg == nil {
		return def
	}
	val, ok := cfg[key]
	if !ok {
		return def
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return n
		}
	}
	return def
}

func transitionsFromConfig(cfg map[string]any) map[string]string {
	result := make(map[string]string)
	if cfg == nil {
		return result
	}
	raw, ok := cfg["transitions"]
	if !ok {
		return result
	}
	switch t := raw.(type) {
	case map[string]any:
		for k, v := range t {
			name := strings.TrimSpace(k)
			if name == "" {
				continue
			}
			if str, ok := v.(string); ok {
				result[name] = strings.TrimSpace(str)
			}
		}
	case map[string]string:
		for k, v := range t {
			result[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return result
}
