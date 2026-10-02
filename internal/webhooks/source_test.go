package webhooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/webhook"
	"github.com/goatkit/goatflow/internal/repository"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("webhook dispatch tests need a test database (TEST_DB_HOST)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

type delivered struct {
	header http.Header
	body   []byte
}

type receiver struct {
	mu     sync.Mutex
	status int
	got    []delivered
	url    string
}

// newReceiver starts an endpoint on 127.0.0.1; delivering to it needs the
// private-target opt-in, which it sets for the test.
func newReceiver(t *testing.T) *receiver {
	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	r := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.got = append(r.got, delivered{header: req.Header.Clone(), body: body})
		status := r.status
		r.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	r.url = srv.URL
	return r
}

func (r *receiver) setStatus(code int) {
	r.mu.Lock()
	r.status = code
	r.mu.Unlock()
}

func (r *receiver) received() []delivered {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]delivered(nil), r.got...)
}

func createWebhook(t *testing.T, db *sql.DB, url, secret string, retryCount int, events ...string) *webhook.Webhook {
	t.Helper()
	w := &webhook.Webhook{
		Name:           fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano()),
		URL:            url,
		Events:         events,
		RetryCount:     retryCount,
		TimeoutSeconds: 5,
		IsActive:       true,
	}
	require.NoError(t, w.Normalize(IsEvent))
	created, err := webhook.NewRepository(db).Create(context.Background(), w, secret, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_webhook WHERE id = ?`), created.ID)
	})
	return created
}

func insertTicket(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, 1, 1, 1, 1, 3, 1, 'ACME', 'jane', 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1) RETURNING id`),
		fmt.Sprintf("WH%d", time.Now().UnixNano()), "Printer on fire", time.Now(), time.Now())
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM ticket_history WHERE ticket_id = ?`,
			`DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`,
			`DELETE FROM article WHERE ticket_id = ?`,
			`DELETE FROM ticket WHERE id = ?`,
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), id)
		}
	})
	return id
}

func insertArticle(t *testing.T, db *sql.DB, ticketID int64) int64 {
	t.Helper()
	now := time.Now()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
			create_time, create_by, change_time, change_by)
		VALUES (?, 3, 1, 1, ?, 1, ?, 1) RETURNING id`), ticketID, now, now)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, incoming_time,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, 0, ?, 1, ?, 1)`), id, "jane@example.com", "It is still burning", []byte("help"), now, now)
	require.NoError(t, err)
	return id
}

func addHistory(t *testing.T, db *sql.DB, ticketID int64, historyType string, stateID int) {
	t.Helper()
	err := repository.NewTicketRepository(db).AddTicketHistoryEntry(context.Background(), nil, models.TicketHistoryInsert{
		TicketID:    int(ticketID),
		TypeID:      1,
		QueueID:     1,
		OwnerID:     1,
		PriorityID:  3,
		StateID:     stateID,
		CreatedBy:   1,
		HistoryType: historyType,
		Name:        "%%" + historyType,
	})
	require.NoError(t, err)
}

func closedStateID(t *testing.T, db *sql.DB) int {
	t.Helper()
	var id int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT s.id FROM ticket_state s JOIN ticket_state_type st ON st.id = s.type_id
		WHERE st.name = 'closed' ORDER BY s.id LIMIT 1`)).Scan(&id))
	return id
}

// runOnce runs one dispatcher cycle and fails the test on error.
func runOnce(t *testing.T, svc *Service) (queued, attempted int) {
	t.Helper()
	queued, attempted, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	return queued, attempted
}

// settle runs the two cycles needed for rows written now to be published:
// the first moves the horizon past them, the second publishes and sends.
func settle(t *testing.T, svc *Service) {
	t.Helper()
	runOnce(t, svc)
	runOnce(t, svc)
}

// caughtUpService returns a service that has already published every row
// present now. The event cursors persist in gk_webhook_event_cursor, so rows
// written since the last dispatcher run (by earlier tests in a shared
// database) are a backlog the first cycles would publish to the webhooks the
// test is about to create. Call it before creating them.
func caughtUpService(t *testing.T, db *sql.DB) *Service {
	t.Helper()
	svc := NewService(db)
	settle(t, svc)
	return svc
}

