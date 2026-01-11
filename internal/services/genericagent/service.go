// Package genericagent provides the Generic Agent execution engine.
// Generic Agent jobs find tickets matching specified criteria and apply actions to them.
package genericagent

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// Service coordinates Generic Agent job execution.
type Service struct {
	db          *sql.DB
	jobRepo     *repository.GenericAgentRepository
	ticketRepo  *repository.TicketRepository
	articleRepo *repository.ArticleRepository
	logger      *log.Logger
	now         func() time.Time
}

// Option configures the service.
type Option func(*Service)

// WithLogger sets a custom logger.
func WithLogger(l *log.Logger) Option {
	return func(s *Service) { s.logger = l }
}

// WithNowFunc sets a custom time function (for testing).
func WithNowFunc(fn func() time.Time) Option {
	return func(s *Service) { s.now = fn }
}

// NewService creates a new Generic Agent service.
func NewService(db *sql.DB, opts ...Option) *Service {
	s := &Service{
		db:          db,
		jobRepo:     repository.NewGenericAgentRepository(db),
		ticketRepo:  repository.NewTicketRepository(db),
		articleRepo: repository.NewArticleRepository(db),
		logger:      log.Default(),
		now:         time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ExecuteAllDueJobs finds and executes all jobs that are due to run.
// This is called by the scheduler every minute.
func (s *Service) ExecuteAllDueJobs(ctx context.Context) error {
	jobs, err := s.jobRepo.GetValidJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get valid jobs: %w", err)
	}

	if len(jobs) == 0 {
		return nil
	}

	now := s.now()
	var executed int

	for _, job := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !s.shouldRun(job, now) {
			continue
		}

		if err := s.ExecuteJob(ctx, job, 1); err != nil {
			s.logger.Printf("genericagent: job %q failed: %v", job.Name, err)
			continue
		}
		executed++
	}

	if executed > 0 {
		s.logger.Printf("genericagent: executed %d job(s)", executed)
	}

	return nil
}

// shouldRun checks if a job should run based on its schedule and last run time.
func (s *Service) shouldRun(job *models.GenericAgentJob, now time.Time) bool {
	days := job.ScheduleDays()
	hours := job.ScheduleHours()
	minutes := job.ScheduleMinutes()

	// If no schedule defined, don't run automatically
	if len(days) == 0 && len(hours) == 0 && len(minutes) == 0 {
		return false
	}

	// Check day of week (0 = Sunday)
	if len(days) > 0 && !containsInt(days, int(now.Weekday())) {
		return false
	}

	// Check hour
	if len(hours) > 0 && !containsInt(hours, now.Hour()) {
		return false
	}

	// Check minute
	if len(minutes) > 0 && !containsInt(minutes, now.Minute()) {
		return false
	}

	// Check if already run this minute
	lastRun := job.ScheduleLastRun()
	if lastRun != nil {
		// Truncate both times to minute precision for comparison
		lastRunMinute := lastRun.Truncate(time.Minute)
		nowMinute := now.Truncate(time.Minute)
		if !nowMinute.After(lastRunMinute) {
			return false
		}
	}

	return true
}

// ExecuteJob runs a specific job immediately.
func (s *Service) ExecuteJob(ctx context.Context, job *models.GenericAgentJob, userID int) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	s.logger.Printf("genericagent: executing job %q", job.Name)

	// Find matching tickets
	ticketIDs, err := s.matchTickets(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to match tickets: %w", err)
	}

	if len(ticketIDs) == 0 {
		s.logger.Printf("genericagent: job %q matched 0 tickets", job.Name)
		// Still update last run time
		_ = s.jobRepo.UpdateLastRun(ctx, job.Name, s.now())
		return nil
	}

	s.logger.Printf("genericagent: job %q matched %d ticket(s)", job.Name, len(ticketIDs))

	// Apply actions to each ticket
	actions := job.Actions()
	if !actions.HasActions() {
		s.logger.Printf("genericagent: job %q has no actions defined", job.Name)
		_ = s.jobRepo.UpdateLastRun(ctx, job.Name, s.now())
		return nil
	}

	var processed int
	for _, ticketID := range ticketIDs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := s.applyActions(ctx, ticketID, actions, userID); err != nil {
			s.logger.Printf("genericagent: failed to apply actions to ticket %d: %v", ticketID, err)
			continue
		}
		processed++
	}

	s.logger.Printf("genericagent: job %q processed %d/%d ticket(s)", job.Name, processed, len(ticketIDs))

	// Update last run time
	if err := s.jobRepo.UpdateLastRun(ctx, job.Name, s.now()); err != nil {
		s.logger.Printf("genericagent: failed to update last run time for job %q: %v", job.Name, err)
	}

	return nil
}

