//go:build integration

package search

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/goatkit/goatflow/internal/platform/database"
)

const searchRunHint = "run it with `make test-search-integration` (needs Docker and the test databases)"

// startSearchService starts a Zinc or Elasticsearch container and returns the
// backend configuration for it.
func startSearchService(t *testing.T, flavor string) (ExternalConfig, testcontainers.Container) {
	t.Helper()
	ctx := context.Background()
	var req testcontainers.ContainerRequest
	cfg := ExternalConfig{Flavor: flavor, IndexPrefix: "gftest" + strconv.FormatInt(time.Now().UnixNano(), 36) + "_"}
	switch flavor {
	case FlavorZinc:
		cfg.Username, cfg.Password = "admin", "Zinc-test-pw#1"
		req = testcontainers.ContainerRequest{
			Image:        "public.ecr.aws/zinclabs/zincsearch:0.4.10",
			ExposedPorts: []string{"4080/tcp"},
			Env: map[string]string{
				"ZINC_FIRST_ADMIN_USER":     cfg.Username,
				"ZINC_FIRST_ADMIN_PASSWORD": cfg.Password,
				"ZINC_DATA_PATH":            "/data",
			},
			WaitingFor: wait.ForHTTP("/healthz").WithPort("4080/tcp").WithStartupTimeout(60 * time.Second),
		}
	default:
		req = testcontainers.ContainerRequest{
			Image:        "docker.elastic.co/elasticsearch/elasticsearch:8.15.3",
			ExposedPorts: []string{"9200/tcp"},
			Env: map[string]string{
				"discovery.type":         "single-node",
				"xpack.security.enabled": "false",
				// CI disks are often fuller than Elasticsearch's watermarks allow.
				"cluster.routing.allocation.disk.threshold_enabled": "false",
				"ES_JAVA_OPTS": "-Xms512m -Xmx512m",
			},
			Tmpfs: map[string]string{"/usr/share/elasticsearch/data": "rw,size=512m,uid=1000"},
			WaitingFor: wait.ForHTTP("/_cluster/health").WithPort("9200/tcp").
				WithStatusCodeMatcher(func(s int) bool { return s == http.StatusOK }).
				WithStartupTimeout(180 * time.Second),
		}
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("FATAL: could not start %s: %v\n%s", flavor, err, searchRunHint)
	}
	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, req.ExposedPorts[0])
	require.NoError(t, err)
	cfg.Endpoint = "http://" + host + ":" + port.Port()
	return cfg, ctr
}

// syncFixture is a set of rows whose searchable words all carry marker.
type syncFixture struct {
	db                       *sql.DB
	marker                   string
	otherQueue               int
	printer, scanner         int64 // tickets in queue 1 and otherQueue
	printerNote, scannerNote int64 // one article each
	customer                 int64
	tnPrefix                 string
}

func seedSyncFixture(t *testing.T, db *sql.DB) *syncFixture {
	t.Helper()
	run := strconv.FormatInt(time.Now().UnixNano(), 36)
	f := &syncFixture{db: db, marker: "sync" + run, tnPrefix: "S" + run}
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT MIN(id) FROM queue WHERE id <> 1`)).Scan(&f.otherQueue))

	t.Cleanup(func() {
		for _, c := range []struct{ q, pattern string }{
			{"DELETE FROM ticket_history WHERE ticket_id IN (SELECT id FROM ticket WHERE tn LIKE ?)", f.tnPrefix + "%"},
			{"DELETE FROM article_data_mime WHERE article_id IN (SELECT a.id FROM article a JOIN ticket t ON t.id = a.ticket_id WHERE t.tn LIKE ?)", f.tnPrefix + "%"},
			{"DELETE FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE tn LIKE ?)", f.tnPrefix + "%"},
			{"DELETE FROM ticket WHERE tn LIKE ?", f.tnPrefix + "%"},
			{"DELETE FROM customer_user WHERE login LIKE ?", f.marker + "%"},
		} {
			if _, err := db.Exec(database.ConvertPlaceholders(c.q), c.pattern); err != nil {
				t.Errorf("cleanup %q: %v", c.q, err)
			}
		}
	})

	f.printer = f.insertTicket(t, "1", 1, "Printer "+f.marker)
	f.scanner = f.insertTicket(t, "2", f.otherQueue, "Scanner "+f.marker)
	f.printerNote = f.insertArticle(t, f.printer, "Toner", "the toner "+f.marker+" is empty")
	f.scannerNote = f.insertArticle(t, f.scanner, "Scanner jam "+f.marker, "paper stuck")
	f.customer = f.insertCustomer(t, "c1", "Ann", "Lee")
	return f
}

func (f *syncFixture) insertTicket(t *testing.T, suffix string, queueID int, title string) int64 {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	id, err := database.GetAdapter().InsertWithReturning(f.db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id,
			ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
			user_id, responsible_user_id, timeout, until_time, escalation_time,
			escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 1, 1, 1, 3, '', '', 1, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)
		RETURNING id`), f.tnPrefix+suffix, title, queueID, now, now)
	require.NoError(t, err)
	return id
}