type payload struct {
	Event string `json:"event"`
	Data  struct {
		Ticket  map[string]interface{} `json:"ticket"`
		Article map[string]interface{} `json:"article"`
		Change  map[string]interface{} `json:"change"`
	} `json:"data"`
}

func TestService_PublishesTicketEventsToSubscribers(t *testing.T) {
	db := testDB(t)
	svc := caughtUpService(t, db)
	recv := newReceiver(t)
	const secret = "dispatch-test-secret-1"
	createWebhook(t, db, recv.url, secret, 3, EventTicketCreated, EventArticleCreated, EventTicketClosed)
	other := newReceiver(t)
	createWebhook(t, db, other.url, "", 3, EventTicketQueueMoved)

	ticketID := insertTicket(t, db)
	articleID := insertArticle(t, db, ticketID)
	addHistory(t, db, ticketID, "AddNote", 1)                        // article history: no event
	addHistory(t, db, ticketID, "StateUpdate", closedStateID(t, db)) // closed
	addHistory(t, db, ticketID, "Move", 1)                           // other webhook only
	settle(t, svc)

	got := recv.received()
	require.Len(t, got, 3)
	var events []string
	for _, d := range got {
		assert.Equal(t, webhook.Sign(secret, d.body), d.header.Get(webhook.SignatureHeader))
		var p payload
		require.NoError(t, json.Unmarshal(d.body, &p))
		assert.Equal(t, p.Event, d.header.Get(webhook.EventHeader))
		assert.Equal(t, float64(ticketID), p.Data.Ticket["id"])
		events = append(events, p.Event)
	}
	assert.Equal(t, []string{EventTicketCreated, EventArticleCreated, EventTicketClosed}, events, "published in write order")

	var created, article, closed payload
	require.NoError(t, json.Unmarshal(got[0].body, &created))
	require.NoError(t, json.Unmarshal(got[1].body, &article))
	require.NoError(t, json.Unmarshal(got[2].body, &closed))
	assert.Equal(t, "Printer on fire", created.Data.Ticket["title"])
	assert.Equal(t, "ACME", created.Data.Ticket["customer_id"])
	assert.Equal(t, float64(articleID), article.Data.Article["id"])
	assert.Equal(t, "It is still burning", article.Data.Article["subject"])
	assert.Equal(t, "jane@example.com", article.Data.Article["from"])
	assert.Equal(t, "StateUpdate", closed.Data.Change["history_type"])

	otherGot := other.received()
	require.Len(t, otherGot, 1)
	var moved payload
	require.NoError(t, json.Unmarshal(otherGot[0].body, &moved))
	assert.Equal(t, EventTicketQueueMoved, moved.Event)
	assert.Empty(t, otherGot[0].header.Get(webhook.SignatureHeader), "unsigned without a secret")

	// Exactly once: further cycles publish nothing new.
	settle(t, svc)
	assert.Len(t, recv.received(), 3)
	assert.Len(t, other.received(), 1)
}

func TestService_InactiveWebhookGetsNothing(t *testing.T) {
	db := testDB(t)
	svc := caughtUpService(t, db)
	recv := newReceiver(t)
	wh := createWebhook(t, db, recv.url, "", 3, EventTicketCreated)
	repo := webhook.NewRepository(db)
	wh.IsActive = false
	_, err := repo.Update(context.Background(), wh, webhook.SecretChange{}, 1)
	require.NoError(t, err)

	insertTicket(t, db)
	settle(t, svc)
	assert.Empty(t, recv.received())
}

func deliveriesOf(t *testing.T, db *sql.DB, webhookID int64) []*webhook.Delivery {
	t.Helper()
	list, err := webhook.NewRepository(db).ListDeliveries(context.Background(), webhookID, 10)
	require.NoError(t, err)
	return list
}

