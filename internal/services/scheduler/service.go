package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/goatkit/goatflow/internal/cache"
	"github.com/goatkit/goatflow/internal/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/notifications"
	"github.com/goatkit/goatflow/internal/repository"
)

const (
	statusSuccess = "success"
	statusFailed  = "failed"
)

type ticketAutoCloser interface {
	AutoClosePendingTickets(ctx context.Context, now time.Time, transitions map[string]string, systemUserID int) (*repository.AutoCloseResult, error)
	FindDuePendingReminders(ctx context.Context, now time.Time, limit int) ([]*models.PendingReminder, error)
}

type emailAccountLister interface {
	GetActiveAccounts() ([]*models.EmailAccount, error)
}

// Handler executes a scheduled job.
type Handler func(context.Context, *models.ScheduledJob) error

// Service coordinates scheduled job execution.
type Service struct {
	db               *sql.DB
	ticketRepo       ticketAutoCloser
	emailRepo        emailAccountLister
	connectorFactory connector.Factory
	emailHandler     connector.Handler
	cron             *cron.Cron
	parser           cron.Parser
	handlers         map[string]Handler
	entries          map[string]cron.EntryID
	jobs             map[string]*models.ScheduledJob
	mu               sync.RWMutex
	handlerMu        sync.RWMutex
	rootCtx          context.Context
	logger           *log.Logger
	startOnce        sync.Once
	stopOnce         sync.Once
	location         *time.Location
	reminderHub      notifications.Hub
	emailPollState   emailPollState
	metrics          *emailPollMetrics
	valkey           *cache.RedisCache
}

type emailPollState struct {
	mu       sync.Mutex
	nextIdx  int
	lastPoll map[int]time.Time
}

// NewService wires a scheduler around the shared database connection.
func NewService(db *sql.DB, opts ...Option) *Service {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	if options.Logger == nil {
		options.Logger = log.Default()
	}

	location := options.Location
	if location == nil {
		location = time.UTC
	}

	ticketRepo := options.TicketRepo
	if ticketRepo == nil && db != nil {
		ticketRepo = repository.NewTicketRepository(db)
	}
	emailRepo := options.EmailRepo
	if emailRepo == nil && db != nil {
		emailRepo = repository.NewEmailAccountRepository(db)
	}
	connectorFactory := options.Factory
	if connectorFactory == nil {
		connectorFactory = connector.DefaultFactory()
	}
	cronEngine := options.Cron
	if cronEngine == nil {
		cronEngine = cron.New(cron.WithLocation(location))
	}
	var zeroParser cron.Parser
	parser := options.Parser
	if parser == zeroParser {
		parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	}

	jobs := make(map[string]*models.ScheduledJob)
	defs := options.Jobs
	if len(defs) == 0 {
		defs = defaultJobs()
	}
	for _, job := range defs {
		if job == nil || job.Slug == "" || job.Schedule == "" {
			continue
		}
		jobs[job.Slug] = job.Clone()
	}

	hub := options.ReminderHub
	if hub == nil {
		hub = notifications.GetHub()
	}

	s := &Service{
		db:               db,
		ticketRepo:       ticketRepo,
		emailRepo:        emailRepo,
		connectorFactory: connectorFactory,
		emailHandler:     options.EmailHandler,
		cron:             cronEngine,
		parser:           parser,
		handlers:         make(map[string]Handler),
		entries:          make(map[string]cron.EntryID),
		jobs:             jobs,
		logger:           options.Logger,
		location:         location,
		reminderHub:      hub,
		metrics:          globalEmailPollMetrics(),
		valkey:           options.Cache,
	}

	// The following line initializes emailPollState with nextIdx set to 0
	// This line is unchanged but included for clarity.
	s.emailPollState = emailPollState{nextIdx: 0}
	s.registerBuiltinHandlers()
	return s
}

// Run starts the scheduler loop until the context is cancelled.
func (s *Service) Run(ctx context.Context) error {
	s.startOnce.Do(func() {
		s.rootCtx = ctx
		s.scheduleAllJobs()
		s.cron.Start()
		s.runStartupJobs()
	})

	<-ctx.Done()
	s.stopCron()
	return nil
}

// runStartupJobs executes all jobs marked with RunOnStartup=true.
func (s *Service) runStartupJobs() {
	s.mu.RLock()
	var startupJobs []string
	for slug, job := range s.jobs {
		if job != nil && job.RunOnStartup {
			startupJobs = append(startupJobs, slug)
		}
	}
	s.mu.RUnlock()

	for _, slug := range startupJobs {
		s.mu.RLock()
		entryID := s.entries[slug]
		s.mu.RUnlock()
		go s.executeJob(slug, entryID)
	}
}

