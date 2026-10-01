package search

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// searchFixture holds rows seeded for one test run. Every searchable value
// carries a per-run marker so rows from other tests never match.
type searchFixture struct {
	marker                         string
	ticketExact, ticketTitle       int64 // title == marker; title contains marker
	ticketBody                     int64 // marker only in an article body
	ticketBodyTN, ticketTitleTN    string
	articleExact, articleSubj      int64 // subject == marker; subject contains marker
	articleBody                    int64 // marker only in body
	customerLogin, customerName    int64 // login == marker; name "Ann Lee<marker>"
	customerNameLogin, companyName string
}

func getSearchTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if err := database.InitTestDB(); err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Test database not available")
	}
	return db
}

func seedSearchFixture(t *testing.T, db *sql.DB) *searchFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	run := strconv.FormatInt(time.Now().UnixNano(), 36)
	marker := "srch" + run
	tnPrefix := "T" + run // ticket numbers must not contain the marker
	f := &searchFixture{
		marker:            marker,
		ticketTitleTN:     tnPrefix + "2",
		ticketBodyTN:      tnPrefix + "3",
		customerNameLogin: marker + ".ann",
		companyName:       "Acme " + marker,
	}
	adapter := database.GetAdapter()

	t.Cleanup(func() {
		for _, c := range []struct{ q, pattern string }{
			{"DELETE FROM article_data_mime WHERE article_id IN (SELECT a.id FROM article a JOIN ticket t ON t.id = a.ticket_id WHERE t.tn LIKE ?)", tnPrefix + "%"},
			{"DELETE FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE tn LIKE ?)", tnPrefix + "%"},
			{"DELETE FROM ticket WHERE tn LIKE ?", tnPrefix + "%"},
			{"DELETE FROM customer_user WHERE login LIKE ?", marker + "%"},
			{"DELETE FROM customer_company WHERE customer_id LIKE ?", marker + "%"},
		} {
			if _, err := db.Exec(database.ConvertPlaceholders(c.q), c.pattern); err != nil {
				t.Errorf("cleanup %q: %v", c.q, err)
			}
		}
	})

	insertTicket := func(tn, title string) int64 {
		id, err := adapter.InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id,
				ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
				user_id, responsible_user_id, timeout, until_time, escalation_time,
				escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, 1, 1, 1, 3, '', '', 1, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)
			RETURNING id`), tn, title, now, now)
		require.NoError(t, err, "seed ticket %s", tn)
		return id
	}
	insertArticle := func(ticketID int64, subject, body string) int64 {
		id, err := adapter.InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer, create_time, create_by, change_time, change_by)
			VALUES (?, 3, 1, 1, ?, 1, ?, 1)
			RETURNING id`), ticketID, now, now)
		require.NoError(t, err, "seed article")
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, incoming_time,
				create_time, create_by, change_time, change_by)
			VALUES (?, 'customer@example.com', ?, ?, 0, ?, 1, ?, 1)`),
			id, subject, body, now, now)
		require.NoError(t, err, "seed article_data_mime")
		return id
	}
	insertCustomer := func(login, customerID, first, last, email string) int64 {
		id, err := adapter.InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO customer_user (login, email, customer_id, first_name, last_name,
				valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, 1, ?, 1, ?, 1)
			RETURNING id`), login, email, customerID, first, last, now, now)
		require.NoError(t, err, "seed customer_user %s", login)
		return id
	}

	f.ticketExact = insertTicket(tnPrefix+"1", marker)
	f.ticketTitle = insertTicket(f.ticketTitleTN, "Printer "+marker)
	f.ticketBody = insertTicket(f.ticketBodyTN, "Unrelated request")

	f.articleExact = insertArticle(f.ticketBody, marker, "hello")
	f.articleSubj = insertArticle(f.ticketBody, "About "+marker, "nothing to see")
	f.articleBody = insertArticle(f.ticketBody, "Hello", "the toner "+marker+" is empty")

	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_company (customer_id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, ?, 1, ?, 1)`), marker+"co", f.companyName, now, now)
	require.NoError(t, err, "seed customer_company")
	f.customerLogin = insertCustomer(marker, marker+"co", "Bob", "Jones", marker+"@example.com")
	f.customerName = insertCustomer(f.customerNameLogin, marker+"co2", "Ann", "Lee"+marker, "ann.lee@example.com")

	return f
}

type hitRef struct {
	Type  string
	ID    string
	Score float64
}

func refs(hits []SearchHit) []hitRef {
	out := make([]hitRef, 0, len(hits))
	for _, h := range hits {
		out = append(out, hitRef{h.Type, h.ID, h.Score})
	}
	return out
}

func idStr(n int64) string { return strconv.FormatInt(n, 10) }

func TestDatabaseBackendSearch(t *testing.T) {
	db := getSearchTestDB(t)
	f := seedSearchFixture(t, db)
	backend, err := NewDatabaseBackend()
	require.NoError(t, err)
	ctx := context.Background()

	search := func(t *testing.T, q SearchQuery) *SearchResults {
		t.Helper()
		res, err := backend.Search(ctx, q)
		require.NoError(t, err)
		return res
	}

	t.Run("tickets rank exact title over title match over article-only match", func(t *testing.T) {
		res := search(t, SearchQuery{Query: f.marker, Types: []string{"ticket"}, Highlight: true})
		assert.Equal(t, []hitRef{
			{"ticket", idStr(f.ticketExact), 3},
			{"ticket", idStr(f.ticketTitle), 2},
			{"ticket", idStr(f.ticketBody), 1},
		}, refs(res.Hits))
		assert.Equal(t, 3, res.TotalHits)

		titleHit := res.Hits[1]
		assert.Equal(t, "Printer "+f.marker, titleHit.Title)
		assert.Equal(t, f.ticketTitleTN, titleHit.Metadata["ticket_number"])
		assert.Equal(t, "Postmaster", titleHit.Metadata["queue"])
		assert.Equal(t, "new", titleHit.Metadata["state"])
		assert.Equal(t, "3 normal", titleHit.Metadata["priority"])
		assert.Equal(t, []string{"Printer <mark>" + f.marker + "</mark>"}, titleHit.Highlights["title"])
	})

	t.Run("exact ticket number ranks highest", func(t *testing.T) {
		res := search(t, SearchQuery{Query: f.ticketTitleTN, Types: []string{"ticket"}})
		assert.Equal(t, []hitRef{{"ticket", idStr(f.ticketTitle), 4}}, refs(res.Hits))
	})

	t.Run("every word must match, in any order and case", func(t *testing.T) {
		// Words out of order: no phrase match on the title, so lowest rank.
		res := search(t, SearchQuery{Query: f.marker + " printer", Types: []string{"ticket"}})
		assert.Equal(t, []hitRef{{"ticket", idStr(f.ticketTitle), 1}}, refs(res.Hits))

		// Case-insensitive exact title.
		res = search(t, SearchQuery{Query: "PRINTER " + f.marker, Types: []string{"ticket"}})
		assert.Equal(t, []hitRef{{"ticket", idStr(f.ticketTitle), 3}}, refs(res.Hits))

		// "printer" is on one ticket, "toner" on another: no ticket has every word.
		res = search(t, SearchQuery{Query: "printer toner " + f.marker, Types: []string{"ticket"}})
		assert.Empty(t, res.Hits)

		res = search(t, SearchQuery{Query: "toner " + f.marker, Types: []string{"ticket"}})
		assert.Equal(t, []hitRef{{"ticket", idStr(f.ticketBody), 1}}, refs(res.Hits))
	})

	t.Run("LIKE wildcards in the query match literally", func(t *testing.T) {
		res := search(t, SearchQuery{Query: f.marker + "%", Types: []string{"ticket", "article", "customer"}})
		assert.Empty(t, res.Hits)
		assert.Zero(t, res.TotalHits)
	})

	t.Run("ticket filters", func(t *testing.T) {
		res := search(t, SearchQuery{Query: f.marker, Types: []string{"ticket"}, Filters: map[string]string{"queue_id": "2"}})
		assert.Empty(t, res.Hits)

		res = search(t, SearchQuery{Query: f.marker, Types: []string{"ticket"}, Filters: map[string]string{"queue_id": "1", "state_id": "1"}})
		assert.Len(t, res.Hits, 3)

		_, err := backend.Search(ctx, SearchQuery{Query: f.marker, Types: []string{"ticket"}, Filters: map[string]string{"queue_id": "x"}})
		assert.ErrorIs(t, err, ErrInvalidQuery)
	})

	t.Run("articles rank exact subject over subject match over body match", func(t *testing.T) {
		res := search(t, SearchQuery{Query: f.marker, Types: []string{"article"}, Highlight: true})
		assert.Equal(t, []hitRef{
			{"article", idStr(f.articleExact), 3},
			{"article", idStr(f.articleSubj), 2},
			{"article", idStr(f.articleBody), 1},
		}, refs(res.Hits))

		bodyHit := res.Hits[2]
		assert.Equal(t, "Hello", bodyHit.Title)
		assert.Equal(t, "the toner "+f.marker+" is empty", bodyHit.Content)
		assert.Equal(t, f.ticketBodyTN, bodyHit.Metadata["ticket_number"])
		assert.Equal(t, "Unrelated request", bodyHit.Metadata["ticket_title"])
		assert.Equal(t, []string{"the toner <mark>" + f.marker + "</mark> is empty"}, bodyHit.Highlights["body"])
	})

	t.Run("customers rank exact login over name match", func(t *testing.T) {
		res := search(t, SearchQuery{Query: f.marker, Types: []string{"customer"}})
		assert.Equal(t, []hitRef{
			{"customer", idStr(f.customerLogin), 3},
			{"customer", idStr(f.customerName), 1},
		}, refs(res.Hits))

		loginHit := res.Hits[0]
		assert.Equal(t, "Bob Jones", loginHit.Title)
		assert.Equal(t, "Email: "+f.marker+"@example.com", loginHit.Content)
		assert.Equal(t, f.marker, loginHit.Metadata["login"])
		assert.Equal(t, f.companyName, loginHit.Metadata["company"])
		_, hasCompany := res.Hits[1].Metadata["company"]
		assert.False(t, hasCompany, "customer without a customer_company row has no company")

		res = search(t, SearchQuery{Query: "ann LEE" + f.marker, Types: []string{"customer"}})
		assert.Equal(t, []hitRef{{"customer", idStr(f.customerName), 2}}, refs(res.Hits))
	})

	t.Run("pagination spans the concatenated types", func(t *testing.T) {
		res := search(t, SearchQuery{Query: f.marker, Types: []string{"ticket", "article", "customer"}, Offset: 3, Limit: 3})
		assert.Equal(t, 8, res.TotalHits)
		assert.Equal(t, []hitRef{
			{"article", idStr(f.articleExact), 3},
			{"article", idStr(f.articleSubj), 2},
			{"article", idStr(f.articleBody), 1},
		}, refs(res.Hits))

		res = search(t, SearchQuery{Query: f.marker, Types: []string{"ticket", "article", "customer"}, Offset: 7, Limit: 5})
		assert.Equal(t, []hitRef{{"customer", idStr(f.customerName), 1}}, refs(res.Hits))

		res = search(t, SearchQuery{Query: f.marker, Types: []string{"ticket", "article", "customer"}, Offset: 8, Limit: 5})
		assert.Empty(t, res.Hits)
		assert.Equal(t, 8, res.TotalHits)
	})

	t.Run("health check", func(t *testing.T) {
		assert.NoError(t, backend.HealthCheck(ctx))
		assert.Equal(t, "database", backend.GetBackendName())
	})
}