// makeDue moves a pending retry into the past so the next cycle sends it.
func makeDue(t *testing.T, db *sql.DB, deliveryID int64) {
	t.Helper()
	_, err := db.Exec(database.ConvertPlaceholders(
		`UPDATE gk_webhook_delivery SET next_attempt_time = ? WHERE id = ?`),
		time.Now().UTC().Add(-time.Hour), deliveryID)
	require.NoError(t, err)
}

func TestService_RetriesFailedDeliveries(t *testing.T) {
	db := testDB(t)
	svc := caughtUpService(t, db)
	recv := newReceiver(t)
	recv.setStatus(http.StatusServiceUnavailable)
	retrying := createWebhook(t, db, recv.url, "", 2, EventTicketCreated)
	noRetry := newReceiver(t)
	noRetry.setStatus(http.StatusBadGateway)
	single := createWebhook(t, db, noRetry.url, "", 0, EventTicketCreated)

	insertTicket(t, db)
	settle(t, svc)

	// retry_count 0: one attempt, then failed.
	singleDeliveries := deliveriesOf(t, db, single.ID)
	require.Len(t, singleDeliveries, 1)
	assert.Equal(t, webhook.StatusFailed, singleDeliveries[0].Status)
	assert.Equal(t, 1, singleDeliveries[0].Attempts)
	assert.Nil(t, singleDeliveries[0].NextAttemptAt)

	// retry_count 2: first failure schedules a retry 30s later.
	ds := deliveriesOf(t, db, retrying.ID)
	require.Len(t, ds, 1)
	d := ds[0]
	assert.Equal(t, webhook.StatusPending, d.Status)
	assert.Equal(t, 1, d.Attempts)
	require.NotNil(t, d.StatusCode)
	assert.Equal(t, http.StatusServiceUnavailable, *d.StatusCode)
	require.NotNil(t, d.NextAttemptAt)
	assert.Equal(t, 30*time.Second, d.NextAttemptAt.Sub(d.UpdatedAt))

	// Not due yet: nothing is sent.
	runOnce(t, svc)
	assert.Len(t, recv.received(), 1)

	// Second failure doubles the backoff.
	makeDue(t, db, d.ID)
	runOnce(t, svc)
	ds = deliveriesOf(t, db, retrying.ID)
	assert.Equal(t, webhook.StatusPending, ds[0].Status)
	assert.Equal(t, 2, ds[0].Attempts)
	require.NotNil(t, ds[0].NextAttemptAt)
	assert.Equal(t, time.Minute, ds[0].NextAttemptAt.Sub(ds[0].UpdatedAt))

	// Endpoint recovers: the last allowed attempt succeeds.
	recv.setStatus(http.StatusOK)
	makeDue(t, db, d.ID)
	runOnce(t, svc)
	ds = deliveriesOf(t, db, retrying.ID)
	assert.Equal(t, webhook.StatusDelivered, ds[0].Status)
	assert.Equal(t, 3, ds[0].Attempts)
	assert.NotNil(t, ds[0].DeliveredAt)
	got := recv.received()
	require.Len(t, got, 3)
	assert.Equal(t, got[0].body, got[2].body, "retries resend the same payload")
	assert.Equal(t, fmt.Sprint(d.ID), got[2].header.Get(webhook.DeliveryHeader))
}

func TestService_RetryBudgetExhausted(t *testing.T) {
	db := testDB(t)
	svc := caughtUpService(t, db)
	recv := newReceiver(t)
	recv.setStatus(http.StatusInternalServerError)
	wh := createWebhook(t, db, recv.url, "", 1, EventTicketCreated)

	insertTicket(t, db)
	settle(t, svc)
	d := deliveriesOf(t, db, wh.ID)[0]
	require.Equal(t, webhook.StatusPending, d.Status)

	makeDue(t, db, d.ID)
	runOnce(t, svc)
	d = deliveriesOf(t, db, wh.ID)[0]
	assert.Equal(t, webhook.StatusFailed, d.Status)
	assert.Equal(t, 2, d.Attempts)
	assert.Nil(t, d.NextAttemptAt)
	assert.Equal(t, "endpoint returned HTTP 500", d.Error)
}

