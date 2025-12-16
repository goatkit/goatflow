package scheduler

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/adapter"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
)

func (s *Service) registerBuiltinHandlers() {
	s.RegisterHandler("ticket.autoClose", s.handleAutoClose)
	s.RegisterHandler("ticket.pendingReminder", s.handlePendingReminder)
	s.RegisterHandler("email.poll", s.handleEmailPoll)
	s.RegisterHandler("scheduler.housekeeping", s.handleHousekeeping)
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
				s.recordEmailPollResult(account, false)
				return
			}
			if err := fetcher.Fetch(ctx, account, s.emailHandler); err != nil {
				s.logger.Printf("scheduler: fetch error for account %s: %v", model.Login, err)
				errMu.Lock()
				fetchErrs = append(fetchErrs, err)
				errMu.Unlock()
				s.recordEmailPollResult(account, false)
				return
			}
			s.recordEmailPollResult(account, true)
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

func (s *Service) recordEmailPollResult(account connector.Account, success bool) {
	s.markAccountPolled(account.ID, s.now())
	if s.metrics == nil {
		return
	}
	s.metrics.recordAccount(account, success)
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
