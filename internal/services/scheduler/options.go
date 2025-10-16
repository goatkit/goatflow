package scheduler

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
)

type options struct {
	Logger      *log.Logger
	TicketRepo  ticketAutoCloser
	EmailRepo   emailAccountLister
	Cron        *cron.Cron
	Parser      cron.Parser
	Jobs        []*models.ScheduledJob
	Location    *time.Location
	ReminderHub notifications.Hub
}

// Option applies configuration to the scheduler service.
type Option func(*options)

func defaultOptions() options {
	return options{Logger: log.Default(), Location: time.UTC}
}

// WithLogger injects a custom logger implementation.
func WithLogger(l *log.Logger) Option {
	return func(o *options) {
		o.Logger = l
	}
}

// WithTicketAutoCloser injects a custom ticket auto-close repository.
func WithTicketAutoCloser(repo ticketAutoCloser) Option {
	return func(o *options) {
		o.TicketRepo = repo
	}
}

// WithEmailAccountLister injects the repository used for email polling.
func WithEmailAccountLister(repo emailAccountLister) Option {
	return func(o *options) {
		o.EmailRepo = repo
	}
}

// WithCron supplies a preconfigured cron scheduler instance.
func WithCron(c *cron.Cron) Option {
	return func(o *options) {
		o.Cron = c
	}
}

// WithCronParser allows replacing the cron expression parser.
func WithCronParser(p cron.Parser) Option {
	return func(o *options) {
		o.Parser = p
	}
}

// WithJobs registers explicit job definitions instead of defaults.
func WithJobs(jobs []*models.ScheduledJob) Option {
	return func(o *options) {
		o.Jobs = jobs
	}
}

// WithLocation sets the scheduler timezone location.
func WithLocation(loc *time.Location) Option {
	return func(o *options) {
		o.Location = loc
	}
}

// WithReminderHub injects a custom reminder hub for dispatching pending reminders.
func WithReminderHub(h notifications.Hub) Option {
	return func(o *options) {
		o.ReminderHub = h
	}
}