// matchTickets finds tickets matching the job's criteria.
func (s *Service) matchTickets(ctx context.Context, job *models.GenericAgentJob) ([]int, error) {
	criteria := job.MatchCriteria()
	if !criteria.HasCriteria() {
		return nil, nil
	}

	// Build dynamic query based on criteria
	query := `SELECT t.id FROM ticket t WHERE 1=1`
	var args []interface{}
	argIdx := 0

	// State filter
	if ids := criteria.StateIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.ticket_state_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// Queue filter
	if ids := criteria.QueueIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.queue_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// Priority filter
	if ids := criteria.PriorityIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.ticket_priority_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// Type filter
	if ids := criteria.TypeIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.type_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// Lock filter
	if ids := criteria.LockIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.ticket_lock_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// Owner filter
	if ids := criteria.OwnerIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.user_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// Service filter
	if ids := criteria.ServiceIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.service_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// SLA filter
	if ids := criteria.SLAIDs(); len(ids) > 0 {
		placeholders := makePlaceholders(len(ids), &argIdx)
		query += fmt.Sprintf(" AND t.sla_id IN (%s)", placeholders)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	// Customer ID filter (supports wildcards)
	if customer := criteria.CustomerID(); customer != "" {
		argIdx++
		if strings.Contains(customer, "*") {
			customer = strings.ReplaceAll(customer, "*", "%")
			query += " AND t.customer_id LIKE ?"
		} else {
			query += " AND t.customer_id = ?"
		}
		args = append(args, customer)
	}

	// Customer user login filter
	if login := criteria.CustomerUserLogin(); login != "" {
		argIdx++
		if strings.Contains(login, "*") {
			login = strings.ReplaceAll(login, "*", "%")
			query += " AND t.customer_user_id LIKE ?"
		} else {
			query += " AND t.customer_user_id = ?"
		}
		args = append(args, login)
	}

	// Title filter (supports wildcards)
	if title := criteria.Title(); title != "" {
		argIdx++
		if strings.Contains(title, "*") {
			title = strings.ReplaceAll(title, "*", "%")
			query += " AND t.title LIKE ?"
		} else {
			query += " AND t.title = ?"
		}
		args = append(args, title)
	}

	now := s.now()

	// Time-based filters
	if mins := criteria.TicketCreateTimeOlderMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.create_time < ?"
		args = append(args, cutoff)
	}

	if mins := criteria.TicketCreateTimeNewerMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.create_time > ?"
		args = append(args, cutoff)
	}

	if mins := criteria.TicketChangeTimeOlderMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.change_time < ?"
		args = append(args, cutoff)
	}

	if mins := criteria.TicketChangeTimeNewerMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.change_time > ?"
		args = append(args, cutoff)
	}

	if mins := criteria.TicketPendingTimeOlderMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.until_time > 0 AND t.until_time < ?"
		args = append(args, cutoff.Unix())
	}

	if mins := criteria.TicketPendingTimeNewerMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.until_time > 0 AND t.until_time > ?"
		args = append(args, cutoff.Unix())
	}

	if mins := criteria.TicketEscalationTimeOlderMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.escalation_time > 0 AND t.escalation_time < ?"
		args = append(args, cutoff.Unix())
	}

	if mins := criteria.TicketEscalationTimeNewerMinutes(); mins > 0 {
		argIdx++
		cutoff := now.Add(-time.Duration(mins) * time.Minute)
		query += " AND t.escalation_time > 0 AND t.escalation_time > ?"
		args = append(args, cutoff.Unix())
	}

	// Add reasonable limit
	query += " ORDER BY t.id LIMIT 1000"

	query = database.ConvertPlaceholders(query)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ticketIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ticketIDs = append(ticketIDs, id)
	}

	return ticketIDs, rows.Err()
}

