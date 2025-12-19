package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

type stubTicketRepository struct {
	reminders []*models.PendingReminder
	limit     int
}

func (s *stubTicketRepository) AutoClosePendingTickets(ctx context.Context, now time.Time, transitions map[string]string, systemUserID int) (*repository.AutoCloseResult, error) {
	return nil, nil
}

func (s *stubTicketRepository) FindDuePendingReminders(ctx context.Context, now time.Time, limit int) ([]*models.PendingReminder, error) {
	s.limit = limit
	return s.reminders, nil
}

type stubEmailRepository struct {
	accounts []*models.EmailAccount
}

func (s *stubEmailRepository) GetActiveAccounts() ([]*models.EmailAccount, error) {
	return s.accounts, nil
}

type stubReminderHub struct {
	mu         sync.Mutex
	dispatched []struct {
		recipients []int
		reminder   notifications.PendingReminder
	}
}

func (s *stubReminderHub) Dispatch(ctx context.Context, recipients []int, reminder notifications.PendingReminder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	storedRecipients := append([]int(nil), recipients...)
	s.dispatched = append(s.dispatched, struct {
		recipients []int
		reminder   notifications.PendingReminder
	}{recipients: storedRecipients, reminder: reminder})
	return nil
}

func (s *stubReminderHub) Consume(userID int) []notifications.PendingReminder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

func TestHandlePendingReminderDispatches(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubTicketRepository{
		reminders: []*models.PendingReminder{{
			TicketID:          10,
			TicketNumber:      "202510161000010",
			Title:             "Follow up",
			QueueID:           2,
			QueueName:         "Support",
			PendingUntil:      time.Now().Add(-time.Minute).UTC(),
			ResponsibleUserID: intPtr(4),
			StateName:         "pending reminder",
		}},
	}
	hub := &stubReminderHub{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithTicketAutoCloser(repo),
		WithReminderHub(hub),
	)

	job := &models.ScheduledJob{Config: map[string]any{"limit": 25}}
	if err := svc.handlePendingReminder(context.Background(), job); err != nil {
		t.Fatalf("handlePendingReminder returned error: %v", err)
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.dispatched) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(hub.dispatched))
	}
	record := hub.dispatched[0]
	if len(record.recipients) != 1 || record.recipients[0] != 4 {
		t.Fatalf("unexpected recipients: %+v", record.recipients)
	}
	if record.reminder.TicketID != 10 {
		t.Fatalf("unexpected reminder ticket id %d", record.reminder.TicketID)
	}
}

func TestHandlePendingReminderOwnerFallback(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubTicketRepository{
		reminders: []*models.PendingReminder{{
			TicketID:     11,
			TicketNumber: "202510161000011",
			Title:        "Call customer",
			QueueID:      2,
			QueueName:    "Support",
			PendingUntil: time.Now().Add(-time.Minute).UTC(),
			OwnerUserID:  intPtr(7),
			StateName:    "pending reminder",
		}},
	}
	hub := &stubReminderHub{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithTicketAutoCloser(repo),
		WithReminderHub(hub),
	)

	job := &models.ScheduledJob{}
	if err := svc.handlePendingReminder(context.Background(), job); err != nil {
		t.Fatalf("handlePendingReminder returned error: %v", err)
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.dispatched) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(hub.dispatched))
	}
	record := hub.dispatched[0]
	if len(record.recipients) != 1 || record.recipients[0] != 7 {
		t.Fatalf("expected fallback recipient 7, got %+v", record.recipients)
	}
}

func TestHandlePendingReminderRespectsLimit(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubTicketRepository{}
	hub := &stubReminderHub{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithTicketAutoCloser(repo),
		WithReminderHub(hub),
	)

	job := &models.ScheduledJob{Config: map[string]any{"limit": 15}}
	if err := svc.handlePendingReminder(context.Background(), job); err != nil {
		t.Fatalf("handlePendingReminder returned error: %v", err)
	}

	if repo.limit != 15 {
		t.Fatalf("expected limit 15, got %d", repo.limit)
	}
}

func TestHandleEmailPollInvokesFetcher(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubEmailRepository{
		accounts: []*models.EmailAccount{
			{ID: 1, Login: "agent", Host: "mail.example", AccountType: "POP3", PasswordEncrypted: "pw"},
			{ID: 2, Login: "ops", Host: "mail.ops", AccountType: "POP3S", PasswordEncrypted: "pw"},
		},
	}
	fetcher := &recordingFetcher{}
	factory := &recordingFactory{fetcher: fetcher}
	handler := &stubConnectorHandler{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithEmailAccountLister(repo),
		WithConnectorFactory(factory),
		WithEmailHandler(handler),
	)

	job := &models.ScheduledJob{Config: map[string]any{"max_accounts": 1}}
	if err := svc.handleEmailPoll(context.Background(), job); err != nil {
		t.Fatalf("handleEmailPoll returned error: %v", err)
	}

	if fetcher.CallCount() != 1 {
		t.Fatalf("expected 1 fetch call, got %d", fetcher.CallCount())
	}
	if handler.Count() != 1 {
		t.Fatalf("expected handler to run once, got %d", handler.Count())
	}
}