func TestService_PendingDeliveryOfDeactivatedWebhookFails(t *testing.T) {
	db := testDB(t)
	svc := caughtUpService(t, db)
	recv := newReceiver(t)
	recv.setStatus(http.StatusServiceUnavailable)
	wh := createWebhook(t, db, recv.url, "", 3, EventTicketCreated)

	insertTicket(t, db)
	settle(t, svc)
	d := deliveriesOf(t, db, wh.ID)[0]
	require.Equal(t, webhook.StatusPending, d.Status)

	wh.IsActive = false
	_, err := webhook.NewRepository(db).Update(context.Background(), wh, webhook.SecretChange{}, 1)
	require.NoError(t, err)
	makeDue(t, db, d.ID)
	runOnce(t, svc)
	d = deliveriesOf(t, db, wh.ID)[0]
	assert.Equal(t, webhook.StatusFailed, d.Status)
	assert.Equal(t, "webhook is inactive", d.Error)
	assert.Len(t, recv.received(), 1)
}

// insertDeliveries queues n deliveries for a webhook, created at createdAt
// with the given status, and returns their ids. Pending ones are due now.
func insertDeliveries(t *testing.T, db *sql.DB, webhookID int64, n int, status string, createdAt time.Time) []int64 {
	t.Helper()
	next := time.Now().UTC().Add(-time.Minute)
	ids := make([]int64, n)
	for i := range ids {
		id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO gk_webhook_delivery (webhook_id, event_type, payload, status, attempts,
				next_attempt_time, create_time, change_time)
			VALUES (?, ?, ?, ?, 0, ?, ?, ?) RETURNING id`),
			webhookID, EventTicketCreated, `{"event":"ticket.created"}`, status, next, createdAt, createdAt)
		require.NoError(t, err)
		ids[i] = id
	}
	return ids
}

func deliveryStatuses(t *testing.T, db *sql.DB, webhookID int64) map[string]int {
	t.Helper()
	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT status, COUNT(*) FROM gk_webhook_delivery WHERE webhook_id = ? GROUP BY status`), webhookID)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		require.NoError(t, rows.Scan(&status, &n))
		out[status] = n
	}
	require.NoError(t, rows.Err())
	return out
}

// TestService_SlowEndpointDoesNotStarveOthers: a webhook whose endpoint never
// answers in time must not hold up other webhooks' deliveries, even when its
// deliveries are older, and a run that ends must leave unsent deliveries
// pending rather than claimed.
func TestService_SlowEndpointDoesNotStarveOthers(t *testing.T) {
	db := testDB(t)
	svc := caughtUpService(t, db)
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); slow.Close() })
	fast := newReceiver(t)

	slowHook := createWebhook(t, db, slow.URL, "", 0, EventTicketCreated)
	_, err := db.Exec(database.ConvertPlaceholders(`UPDATE gk_webhook SET timeout_seconds = 1 WHERE id = ?`), slowHook.ID)
	require.NoError(t, err)
	fastHook := createWebhook(t, db, fast.url, "", 0, EventTicketCreated)
	insertDeliveries(t, db, slowHook.ID, 4, webhook.StatusPending, time.Now().UTC())
	insertDeliveries(t, db, fastHook.ID, 4, webhook.StatusPending, time.Now().UTC())

	// Four 1-second timeouts do not fit in 2.5 seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	_, _, runErr := svc.RunOnce(ctx)

	assert.Len(t, fast.received(), 4, "the fast endpoint gets all its deliveries")
	assert.Equal(t, map[string]int{webhook.StatusDelivered: 4}, deliveryStatuses(t, db, fastHook.ID))
	slowStatuses := deliveryStatuses(t, db, slowHook.ID)
	assert.Zero(t, slowStatuses[webhook.StatusDelivering], "no delivery is left claimed: %v", slowStatuses)
	assert.Positive(t, slowStatuses[webhook.StatusFailed], "slow deliveries were attempted: %v", slowStatuses)
	assert.Positive(t, slowStatuses[webhook.StatusPending], "deliveries not reached stay pending: %v", slowStatuses)
	assert.Equal(t, 4, slowStatuses[webhook.StatusFailed]+slowStatuses[webhook.StatusPending])
	assert.NoError(t, runErr, "a run that runs out of time is not an error")
}