// applyActions applies the configured actions to a ticket.
func (s *Service) applyActions(ctx context.Context, ticketID int, actions *models.GenericAgentActions, userID int) error {
	now := s.now()

	// Build update query dynamically
	var setClauses []string
	var args []interface{}

	if id := actions.NewStateID(); id != nil {
		setClauses = append(setClauses, "ticket_state_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewQueueID(); id != nil {
		setClauses = append(setClauses, "queue_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewPriorityID(); id != nil {
		setClauses = append(setClauses, "ticket_priority_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewOwnerID(); id != nil {
		setClauses = append(setClauses, "user_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewResponsibleID(); id != nil {
		setClauses = append(setClauses, "responsible_user_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewLockID(); id != nil {
		setClauses = append(setClauses, "ticket_lock_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewTypeID(); id != nil {
		setClauses = append(setClauses, "type_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewServiceID(); id != nil {
		setClauses = append(setClauses, "service_id = ?")
		args = append(args, *id)
	}

	if id := actions.NewSLAID(); id != nil {
		setClauses = append(setClauses, "sla_id = ?")
		args = append(args, *id)
	}

	if customer := actions.NewCustomerID(); customer != "" {
		setClauses = append(setClauses, "customer_id = ?")
		args = append(args, customer)
	}

	if login := actions.NewCustomerUserLogin(); login != "" {
		setClauses = append(setClauses, "customer_user_id = ?")
		args = append(args, login)
	}

	if title := actions.NewTitle(); title != "" {
		setClauses = append(setClauses, "title = ?")
		args = append(args, title)
	}

	// Handle pending time
	if t := actions.NewPendingTime(); t != nil {
		setClauses = append(setClauses, "until_time = ?")
		args = append(args, t.Unix())
	} else if diff := actions.NewPendingTimeDiff(); diff != 0 {
		pendingTime := now.Add(time.Duration(diff) * time.Minute)
		setClauses = append(setClauses, "until_time = ?")
		args = append(args, pendingTime.Unix())
	}

	// Always update change_time and change_by
	if len(setClauses) > 0 {
		setClauses = append(setClauses, "change_time = ?", "change_by = ?")
		args = append(args, now, userID, ticketID)

		query := fmt.Sprintf("UPDATE ticket SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		query = database.ConvertPlaceholders(query)

		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to update ticket: %w", err)
		}
	}

	// Add note if configured
	if body := actions.NoteBody(); body != "" {
		subject := actions.NoteSubject()
		if subject == "" {
			subject = "Generic Agent Note"
		}

		article := &models.Article{
			TicketID:              ticketID,
			ArticleTypeID:         10, // note-internal
			SenderTypeID:          1,  // agent
			CommunicationChannelID: 1,
			IsVisibleForCustomer:  0, // Internal note
			Subject:               subject,
			Body:                  body,
			MimeType:              "text/plain",
			CreateBy:              userID,
			ChangeBy:              userID,
		}

		if err := s.articleRepo.Create(article); err != nil {
			return fmt.Errorf("failed to create note: %w", err)
		}
	}

	// Handle delete action
	if actions.Delete() {
		if err := s.ticketRepo.Delete(uint(ticketID)); err != nil {
			return fmt.Errorf("failed to delete ticket: %w", err)
		}
	}

	return nil
}

// containsInt checks if a slice contains an integer.
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// makePlaceholders creates SQL placeholders like "?, ?, ?".
func makePlaceholders(count int, _ *int) string {
	if count <= 0 {
		return ""
	}
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}