func TestHandleEmailPollPropagatesErrors(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubEmailRepository{
		accounts: []*models.EmailAccount{{ID: 1, Login: "agent", Host: "mail.example", AccountType: "POP3", PasswordEncrypted: "pw"}},
	}
	fetcher := &recordingFetcher{err: fmt.Errorf("boom")}
	factory := &recordingFactory{fetcher: fetcher}
	handler := &stubConnectorHandler{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithEmailAccountLister(repo),
		WithConnectorFactory(factory),
		WithEmailHandler(handler),
	)

	job := &models.ScheduledJob{}
	if err := svc.handleEmailPoll(context.Background(), job); err == nil {
		t.Fatalf("expected error when fetcher fails")
	}
}

func TestHandleEmailPollRespectsWorkerCount(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubEmailRepository{
		accounts: []*models.EmailAccount{
			{ID: 1, Login: "agent", Host: "mail.example", AccountType: "POP3", PasswordEncrypted: "pw"},
			{ID: 2, Login: "ops", Host: "mail.ops", AccountType: "POP3", PasswordEncrypted: "pw"},
			{ID: 3, Login: "sales", Host: "mail.sales", AccountType: "POP3", PasswordEncrypted: "pw"},
			{ID: 4, Login: "vip", Host: "mail.vip", AccountType: "POP3", PasswordEncrypted: "pw"},
		},
	}
	fetcher := &workerTrackingFetcher{delay: 10 * time.Millisecond}
	factory := &recordingFactory{fetcher: fetcher}
	handler := &stubConnectorHandler{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithEmailAccountLister(repo),
		WithConnectorFactory(factory),
		WithEmailHandler(handler),
	)

	job := &models.ScheduledJob{Config: map[string]any{"worker_count": 2, "max_accounts": 4}}
	if err := svc.handleEmailPoll(context.Background(), job); err != nil {
		t.Fatalf("handleEmailPoll returned error: %v", err)
	}

	if fetcher.calls != len(repo.accounts) {
		t.Fatalf("expected %d fetch calls, got %d", len(repo.accounts), fetcher.calls)
	}
	if handler.Count() != len(repo.accounts) {
		t.Fatalf("expected handler invocations to match accounts, got %d", handler.Count())
	}
	if fetcher.maxConcurrent > 2 {
		t.Fatalf("expected max concurrency <= 2, got %d", fetcher.maxConcurrent)
	}
}

func TestHandleEmailPollRotatesAccounts(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubEmailRepository{
		accounts: []*models.EmailAccount{
			{ID: 1, Login: "a", Host: "mail.example", AccountType: "POP3", PasswordEncrypted: "pw"},
			{ID: 2, Login: "b", Host: "mail.example", AccountType: "POP3", PasswordEncrypted: "pw"},
			{ID: 3, Login: "c", Host: "mail.example", AccountType: "POP3", PasswordEncrypted: "pw"},
			{ID: 4, Login: "d", Host: "mail.example", AccountType: "POP3", PasswordEncrypted: "pw"},
		},
	}
	fetcher := &recordingFetcher{}
	factory := &recordingFactory{fetcher: fetcher}
	handler := &stubConnectorHandler{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithEmailAccountLister(repo),
		WithConnectorFactory(factory),
		WithEmailHandler(handler),
	)

	job := &models.ScheduledJob{Config: map[string]any{"max_accounts": 2, "worker_count": 1}}
	if err := svc.handleEmailPoll(context.Background(), job); err != nil {
		t.Fatalf("first handleEmailPoll returned error: %v", err)
	}
	if got := fetcher.CallAccountIDs(); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("first batch mismatch: %v", got)
	}
	fetcher.ResetCalls()

	if err := svc.handleEmailPoll(context.Background(), job); err != nil {
		t.Fatalf("second handleEmailPoll returned error: %v", err)
	}
	if got := fetcher.CallAccountIDs(); !reflect.DeepEqual(got, []int{3, 4}) {
		t.Fatalf("second batch mismatch: %v", got)
	}
	fetcher.ResetCalls()

	if err := svc.handleEmailPoll(context.Background(), job); err != nil {
		t.Fatalf("third handleEmailPoll returned error: %v", err)
	}
	if got := fetcher.CallAccountIDs(); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("third batch mismatch: %v", got)
	}
}

