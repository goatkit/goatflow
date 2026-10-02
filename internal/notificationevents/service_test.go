package notificationevents

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime"
	"net/mail"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("notification event tests need a test database (TEST_DB_HOST)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

var seq int64

// uniq returns a name unique to this test run.
func uniq(prefix string) string {
	seq++
	return fmt.Sprintf("%s%d%d", prefix, time.Now().UnixNano()%1e9, seq)
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(database.ConvertPlaceholders(query), args...)
	require.NoError(t, err, query)
}

// mustID unwraps InsertWithReturning's result.
func mustID(t *testing.T) func(int64, error) int {
	return func(id int64, err error) int {
		t.Helper()
		require.NoError(t, err)
		return int(id)
	}
}

// fixture is a queue in its own group, so permission checks see only the
// agents a test grants access.
type fixture struct {
	db      *sql.DB
	groupID int
	queueID int
	queue   string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testDB(t)
	now := time.Now()
	f := &fixture{db: db, queue: uniq("NEQueue")}
	f.groupID = mustID(t)(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO `+"`groups`"+` (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, ?, 1, ?, 1) RETURNING id`), uniq("negroup"), now, now))
	t.Cleanup(func() { exec(t, db, "DELETE FROM `groups` WHERE id = ?", f.groupID) })
	f.queueID = mustID(t)(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id,
		follow_up_id, follow_up_lock, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, 1, 1, 1, 0, 1, ?, 1, ?, 1) RETURNING id`), f.queue, f.groupID, now, now))
	t.Cleanup(func() { exec(t, db, `DELETE FROM queue WHERE id = ?`, f.queueID) })
	return f
}

type agent struct {
	id    int
	login string
	email string
}

// agent creates a valid agent with an email address, a language preference
// and (when perm is not empty) that permission on the fixture's group.
func (f *fixture) agent(t *testing.T, first, lang, perm string) agent {
	t.Helper()
	now := time.Now()
	a := agent{login: uniq("neagent")}
	a.email = a.login + "@agents.example.com"
	a.id = mustID(t)(database.GetAdapter().InsertWithReturning(f.db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', ?, 'Agent', 1, ?, 1, ?, 1) RETURNING id`), a.login, first, now, now))
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM group_user WHERE user_id = ?`,
			`DELETE FROM personal_queues WHERE user_id = ?`,
			`DELETE FROM user_preferences WHERE user_id = ?`,
			`DELETE FROM users WHERE id = ?`,
		} {
			exec(t, f.db, q, a.id)
		}
	})
	exec(t, f.db, `INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, 'UserEmail', ?)`,
		a.id, []byte(a.email))
	if lang != "" {
		exec(t, f.db, `INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, 'Language', ?)`,
			a.id, []byte(lang))
	}
	if perm != "" {
		exec(t, f.db, `INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, ?, 1)`, a.id, f.groupID, perm, now, now)
	}
	return a
}

// customer creates a customer user and returns its login and email.
func (f *fixture) customer(t *testing.T, first string) (string, string) {
	t.Helper()
	now := time.Now()
	login := uniq("necust")
	email := login + "@customers.example.com"
	exec(t, f.db, `INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id,
		create_time, create_by, change_time, change_by) VALUES (?, ?, 'ACME', ?, 'Customer', 1, ?, 1, ?, 1)`,
		login, email, first, now, now)
	t.Cleanup(func() { exec(t, f.db, `DELETE FROM customer_user WHERE login = ?`, login) })
	return login, email
}

type ruleSpec struct {
	validID     int
	items       map[string][]string // includes "Events"
	messages    map[string][2]string
	contentType string
}

func (f *fixture) rule(t *testing.T, spec ruleSpec) int {
	t.Helper()
	if spec.validID == 0 {
		spec.validID = 1
	}
	if spec.contentType == "" {
		spec.contentType = "text/plain"
	}
	now := time.Now()
	id := mustID(t)(database.GetAdapter().InsertWithReturning(f.db, database.ConvertPlaceholders(`
		INSERT INTO notification_event (name, valid_id, comments, create_time, create_by, change_time, change_by)
		VALUES (?, ?, '', ?, 1, ?, 1) RETURNING id`), uniq("Rule "), spec.validID, now, now))
	t.Cleanup(func() {
		exec(t, f.db, `DELETE FROM notification_event_message WHERE notification_id = ?`, id)
		exec(t, f.db, `DELETE FROM notification_event_item WHERE notification_id = ?`, id)
		exec(t, f.db, `DELETE FROM notification_event WHERE id = ?`, id)
	})
	for key, values := range spec.items {
		for _, v := range values {
			exec(t, f.db, `INSERT INTO notification_event_item (notification_id, event_key, event_value) VALUES (?, ?, ?)`, id, key, v)
		}
	}
	for lang, m := range spec.messages {
		exec(t, f.db, `INSERT INTO notification_event_message (notification_id, subject, text, content_type, language)
			VALUES (?, ?, ?, ?, ?)`, id, m[0], m[1], spec.contentType, lang)
	}
	return id
}

// ticket inserts a ticket in the fixture's queue created by createBy.
func (f *fixture) ticket(t *testing.T, title string, ownerID int, customerUserID string, createBy int) int64 {
	t.Helper()
	now := time.Now()
	id := mustID(t)(database.GetAdapter().InsertWithReturning(f.db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 1, 1, ?, ?, 3, 1, 'ACME', ?, 0, 0, 0, 0, 0, 0, 0, ?, ?, ?, ?) RETURNING id`),
		uniq("NE"), title, f.queueID, ownerID, ownerID, customerUserID, now, createBy, now, createBy))
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM ticket_history WHERE ticket_id = ?`,
			`DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`,
			`DELETE FROM article WHERE ticket_id = ?`,
			`DELETE FROM ticket WHERE id = ?`,
		} {
			exec(t, f.db, q, id)
		}
	})
	return int64(id)
}

func (f *fixture) article(t *testing.T, ticketID int64, senderTypeID, createBy int, subject, body string) int64 {
	t.Helper()
	now := time.Now()
	id := mustID(t)(database.GetAdapter().InsertWithReturning(f.db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
		is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, 1, ?, ?, ?, ?) RETURNING id`), ticketID, senderTypeID, now, createBy, now, createBy))
	exec(t, f.db, `INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type, incoming_time,
		create_time, create_by, change_time, change_by) VALUES (?, 'someone@example.com', ?, ?, 'text/plain', 0, ?, 1, ?, 1)`,
		id, subject, []byte(body), now, now)
	return int64(id)
}