// TestService_RunEndingMidAttemptKeepsDeliveryPending: when the run ends
// while an endpoint still has time to answer, the attempt was cut short by
// GoatFlow, not failed by the endpoint. The delivery stays pending, due at
// once, without using up an attempt.
func TestService_RunEndingMidAttemptKeepsDeliveryPending(t *testing.T) {
	db := testDB(t)
	svc := caughtUpService(t, db)
	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); slow.Close() })

	wh := createWebhook(t, db, slow.URL, "", 0, EventTicketCreated)
	_, err := db.Exec(database.ConvertPlaceholders(`UPDATE gk_webhook SET timeout_seconds = 30 WHERE id = ?`), wh.ID)
	require.NoError(t, err)
	insertDeliveries(t, db, wh.ID, 1, webhook.StatusPending, time.Now().UTC())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, runErr := svc.RunOnce(ctx)
	require.NoError(t, runErr)

	d := deliveriesOf(t, db, wh.ID)[0]
	assert.Equal(t, webhook.StatusPending, d.Status, "no retries left, yet the endpoint never had its full time")
	assert.Zero(t, d.Attempts, "the cut-short attempt is not counted")
	assert.Equal(t, "run ended before the endpoint answered; will try again", d.Error)
	require.NotNil(t, d.NextAttemptAt)
	assert.False(t, d.NextAttemptAt.After(time.Now().UTC()), "due again at once")
}

// TestService_PrunesOldDeliveries: delivered and failed deliveries older than
// the retention (default 30 days) are deleted; pending ones are kept, and a
// retention of 0 keeps everything.
func TestService_PrunesOldDeliveries(t *testing.T) {
	db := testDB(t)
	recv := newReceiver(t)
	wh := createWebhook(t, db, recv.url, "", 3, EventTicketCreated)
	now := time.Now().UTC()
	old, recent := now.Add(-40*24*time.Hour), now.Add(-2*24*time.Hour)

	oldDelivered := insertDeliveries(t, db, wh.ID, 2, webhook.StatusDelivered, old)
	oldFailed := insertDeliveries(t, db, wh.ID, 1, webhook.StatusFailed, old)
	oldPending := insertDeliveries(t, db, wh.ID, 1, webhook.StatusPending, old)
	_, err := db.Exec(database.ConvertPlaceholders(
		`UPDATE gk_webhook_delivery SET next_attempt_time = ? WHERE id = ?`), now.Add(time.Hour), oldPending[0])
	require.NoError(t, err)
	recentDelivered := insertDeliveries(t, db, wh.ID, 1, webhook.StatusDelivered, recent)
	ids := func() []int64 {
		var out []int64
		for _, d := range deliveriesOf(t, db, wh.ID) {
			out = append(out, d.ID)
		}
		return out
	}
	require.Len(t, ids(), len(oldDelivered)+len(oldFailed)+1+1)

	t.Setenv(RetentionDaysEnv, "")
	runOnce(t, NewService(db))
	assert.ElementsMatch(t, []int64{oldPending[0], recentDelivered[0]}, ids(), "default retention is 30 days")

	t.Setenv(RetentionDaysEnv, "0")
	keptOld := insertDeliveries(t, db, wh.ID, 1, webhook.StatusDelivered, old)
	runOnce(t, NewService(db))
	assert.ElementsMatch(t, []int64{oldPending[0], recentDelivered[0], keptOld[0]}, ids(), "0 keeps deliveries forever")

	t.Setenv(RetentionDaysEnv, "1")
	runOnce(t, NewService(db))
	assert.ElementsMatch(t, []int64{oldPending[0]}, ids(), "pending deliveries are never pruned")
}