func TestHandleEmailPollSkipsThrottledAccounts(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubEmailRepository{
		accounts: []*models.EmailAccount{{
			ID:                  1,
			Login:               "a",
			Host:                "mail.example",
			AccountType:         "POP3",
			PasswordEncrypted:   "pw",
			PollIntervalSeconds: 600,
		}},
	}
	fetcher := &recordingFetcher{}
	factory := &recordingFactory{fetcher: fetcher}
	handler := &stubConnectorHandler{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithEmailAccountLister(repo),
		WithConnectorFactory(factory),
		WithEmailHandler(handler),
	)

	svc.emailPollState.mu.Lock()
	svc.emailPollState.lastPoll = map[int]time.Time{1: time.Now()}
	svc.emailPollState.mu.Unlock()

	job := &models.ScheduledJob{}
	if err := svc.handleEmailPoll(context.Background(), job); err != nil {
		t.Fatalf("handleEmailPoll returned error: %v", err)
	}
	if fetcher.CallCount() != 0 {
		t.Fatalf("expected throttled account to be skipped, got %d calls", fetcher.CallCount())
	}
}

func TestSelectEmailPollAccountsRespectsIntervals(t *testing.T) {
	svc := NewService(nil)
	accounts := []*models.EmailAccount{
		{ID: 1, PollIntervalSeconds: 60},
		{ID: 2, PollIntervalSeconds: 0},
		{ID: 3, PollIntervalSeconds: 15},
	}
	now := time.Now()
	svc.emailPollState.mu.Lock()
	svc.emailPollState.lastPoll = map[int]time.Time{
		1: now.Add(-30 * time.Second),
		3: now.Add(-20 * time.Second),
	}
	svc.emailPollState.mu.Unlock()

	selected := svc.selectEmailPollAccounts(accounts, 2, now)
	if got := emailAccountIDs(selected); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatalf("expected accounts 2 and 3, got %v", got)
	}

	svc.emailPollState.mu.Lock()
	svc.emailPollState.lastPoll[1] = now.Add(-90 * time.Second)
	svc.emailPollState.mu.Unlock()

	selected = svc.selectEmailPollAccounts(accounts, 1, now.Add(10*time.Second))
	if got := emailAccountIDs(selected); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("expected account 1 after interval elapsed, got %v", got)
	}
}

func TestRecordEmailPollResultThrottlesAccount(t *testing.T) {
	svc := NewService(nil)
	accounts := []*models.EmailAccount{{ID: 1, PollIntervalSeconds: 120}}

	svc.recordEmailPollResult(context.Background(), connector.Account{ID: 1}, 0, true, nil)

	selected := svc.selectEmailPollAccounts(accounts, 1, time.Now())
	if len(selected) != 0 {
		t.Fatalf("expected no eligible accounts immediately after poll, got %v", emailAccountIDs(selected))
	}
}

func emailAccountIDs(accounts []*models.EmailAccount) []int {
	ids := make([]int, len(accounts))
	for i, acc := range accounts {
		if acc != nil {
			ids[i] = acc.ID
		}
	}
	return ids
}

type recordingFactory struct {
	mu       sync.Mutex
	fetcher  connector.Fetcher
	accounts []connector.Account
}

func (f *recordingFactory) FetcherFor(account connector.Account) (connector.Fetcher, error) {
	f.mu.Lock()
	f.accounts = append(f.accounts, account)
	fetcher := f.fetcher
	f.mu.Unlock()
	if fetcher == nil {
		return nil, fmt.Errorf("no fetcher")
	}
	return fetcher, nil
}

type recordingFetcher struct {
	mu    sync.Mutex
	calls []connector.Account
	err   error
}

func (f *recordingFetcher) Name() string { return "recording" }

func (f *recordingFetcher) Fetch(ctx context.Context, account connector.Account, handler connector.Handler) error {
	f.mu.Lock()
	f.calls = append(f.calls, account)
	f.mu.Unlock()
	if handler != nil {
		_ = handler.Handle(ctx, &connector.FetchedMessage{})
	}
	return f.err
}

func (f *recordingFetcher) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *recordingFetcher) CallAccountIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]int, len(f.calls))
	for i, acc := range f.calls {
		ids[i] = acc.ID
	}
	return ids
}

func (f *recordingFetcher) ResetCalls() {
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
}

type stubConnectorHandler struct {
	mu    sync.Mutex
	count int
}

func (h *stubConnectorHandler) Handle(ctx context.Context, msg *connector.FetchedMessage) error {
	h.mu.Lock()
	h.count++
	h.mu.Unlock()
	return nil
}

func (h *stubConnectorHandler) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

type workerTrackingFetcher struct {
	delay         time.Duration
	mu            sync.Mutex
	active        int
	maxConcurrent int
	calls         int
}

func (f *workerTrackingFetcher) Name() string { return "worker-tracking" }

func (f *workerTrackingFetcher) Fetch(ctx context.Context, account connector.Account, handler connector.Handler) error {
	f.mu.Lock()
	f.active++
	if f.active > f.maxConcurrent {
		f.maxConcurrent = f.active
	}
	f.mu.Unlock()

	if handler != nil {
		_ = handler.Handle(ctx, &connector.FetchedMessage{})
	}
	time.Sleep(f.delay)

	f.mu.Lock()
	f.active--
	f.calls++
	f.mu.Unlock()
	return nil
}

func intPtr(v int) *int {
	return &v
}