func (f *fixture) history(t *testing.T, ticketID int64, historyType string, stateID, createBy int) {
	t.Helper()
	err := repository.NewTicketRepository(f.db).AddTicketHistoryEntry(context.Background(), nil, models.TicketHistoryInsert{
		TicketID: int(ticketID), TypeID: 1, QueueID: f.queueID, OwnerID: 1, PriorityID: 3, StateID: stateID,
		CreatedBy: createBy, HistoryType: historyType, Name: "%%" + historyType,
	})
	require.NoError(t, err)
}

type sent struct {
	to      string
	subject string
	body    string
	header  mail.Header
}

// mails returns the mail_queue rows addressed to addrs, sorted by recipient,
// and deletes them when the test ends.
func (f *fixture) mails(t *testing.T, addrs ...string) []sent {
	t.Helper()
	args := make([]any, len(addrs))
	marks := make([]string, len(addrs))
	for i, a := range addrs {
		args[i] = a
		marks[i] = "?"
	}
	where := ` WHERE recipient IN (` + strings.Join(marks, ", ") + `)`
	t.Cleanup(func() { exec(t, f.db, `DELETE FROM mail_queue`+where, args...) })
	rows, err := f.db.Query(database.ConvertPlaceholders(`SELECT recipient, raw_message FROM mail_queue`+where+` ORDER BY id`), args...)
	require.NoError(t, err)
	defer rows.Close()
	var out []sent
	dec := new(mime.WordDecoder)
	for rows.Next() {
		var (
			s   sent
			raw []byte
		)
		require.NoError(t, rows.Scan(&s.to, &raw))
		msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
		require.NoError(t, err)
		body, err := io.ReadAll(msg.Body)
		require.NoError(t, err)
		s.header = msg.Header
		s.subject, err = dec.DecodeHeader(msg.Header.Get("Subject"))
		require.NoError(t, err)
		s.body = string(body)
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	sort.SliceStable(out, func(i, j int) bool { return out[i].to < out[j].to })
	return out
}

func quietLogger() *log.Logger { return log.New(io.Discard, "", 0) }

// settle runs the two cycles needed for rows written now to be evaluated:
// the first moves the horizon past them, the second evaluates them.
func settle(t *testing.T, svc *Service) int {
	t.Helper()
	total := 0
	for range 2 {
		n, err := svc.RunOnce(context.Background())
		require.NoError(t, err)
		total += n
	}
	return total
}

// caughtUp returns a service that has evaluated every row present now. The
// cursors persist in gk_notification_event_cursor, so rows written by
// earlier tests would otherwise be evaluated against this test's rules.
func caughtUp(t *testing.T, db *sql.DB) *Service {
	t.Helper()
	svc := NewService(db, quietLogger())
	settle(t, svc)
	return svc
}

func historyCount(t *testing.T, db *sql.DB, ticketID int64, historyType string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM ticket_history th JOIN ticket_history_type tht ON tht.id = th.history_type_id
		WHERE th.ticket_id = ? AND tht.name = ?`), ticketID, historyType).Scan(&n))
	return n
}

func TestTicketCreateNotifiesOwnerCustomerAndAddressesInTheirLanguage(t *testing.T) {
	f := newFixture(t)
	svc := caughtUp(t, f.db)
	owner := f.agent(t, "Olga", "de", "rw")
	creator := f.agent(t, "Carl", "", "rw")
	custLogin, custEmail := f.customer(t, "Jane")
	f.rule(t, ruleSpec{
		items: map[string][]string{
			"Events":         {"TicketCreate"},
			"QueueID":        {fmt.Sprint(f.queueID)},
			"Recipients":     {"AgentOwner", "Customer"},
			"RecipientEmail": {"Leads@Example.com, oncall@example.com"},
		},
		messages: map[string][2]string{
			"en": {"[<OTRS_TICKET_TicketNumber>] New: <OTRS_TICKET_Title>",
				"Hello <OTRS_NOTIFICATION_RECIPIENT_UserFirstname>,\nqueue <OTRS_TICKET_Queue>, customer <OTRS_CUSTOMER_DATA_UserFirstname>, by <OTRS_CURRENT_UserFirstname>.\n<OTRS_CUSTOMER_BODY[1]>\n<OTRS_CONFIG_Unknown>"},
			"de": {"Neu: <OTRS_TICKET_Title>", "Hallo <OTRS_NOTIFICATION_RECIPIENT_UserFirstname>"},
		},
	})

	ticketID := f.ticket(t, "Printer on fire", owner.id, custLogin, creator.id)
	f.article(t, ticketID, 3, 1, "Help", "first line\nsecond line")
	assert.Equal(t, 4, settle(t, svc))

	got := f.mails(t, owner.email, custEmail, "Leads@Example.com", "oncall@example.com")
	require.Len(t, got, 4)
	byTo := map[string]sent{}
	for _, s := range got {
		byTo[s.to] = s
		assert.Equal(t, "auto-generated", s.header.Get("Auto-Submitted"))
		assert.Contains(t, s.header.Get("Content-Type"), "text/plain")
	}

	de := byTo[owner.email]
	assert.Equal(t, "Neu: Printer on fire", de.subject, "owner prefers German")
	assert.Equal(t, "Hallo Olga", de.body)

	var tn string
	require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(`SELECT tn FROM ticket WHERE id = ?`), ticketID).Scan(&tn))
	en := byTo[custEmail]
	assert.Equal(t, "["+tn+"] New: Printer on fire", en.subject)
	assert.Equal(t, "Hello Jane,\nqueue "+f.queue+", customer Jane, by Carl.\nfirst line\n-", en.body)

	extra := byTo["Leads@Example.com"]
	assert.True(t, strings.HasPrefix(extra.body, "Hello ,"), "plain addresses have no name: %q", extra.body)
	assert.Contains(t, byTo, "oncall@example.com")

	assert.Equal(t, 3, historyCount(t, f.db, ticketID, "SendAgentNotification"), "owner and the two addresses")
	assert.Equal(t, 1, historyCount(t, f.db, ticketID, "SendCustomerNotification"))

	// Exactly once: later cycles queue nothing for the same rows.
	assert.Equal(t, 0, settle(t, svc))
	assert.Len(t, f.mails(t, owner.email, custEmail, "Leads@Example.com", "oncall@example.com"), 4)
}

func TestFiltersValidityActorAndPermissionsLimitRecipients(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	svc := caughtUp(t, f.db)
	owner := f.agent(t, "Owner", "", "rw")
	reader := f.agent(t, "Reader", "", "ro")
	outsider := f.agent(t, "Outsider", "", "")
	en := map[string][2]string{"en": {"S <OTRS_TICKET_Title>", "B"}}

	// Fires: matches queue; outsider has no access to the queue.
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"TicketCreate"}, "QueueID": {fmt.Sprint(f.queueID)},
		"RecipientAgents": {fmt.Sprint(reader.id), fmt.Sprint(outsider.id)},
	}, messages: en})
	// Never fires: other queue, invalid rule, unsupported filter.
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"TicketCreate"}, "QueueID": {fmt.Sprint(other.queueID)}, "RecipientAgents": {fmt.Sprint(reader.id)},
	}, messages: en})
	f.rule(t, ruleSpec{validID: 2, items: map[string][]string{
		"Events": {"TicketCreate"}, "RecipientAgents": {fmt.Sprint(reader.id)},
	}, messages: en})
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"TicketCreate"}, "DynamicField_Product": {"x"}, "RecipientAgents": {fmt.Sprint(reader.id)},
	}, messages: en})
	// Owner caused the event: not notified about their own action.
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"TicketCreate"}, "Recipients": {"AgentOwner"},
	}, messages: en})

	f.ticket(t, "Filtered", owner.id, "", owner.id)
	settle(t, svc)

	got := f.mails(t, owner.email, reader.email, outsider.email)
	require.Len(t, got, 1)
	assert.Equal(t, reader.email, got[0].to)
	assert.Equal(t, "S Filtered", got[0].subject)
}

func TestHistoryEventsAndGroupRecipients(t *testing.T) {
	f := newFixture(t)
	svc := caughtUp(t, f.db)
	owner := f.agent(t, "Owner", "", "rw")
	writer := f.agent(t, "Writer", "", "rw")
	reader := f.agent(t, "Reader", "", "ro")
	watcherOfQueue := f.agent(t, "Mine", "", "ro")
	exec(t, f.db, `INSERT INTO personal_queues (user_id, queue_id) VALUES (?, ?)`, watcherOfQueue.id, f.queueID)
	en := func(s string) map[string][2]string { return map[string][2]string{"en": {s, "<OTRS_TICKET_State>"}} }

	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"TicketStateUpdate"}, "Recipients": {"AgentWritePermissions"},
	}, messages: en("state")})
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"NotificationMove"}, "Recipients": {"AgentMyQueues"},
	}, messages: en("moved")})
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"TicketOwnerUpdate"}, "RecipientGroups": {fmt.Sprint(f.groupID)},
	}, messages: en("owner")})

	ticketID := f.ticket(t, "History", owner.id, "", 1)
	settle(t, svc) // TicketCreate: no rule
	f.history(t, ticketID, "StateUpdate", 2, owner.id)
	f.history(t, ticketID, "Move", 1, owner.id)
	f.history(t, ticketID, "OwnerUpdate", 1, writer.id)
	f.history(t, ticketID, "AddNote", 1, owner.id) // no rule
	settle(t, svc)

	subjects := map[string][]string{}
	for _, s := range f.mails(t, owner.email, writer.email, reader.email, watcherOfQueue.email) {
		subjects[s.to] = append(subjects[s.to], s.subject)
	}
	assert.Equal(t, map[string][]string{
		// state: rw agents except the actor (owner); owner update: group
		// members except the actor (writer); move: My Queues.
		writer.email:         {"state"},
		owner.email:          {"owner"},
		reader.email:         {"owner"},
		watcherOfQueue.email: {"moved", "owner"},
	}, subjects)
}

func TestEscalationMetaEventFiresOncePerCheck(t *testing.T) {
	f := newFixture(t)
	svc := caughtUp(t, f.db)
	owner := f.agent(t, "Owner", "", "rw")
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"NotificationEscalation"}, "Recipients": {"AgentOwner"},
	}, messages: map[string][2]string{"en": {"escalated <OTRS_EVENT>", "-"}}})
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"EscalationSolutionTimeStart"}, "Recipients": {"AgentOwner"},
	}, messages: map[string][2]string{"en": {"solution <OTRS_EVENT>", "-"}}})

	ticketID := f.ticket(t, "Late", owner.id, "", 1)
	settle(t, svc)
	for _, h := range []string{"EscalationResponseTimeStart", "EscalationUpdateTimeStart", "EscalationSolutionTimeStart"} {
		f.history(t, ticketID, h, 1, 1)
	}
	settle(t, svc)

	var subjects []string
	for _, s := range f.mails(t, owner.email) {
		subjects = append(subjects, s.subject)
	}
	assert.ElementsMatch(t, []string{"escalated NotificationEscalation", "solution EscalationSolutionTimeStart"}, subjects)
}

func TestArticleFiltersAndHTMLEscaping(t *testing.T) {
	f := newFixture(t)
	svc := caughtUp(t, f.db)
	owner := f.agent(t, "Owner", "", "rw")
	f.rule(t, ruleSpec{
		items: map[string][]string{
			"Events": {"ArticleCreate"}, "ArticleSenderTypeID": {"3"}, "ArticleSubjectMatch": {"URGENT"},
			"Recipients": {"AgentOwner"},
		},
		messages:    map[string][2]string{"en": {"Re: <OTRS_CUSTOMER_SUBJECT>", "<p><OTRS_TICKET_Title></p>&lt;OTRS_CUSTOMER_BODY&gt;"}},
		contentType: "text/html",
	})

	ticketID := f.ticket(t, `<script>alert(1)</script>`, owner.id, "", 1)
	settle(t, svc)
	f.article(t, ticketID, 1, 1, "urgent from agent", "agent text") // agent: filtered out
	f.article(t, ticketID, 3, 1, "routine", "no")                   // subject does not match
	f.article(t, ticketID, 3, 1, "This is urgent", "a <b>\nb")
	settle(t, svc)

	got := f.mails(t, owner.email)
	require.Len(t, got, 1)
	assert.Equal(t, "Re: This is urgent", got[0].subject)
	assert.Contains(t, got[0].header.Get("Content-Type"), "text/html")
	assert.Equal(t, "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>a &lt;b&gt;<br/>\nb", got[0].body)
}

func TestSubjectCannotInjectHeaders(t *testing.T) {
	f := newFixture(t)
	svc := caughtUp(t, f.db)
	owner := f.agent(t, "Owner", "", "rw")
	f.rule(t, ruleSpec{items: map[string][]string{"Events": {"ArticleCreate"}, "Recipients": {"AgentOwner"}},
		messages: map[string][2]string{"en": {"<OTRS_CUSTOMER_SUBJECT>", "x"}}})
	ticketID := f.ticket(t, "T", owner.id, "", 1)
	settle(t, svc)
	f.article(t, ticketID, 3, 1, "hi\r\nBcc: victim@example.com", "x")
	settle(t, svc)

	got := f.mails(t, owner.email)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].header.Get("Bcc"))
	assert.Equal(t, "hi Bcc: victim@example.com", got[0].subject)
}

func TestOncePerDay(t *testing.T) {
	f := newFixture(t)
	svc := caughtUp(t, f.db)
	owner := f.agent(t, "Owner", "", "rw")
	f.rule(t, ruleSpec{items: map[string][]string{
		"Events": {"TicketStateUpdate"}, "Recipients": {"AgentOwner"}, "OncePerDay": {"1"},
	}, messages: map[string][2]string{"en": {"s", "b"}}})
	ticketID := f.ticket(t, "Daily", owner.id, "", 1)
	settle(t, svc)
	f.history(t, ticketID, "StateUpdate", 2, 1)
	f.history(t, ticketID, "StateUpdate", 1, 1)
	settle(t, svc)
	f.history(t, ticketID, "StateUpdate", 2, 1)
	settle(t, svc)
	assert.Len(t, f.mails(t, owner.email), 1)
}

func TestConcurrentServicesQueueEachNotificationOnce(t *testing.T) {
	f := newFixture(t)
	caughtUp(t, f.db)
	owner := f.agent(t, "Owner", "", "rw")
	f.rule(t, ruleSpec{items: map[string][]string{"Events": {"TicketCreate"}, "Recipients": {"AgentOwner"}},
		messages: map[string][2]string{"en": {"s", "b"}}})
	svcs := []*Service{NewService(f.db, quietLogger()), NewService(f.db, quietLogger()), NewService(f.db, quietLogger())}
	for _, s := range svcs {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
	}
	for i := range 5 {
		f.ticket(t, fmt.Sprint("C", i), owner.id, "", 1)
	}
	var wg sync.WaitGroup
	for _, s := range svcs {
		wg.Add(1)
		go func(s *Service) {
			defer wg.Done()
			for range 2 {
				_, err := s.RunOnce(context.Background())
				assert.NoError(t, err)
			}
		}(s)
	}
	wg.Wait()
	for _, s := range svcs {
		settle(t, s)
	}
	assert.Len(t, f.mails(t, owner.email), 5)
}

func TestValidateRecipientEmail(t *testing.T) {
	assert.NoError(t, ValidateRecipientEmail("a@example.com, b@example.com;c@example.com"))
	assert.Error(t, ValidateRecipientEmail("a@example.com, not-an-address"))
	assert.Error(t, ValidateRecipientEmail("Jane <jane@example.com>"))
	assert.Error(t, ValidateRecipientEmail(strings.Repeat("a", 190)+"@example.com"))
}