func (f *syncFixture) insertArticle(t *testing.T, ticketID int64, subject, body string) int64 {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	id, err := database.GetAdapter().InsertWithReturning(f.db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, 3, 1, 1, ?, 1, ?, 1)
		RETURNING id`), ticketID, now, now)
	require.NoError(t, err)
	_, err = f.db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, incoming_time,
			create_time, create_by, change_time, change_by)
		VALUES (?, 'customer@example.com', ?, ?, 0, ?, 1, ?, 1)`), id, subject, body, now, now)
	require.NoError(t, err)
	return id
}

func (f *syncFixture) insertCustomer(t *testing.T, suffix, first, last string) int64 {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	login := f.marker + "-" + suffix
	id, err := database.GetAdapter().InsertWithReturning(f.db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name,
			valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'acme', ?, ?, 1, ?, 1, ?, 1)
		RETURNING id`), login, login+"@example.com", first, last, now, now)
	require.NoError(t, err)
	return id
}

// later is a change_time after every value the fixture wrote before.
func later(n int) time.Time {
	return time.Now().UTC().Truncate(time.Second).Add(time.Duration(n) * time.Minute)
}

func (f *syncFixture) exec(t *testing.T, q string, args ...interface{}) {
	t.Helper()
	_, err := f.db.Exec(database.ConvertPlaceholders(q), args...)
	require.NoError(t, err, q)
}

func hitSet(res *SearchResults) []string {
	out := make([]string, 0, len(res.Hits))
	for _, h := range res.Hits {
		out = append(out, h.Type+":"+h.ID)
	}
	sort.Strings(out)
	return out
}

func ref(docType string, id int64) string { return docType + ":" + strconv.FormatInt(id, 10) }

func sorted(s ...string) []string { sort.Strings(s); return s }

// eventually retries fn until it passes: Zinc makes writes searchable after
// about a second.
func eventually(t *testing.T, fn func(c *assert.CollectT)) {
	t.Helper()
	assert.EventuallyWithT(t, fn, 15*time.Second, 200*time.Millisecond)
}

func TestExternalBackendIntegration(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Fatalf("test database not available: %v\n%s", err, searchRunHint)
	}
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db, searchRunHint)

	for _, flavor := range []string{FlavorZinc, FlavorElasticsearch} {
		t.Run(flavor, func(t *testing.T) {
			cfg, ctr := startSearchService(t, flavor)
			b := NewExternalBackend(cfg)
			ctx := context.Background()
			f := seedSyncFixture(t, db)
			dbBackend, err := NewDatabaseBackend()
			require.NoError(t, err)
			ticketsAndArticles := []string{"ticket", "article"}

			search := func(c *assert.CollectT, q SearchQuery) *SearchResults {
				res, err := b.Search(ctx, q)
				if !assert.NoError(c, err) {
					return &SearchResults{}
				}
				return res
			}

			t.Run("an unbuilt index refuses searches instead of returning nothing", func(t *testing.T) {
				_, err := b.Search(ctx, SearchQuery{Query: f.marker, Types: ticketsAndArticles})
				assert.ErrorIs(t, err, ErrIndexNotReady)
				assert.ErrorIs(t, b.HealthCheck(ctx), ErrIndexNotReady)
			})

			t.Run("the first sync builds the index", func(t *testing.T) {
				stats, err := b.Sync(ctx, db)
				require.NoError(t, err)
				assert.True(t, stats.Rebuilt)
				eventually(t, func(c *assert.CollectT) { assert.NoError(c, b.HealthCheck(ctx)) })
				// A second backend (another process) sees the built index too.
				eventually(t, func(c *assert.CollectT) { assert.NoError(c, NewExternalBackend(cfg).HealthCheck(ctx)) })
			})

			t.Run("queue permissions match the database backend", func(t *testing.T) {
				for _, scope := range []struct {
					name       string
					restricted bool
					queues     []int
				}{
					{"all queues", false, nil},
					{"queue 1", true, []int{1}},
					{"other queue", true, []int{f.otherQueue}},
					{"no queues", true, []int{}},
				} {
					q := SearchQuery{Query: f.marker, Types: ticketsAndArticles, Limit: 50,
						RestrictQueues: scope.restricted, QueueIDs: scope.queues}
					want, err := dbBackend.Search(ctx, q)
					require.NoError(t, err)
					eventually(t, func(c *assert.CollectT) {
						got := search(c, q)
						assert.Equal(c, hitSet(want), hitSet(got), scope.name)
						assert.Equal(c, want.TotalHits, got.TotalHits, scope.name)
					})
				}
				res, err := dbBackend.Search(ctx, SearchQuery{Query: f.marker, Types: ticketsAndArticles,
					RestrictQueues: true, QueueIDs: []int{1}})
				require.NoError(t, err)
				assert.Equal(t, sorted(ref("ticket", f.printer), ref("article", f.printerNote)), hitSet(res),
					"fixture sanity: queue 1 holds the printer ticket and its article")
			})

			t.Run("tickets match article text; customers are found", func(t *testing.T) {
				eventually(t, func(c *assert.CollectT) {
					res := search(c, SearchQuery{Query: "toner " + f.marker, Types: []string{"ticket", "article", "customer"}})
					assert.Equal(c, sorted(ref("ticket", f.printer), ref("article", f.printerNote)), hitSet(res))
					res = search(c, SearchQuery{Query: "ann " + f.marker, Types: []string{"customer"}})
					assert.Equal(c, []string{ref("customer", f.customer)}, hitSet(res))
					if assert.Len(c, res.Hits, 1) {
						assert.Equal(c, "Ann Lee", res.Hits[0].Title)
						assert.Equal(c, f.marker+"-c1", res.Hits[0].Metadata["login"])
					}
				})
			})

			t.Run("a queue move hides the ticket before the next sync", func(t *testing.T) {
				f.exec(t, `UPDATE ticket SET queue_id = ? WHERE id = ?`, f.otherQueue, f.printer)
				q := SearchQuery{Query: f.marker, Types: ticketsAndArticles, RestrictQueues: true, QueueIDs: []int{1}}
				res, err := b.Search(ctx, q)
				require.NoError(t, err)
				assert.Empty(t, res.Hits, "index still says queue 1; the database check drops the hits")
				assert.Zero(t, res.TotalHits)

				// The move is recorded in ticket_history (ticket.change_time untouched).
				f.exec(t, `INSERT INTO ticket_history (name, history_type_id, ticket_id, type_id, queue_id,
					owner_id, priority_id, state_id, create_time, create_by, change_time, change_by)
					SELECT 'moved', id, ?, 1, ?, 1, 3, 1, ?, 1, ?, 1 FROM ticket_history_type WHERE name = 'Move'`,
					f.printer, f.otherQueue, later(0), later(0))
				stats, err := b.Sync(ctx, db)
				require.NoError(t, err)
				assert.False(t, stats.Rebuilt)
				assert.GreaterOrEqual(t, stats.Tickets, 1)
				eventually(t, func(c *assert.CollectT) {
					res := search(c, SearchQuery{Query: "printer " + f.marker, Types: ticketsAndArticles,
						RestrictQueues: true, QueueIDs: []int{f.otherQueue}})
					assert.Equal(c, []string{ref("ticket", f.printer)}, hitSet(res))
					res = search(c, SearchQuery{Query: "toner " + f.marker, Types: []string{"article"},
						RestrictQueues: true, QueueIDs: []int{f.otherQueue}})
					assert.Equal(c, []string{ref("article", f.printerNote)}, hitSet(res), "article follows its ticket's queue")
				})
			})

			t.Run("edits, new rows and new customers are indexed by sync", func(t *testing.T) {
				f.exec(t, `UPDATE ticket SET title = ?, change_time = ? WHERE id = ?`, "Plotter "+f.marker, later(1), f.scanner)
				f.exec(t, `UPDATE article_data_mime SET a_body = ? WHERE article_id = ?`, "cartridge "+f.marker+" leaks", f.printerNote)
				f.exec(t, `UPDATE article SET change_time = ? WHERE id = ?`, later(1), f.printerNote)
				fresh := f.insertTicket(t, "3", 1, "Monitor "+f.marker)
				freshNote := f.insertArticle(t, fresh, "Flicker", "screen "+f.marker+" flickers")
				newCustomer := f.insertCustomer(t, "c2", "Bob", "Stone")

				_, err := b.Sync(ctx, db)
				require.NoError(t, err)
				eventually(t, func(c *assert.CollectT) {
					all := []string{"ticket", "article", "customer"}
					plotter := search(c, SearchQuery{Query: "plotter " + f.marker, Types: all})
					assert.Equal(c, []string{ref("ticket", f.scanner)}, hitSet(plotter))
					if assert.Len(c, plotter.Hits, 1) {
						assert.Equal(c, "Plotter "+f.marker, plotter.Hits[0].Title, "the title was reindexed")
					}
					assert.Equal(c, sorted(ref("ticket", f.printer), ref("article", f.printerNote)),
						hitSet(search(c, SearchQuery{Query: "cartridge " + f.marker, Types: all})))
					assert.Empty(c, hitSet(search(c, SearchQuery{Query: "empty " + f.marker, Types: all})),
						"the old article body is gone from the article and its ticket")
					assert.Equal(c, sorted(ref("ticket", fresh), ref("article", freshNote)),
						hitSet(search(c, SearchQuery{Query: "flickers " + f.marker, Types: all})))
					assert.Equal(c, []string{ref("customer", newCustomer)},
						hitSet(search(c, SearchQuery{Query: "stone " + f.marker, Types: all})))
				})
			})

			t.Run("deleted rows never show up and their documents are removed", func(t *testing.T) {
				before, err := b.countDocs(ctx, docTicket)
				require.NoError(t, err)
				f.exec(t, `DELETE FROM article_data_mime WHERE article_id = ?`, f.scannerNote)
				f.exec(t, `DELETE FROM article WHERE id = ?`, f.scannerNote)
				f.exec(t, `DELETE FROM ticket WHERE id = ?`, f.scanner)
				res, err := b.Search(ctx, SearchQuery{Query: "plotter " + f.marker, Types: ticketsAndArticles})
				require.NoError(t, err)
				assert.Empty(t, res.Hits)
				assert.Zero(t, res.TotalHits)
				eventually(t, func(c *assert.CollectT) {
					n, err := b.countDocs(ctx, docTicket)
					assert.NoError(c, err)
					assert.Equal(c, before-1, n, "the stale ticket document was removed when it was hit")
				})

				// Deletions nobody searched for are found by the reconcile pass.
				custBefore, err := b.countDocs(ctx, docCustomer)
				require.NoError(t, err)
				artBefore, err := b.countDocs(ctx, docArticle)
				require.NoError(t, err)
				f.exec(t, `DELETE FROM customer_user WHERE id = ?`, f.customer)
				require.NoError(t, b.Rebuild(ctx, db))
				eventually(t, func(c *assert.CollectT) {
					n, err := b.countDocs(ctx, docCustomer)
					assert.NoError(c, err)
					assert.Equal(c, custBefore-1, n)
					n, err = b.countDocs(ctx, docArticle)
					assert.NoError(c, err)
					assert.Equal(c, artBefore-1, n, "the deleted ticket's article document is gone")
				})
				st, err := b.Status(ctx)
				require.NoError(t, err)
				assert.True(t, st.Ready)
				assert.NotEmpty(t, st.LastSync)
			})

			t.Run("a service outage is an error, not an empty result", func(t *testing.T) {
				require.NoError(t, ctr.Stop(ctx, nil))
				_, err := b.Search(ctx, SearchQuery{Query: f.marker, Types: ticketsAndArticles})
				var svcErr *ServiceError
				if !assert.ErrorAs(t, err, &svcErr) {
					return
				}
				assert.Equal(t, flavor, svcErr.Backend)
				assert.ErrorAs(t, b.HealthCheck(ctx), &svcErr)
				_, err = b.Sync(ctx, db)
				assert.ErrorAs(t, err, &svcErr)
			})
		})
	}
}