func (s *Service) scheduleAllJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for slug, job := range s.jobs {
		if job == nil {
			continue
		}
		if err := s.addJobLocked(job.Clone()); err != nil {
			s.logger.Printf("scheduler: failed to schedule job %s: %v", slug, err)
		}
	}
}

func (s *Service) stopCron() {
	s.stopOnce.Do(func() {
		ctx := s.cron.Stop()
		if ctx == nil {
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			s.logger.Printf("scheduler: timed out waiting for jobs to finish")
		}
	})
}

func (s *Service) addJobLocked(job *models.ScheduledJob) error {
	schedule, err := s.parser.Parse(job.Schedule)
	if err != nil {
		return err
	}

	slug := job.Slug
	var entryID cron.EntryID
	entryID = s.cron.Schedule(schedule, cron.FuncJob(func() {
		s.executeJob(slug, entryID)
	}))

	s.entries[slug] = entryID
	s.jobs[slug] = job
	return nil
}

func (s *Service) executeJob(slug string, entryID cron.EntryID) {
	job := s.jobSnapshot(slug)
	if job == nil {
		return
	}

	handler := s.getHandler(job.Handler)
	if handler == nil {
		start := s.now()
		finish := start
		s.finalizeRun(job, slug, entryID, start, finish, statusFailed, fmt.Errorf("handler %s not registered", job.Handler))
		return
	}

	ctx := s.rootCtx
	if ctx == nil {
		ctx = context.Background()
	}

	start := s.now()
	jobCtx := ctx
	var cancel context.CancelFunc
	if job.TimeoutSeconds > 0 {
		jobCtx, cancel = context.WithTimeout(ctx, time.Duration(job.TimeoutSeconds)*time.Second)
	}

	var runErr error
	func() {
		defer func() {
			if cancel != nil {
				cancel()
			}
			if r := recover(); r != nil {
				runErr = fmt.Errorf("panic: %v", r)
			}
		}()
		runErr = handler(jobCtx, job)
	}()

	finish := s.now()
	status := statusSuccess
	if runErr != nil {
		status = statusFailed
	}

	s.finalizeRun(job, slug, entryID, start, finish, status, runErr)
}

func (s *Service) finalizeRun(job *models.ScheduledJob, slug string, entryID cron.EntryID, start, finish time.Time, status string, runErr error) {
	duration := finish.Sub(start)
	cloned := job.Clone()
	cloned.LastRunAt = &finish
	cloned.LastDurationMS = duration.Milliseconds()
	cloned.LastStatus = status
	if runErr != nil {
		msg := runErr.Error()
		cloned.ErrorMessage = &msg
	} else {
		cloned.ErrorMessage = nil
	}

	if entry := s.cron.Entry(entryID); entry.ID != 0 && !entry.Next.IsZero() {
		next := entry.Next.In(s.location)
		cloned.NextRunAt = &next
	} else {
		cloned.NextRunAt = nil
	}

	s.applyExecutionResult(slug, cloned)
}

func (s *Service) now() time.Time {
	if s.location == nil {
		return time.Now()
	}
	return time.Now().In(s.location)
}

func (s *Service) applyExecutionResult(slug string, job *models.ScheduledJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[slug] = job.Clone()
}

func (s *Service) jobSnapshot(slug string) *models.ScheduledJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if job, ok := s.jobs[slug]; ok {
		return job.Clone()
	}
	return nil
}

func (s *Service) getHandler(name string) Handler {
	if name == "" {
		return nil
	}
	s.handlerMu.RLock()
	defer s.handlerMu.RUnlock()
	return s.handlers[name]
}

// RegisterHandler attaches or replaces a handler for the given name. Passing nil removes the handler.
func (s *Service) RegisterHandler(name string, handler Handler) {
	if name == "" {
		return
	}
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	if handler == nil {
		delete(s.handlers, name)
		return
	}
	s.handlers[name] = handler
}

// AddJob dynamically adds a job to the scheduler.
// The job's Handler field should match a registered handler name.
// Can be called after the scheduler has started.
func (s *Service) AddJob(job *models.ScheduledJob) error {
	if job == nil || job.Slug == "" || job.Schedule == "" {
		return fmt.Errorf("invalid job: slug and schedule required")
	}

	// Set the handler name to match the slug if not specified
	if job.Handler == "" {
		job.Handler = job.Slug
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If scheduler is already running, add the job immediately
	return s.addJobLocked(job.Clone())
}
