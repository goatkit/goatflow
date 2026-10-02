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

func newReceiver(t *testing.T) *receiver {
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
