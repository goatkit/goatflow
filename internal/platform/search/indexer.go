package search

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

const (
	// syncWindow is how far back Sync looks for changed rows (by change_time;
	// ticket_history by id). It is longer than a day so rows stamped in
	// another time zone than the database clock are still seen.
	syncWindow = 26 * time.Hour
	// staleAfter: an index not synced for this long may have missed changes
	// that have left the window; Sync rebuilds it.
	staleAfter = syncWindow - 2*time.Hour
	// reconcileEvery bounds how often Sync compares document counts with the
	// database to find deleted rows.
	reconcileEvery = 10 * time.Minute
	// dbBatch is the number of ids per database query and per index scan page.
	dbBatch = 500
	// maxBulkBytes bounds the size of one _bulk request body.
	maxBulkBytes = 8 << 20
)

// bulkOp indexes doc under id, or deletes id when doc is nil.
type bulkOp struct {
	docType string
	id      string
	doc     interface{}
}

type bulkResponse struct {
	Errors bool                        `json:"errors"`
	Items  []map[string]bulkItemResult `json:"items"`
}

type bulkItemResult struct {
	ID     string          `json:"_id"`
	Status int             `json:"status"`
	Error  json.RawMessage `json:"error"`
}

// bulk sends ops in _bulk requests of at most maxBulkBytes.
func (b *ExternalBackend) bulk(ctx context.Context, ops []bulkOp) error {
	return b.bulkTo(ctx, "/_bulk", ops)
}

// bulkTo is bulk with an explicit API path (to pass query parameters).
func (b *ExternalBackend) bulkTo(ctx context.Context, path string, ops []bulkOp) error {
	var buf bytes.Buffer
	flush := func() error {
		if buf.Len() == 0 {
			return nil
		}
		defer buf.Reset()
		var resp bulkResponse
		status, body, err := b.do(ctx, http.MethodPost, path, "application/x-ndjson", buf.Bytes(), &resp)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return b.rejected(http.MethodPost, path, status, body)
		}
		if !resp.Errors {
			return nil
		}
		for _, item := range resp.Items {
			for action, r := range item {
				if r.Status < 300 || (action == "delete" && r.Status == http.StatusNotFound) {
					continue
				}
				return &ServiceError{Backend: b.cfg.Flavor, Reason: "indexing failed",
					Err: fmt.Errorf("%w: %s %s: HTTP %d: %s", errBulk, action, r.ID, r.Status, truncate(string(r.Error), 300))}
			}
		}
		return nil
	}
	for _, op := range ops {
		action := "index"
		if op.doc == nil {
			action = "delete"
		}
		line, err := json.Marshal(map[string]interface{}{action: map[string]string{"_index": b.index(op.docType), "_id": op.id}})
		if err != nil {
			return err
		}
		var doc []byte
		if op.doc != nil {
			if doc, err = json.Marshal(op.doc); err != nil {
				return err
			}
		}
		if buf.Len() > 0 && buf.Len()+len(line)+len(doc)+2 > maxBulkBytes {
			if err := flush(); err != nil {
				return err
			}
		}
		buf.Write(line)
		buf.WriteByte('\n')
		if doc != nil {
			buf.Write(doc)
			buf.WriteByte('\n')
		}
	}
	return flush()
}

// Documents stored in the index.

type ticketDoc struct {
	TicketID  int64  `json:"ticket_id"`
	QueueID   int64  `json:"queue_id"`
	StateID   int64  `json:"state_id"`
	TN        string `json:"tn"`
	Title     string `json:"title"`
	Articles  string `json:"articles"` // subjects and bodies of the ticket's articles
	Queue     string `json:"queue"`
	State     string `json:"state"`
	Priority  string `json:"priority"`
	CreatedAt string `json:"created_at"`
}

type articleDoc struct {
	ArticleID    int64  `json:"article_id"`
	TicketID     int64  `json:"ticket_id"`
	QueueID      int64  `json:"queue_id"`
	Subject      string `json:"subject"`
	Body         string `json:"body"`
	Sender       string `json:"sender"`
	TicketNumber string `json:"ticket_number"`
	TicketTitle  string `json:"ticket_title"`
	CreatedAt    string `json:"created_at"`
}

type customerDoc struct {
	CustomerUserID int64  `json:"customer_user_id"`
	Login          string `json:"login"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Company        string `json:"company"`
	CreatedAt      string `json:"created_at"`
}

func inList(n int) string { return "(?" + strings.Repeat(", ?", n-1) + ")" }

func idArgs(ids []int64) []interface{} {
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

func idString(id int64) string { return strconv.FormatInt(id, 10) }

// chunks splits ids into slices of at most n.
func chunks(ids []int64, n int) [][]int64 {
	var out [][]int64
	for len(ids) > n {
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

// loadTickets reads the tickets in ids (at most dbBatch) and their articles.
func loadTickets(ctx context.Context, db *sql.DB, ids []int64) (map[int64]*ticketDoc, []articleDoc, error) {
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT t.id, t.tn, COALESCE(t.title, ''), t.queue_id, t.ticket_state_id, t.create_time,
			COALESCE(q.name, ''), COALESCE(s.name, ''), COALESCE(p.name, '')
		FROM ticket t
		LEFT JOIN queue q ON q.id = t.queue_id
		LEFT JOIN ticket_state s ON s.id = t.ticket_state_id
		LEFT JOIN ticket_priority p ON p.id = t.ticket_priority_id
		WHERE t.id IN `+inList(len(ids))), idArgs(ids)...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	tickets := map[int64]*ticketDoc{}
	for rows.Next() {
		var (
			d       ticketDoc
			created time.Time
		)
		if err := rows.Scan(&d.TicketID, &d.TN, &d.Title, &d.QueueID, &d.StateID, &created,
			&d.Queue, &d.State, &d.Priority); err != nil {
			return nil, nil, err
		}
		d.CreatedAt = created.Format(time.RFC3339)
		tickets[d.TicketID] = &d
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(tickets) == 0 {
		return tickets, nil, nil
	}

	found := make([]int64, 0, len(tickets))
	for id := range tickets {
		found = append(found, id)
	}
	arows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT a.id, a.ticket_id, COALESCE(adm.a_subject, ''), COALESCE(adm.a_body, ''),
			COALESCE(adm.a_from, ''), a.create_time
		FROM article a
		JOIN article_data_mime adm ON adm.article_id = a.id
		WHERE a.ticket_id IN `+inList(len(found))+`
		ORDER BY a.id`), idArgs(found)...)
	if err != nil {
		return nil, nil, err
	}
	defer arows.Close()
	var articles []articleDoc
	text := map[int64]*strings.Builder{}
	for arows.Next() {
		var (
			d       articleDoc
			created time.Time
		)
		if err := arows.Scan(&d.ArticleID, &d.TicketID, &d.Subject, &d.Body, &d.Sender, &created); err != nil {
			return nil, nil, err
		}
		t := tickets[d.TicketID]
		d.QueueID, d.TicketNumber, d.TicketTitle = t.QueueID, t.TN, t.Title
		d.CreatedAt = created.Format(time.RFC3339)
		articles = append(articles, d)
		sb := text[d.TicketID]
		if sb == nil {
			sb = &strings.Builder{}
			text[d.TicketID] = sb
		}
		sb.WriteString(d.Subject)
		sb.WriteByte('\n')
		sb.WriteString(d.Body)
		sb.WriteByte('\n')
	}
	if err := arows.Err(); err != nil {
		return nil, nil, err
	}
	for id, sb := range text {
		tickets[id].Articles = sb.String()
	}
	return tickets, articles, nil
}

// indexTickets brings the documents of the given tickets and their articles
// in line with the database: present rows are (re)indexed, documents of
// deleted tickets and articles are removed.
func (b *ExternalBackend) indexTickets(ctx context.Context, db *sql.DB, ids []int64) error {
	for _, batch := range chunks(ids, dbBatch) {
		tickets, articles, err := loadTickets(ctx, db, batch)
		if err != nil {
			return fmt.Errorf("load tickets: %w", err)
		}
		ops := make([]bulkOp, 0, len(batch)+len(articles))
		for _, id := range batch {
			if t, ok := tickets[id]; ok {
				ops = append(ops, bulkOp{docType: docTicket, id: idString(id), doc: t})
			} else {
				ops = append(ops, bulkOp{docType: docTicket, id: idString(id)})
			}
		}
		current := make(map[int64]bool, len(articles))
		for i := range articles {
			current[articles[i].ArticleID] = true
			ops = append(ops, bulkOp{docType: docArticle, id: idString(articles[i].ArticleID), doc: &articles[i]})
		}
		// Article documents of these tickets whose article is gone.
		err = b.scanIDs(ctx, docArticle, []interface{}{
			map[string]interface{}{"terms": map[string]interface{}{"ticket_id": batch}},
		}, func(page []indexedDoc) error {
			for _, d := range page {
				if !current[d.id] {
					ops = append(ops, bulkOp{docType: docArticle, id: idString(d.id)})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if err := b.bulk(ctx, ops); err != nil {
			return err
		}
	}
	return nil
}

// indexCustomers (re)indexes the given customer users and removes the
// documents of deleted ones.
func (b *ExternalBackend) indexCustomers(ctx context.Context, db *sql.DB, ids []int64) error {
	for _, batch := range chunks(ids, dbBatch) {
		rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
			SELECT cu.id, cu.login, cu.email, cu.first_name, cu.last_name, cu.create_time,
				COALESCE(cc.name, '')
			FROM customer_user cu
			LEFT JOIN customer_company cc ON cc.customer_id = cu.customer_id
			WHERE cu.id IN `+inList(len(batch))), idArgs(batch)...)
		if err != nil {
			return fmt.Errorf("load customers: %w", err)
		}
		found := map[int64]*customerDoc{}
		for rows.Next() {
			var (
				d       customerDoc
				created time.Time
			)
			if err := rows.Scan(&d.CustomerUserID, &d.Login, &d.Email, &d.FirstName, &d.LastName,
				&created, &d.Company); err != nil {
				_ = rows.Close()
				return err
			}
			d.CreatedAt = created.Format(time.RFC3339)
			found[d.CustomerUserID] = &d
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		ops := make([]bulkOp, 0, len(batch))
		for _, id := range batch {
			if d, ok := found[id]; ok {
				ops = append(ops, bulkOp{docType: docCustomer, id: idString(id), doc: d})
			} else {
				ops = append(ops, bulkOp{docType: docCustomer, id: idString(id)})
			}
		}
		if err := b.bulk(ctx, ops); err != nil {
			return err
		}
	}
	return nil
}

// indexedDoc is one document returned by scanIDs.
type indexedDoc struct {
	id       int64
	ticketID int64
}

// scanIDs pages through the documents of one index matching filter in
// ascending id order.
func (b *ExternalBackend) scanIDs(ctx context.Context, docType string, filter []interface{}, fn func([]indexedDoc) error) error {
	field := idField[docType]
	var last int64 = -1
	for {
		f := append([]interface{}{
			map[string]interface{}{"range": map[string]interface{}{field: map[string]interface{}{"gt": last}}},
		}, filter...)
		resp, err := b.searchIndex(ctx, docType, map[string]interface{}{
			"query":   map[string]interface{}{"bool": map[string]interface{}{"filter": f}},
			"sort":    []interface{}{map[string]interface{}{field: map[string]interface{}{"order": "asc"}}},
			"size":    dbBatch,
			"_source": []string{field, "ticket_id"},
		})
		if err != nil {
			return err
		}
		if len(resp.Hits.Hits) == 0 {
			return nil
		}
		page := make([]indexedDoc, 0, len(resp.Hits.Hits))
		for _, h := range resp.Hits.Hits {
			id, err := strconv.ParseInt(h.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("%s index holds a document with non-numeric id %q", docType, h.ID)
			}
			d := indexedDoc{id: id}
			if v, ok := h.Source["ticket_id"].(float64); ok {
				d.ticketID = int64(v)
			}
			page = append(page, d)
			last = max(last, id)
		}
		if err := fn(page); err != nil {
			return err
		}
		if len(resp.Hits.Hits) < dbBatch {
			return nil
		}
	}
}

// existsQuery returns the ids of rows that back documents of docType.
var existsQuery = map[string]string{
	docTicket:   `SELECT id FROM ticket WHERE id IN `,
	docArticle:  `SELECT a.id FROM article a JOIN article_data_mime adm ON adm.article_id = a.id WHERE a.id IN `,
	docCustomer: `SELECT id FROM customer_user WHERE id IN `,
}

func existingIDs(ctx context.Context, db *sql.DB, docType string, ids []int64) (map[int64]bool, error) {
	out := make(map[int64]bool, len(ids))
	for _, batch := range chunks(ids, dbBatch) {
		rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(existsQuery[docType]+inList(len(batch))), idArgs(batch)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[id] = true
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// reconcile removes documents of docType whose row no longer exists. Tickets
// that lost articles are reindexed so their article text is current.
func (b *ExternalBackend) reconcile(ctx context.Context, db *sql.DB, docType string) error {
	var stale []bulkOp
	touched := map[int64]bool{}
	err := b.scanIDs(ctx, docType, nil, func(page []indexedDoc) error {
		ids := make([]int64, len(page))
		for i, d := range page {
			ids[i] = d.id
		}
		exists, err := existingIDs(ctx, db, docType, ids)
		if err != nil {
			return err
		}
		for _, d := range page {
			if !exists[d.id] {
				stale = append(stale, bulkOp{docType: docType, id: idString(d.id)})
				if docType == docArticle {
					touched[d.ticketID] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := b.bulk(ctx, stale); err != nil {
		return err
	}
	if len(touched) > 0 {
		ids := make([]int64, 0, len(touched))
		for id := range touched {
			ids = append(ids, id)
		}
		return b.indexTickets(ctx, db, ids)
	}
	return nil
}

// countQuery counts the rows that back documents of docType.
var countQuery = map[string]string{
	docTicket:   `SELECT COUNT(*) FROM ticket`,
	docArticle:  `SELECT COUNT(*) FROM article_data_mime`,
	docCustomer: `SELECT COUNT(*) FROM customer_user`,
}

// reconcileIfNeeded reconciles every type whose index holds more documents
// than the database has rows: something was deleted.
func (b *ExternalBackend) reconcileIfNeeded(ctx context.Context, db *sql.DB) error {
	for _, docType := range []string{docTicket, docArticle, docCustomer} {
		var rowsN int
		if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(countQuery[docType])).Scan(&rowsN); err != nil {
			return err
		}
		docsN, err := b.countDocs(ctx, docType)
		if err != nil {
			return err
		}
		if docsN > rowsN {
			if err := b.reconcile(ctx, db, docType); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifyHits drops hits whose row no longer exists (and removes their
// documents) and, for queue-restricted queries, ticket and article hits whose
// ticket is no longer in a permitted queue.
func (b *ExternalBackend) verifyHits(ctx context.Context, docType string, hits []esHit, query SearchQuery) ([]esHit, int, error) {
	if len(hits) == 0 {
		return hits, 0, nil
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(hits))
	for _, h := range hits {
		id, err := strconv.ParseInt(h.ID, 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("%s index holds a document with non-numeric id %q", docType, h.ID)
		}
		ids = append(ids, id)
	}
	var q string
	switch docType {
	case docTicket:
		q = `SELECT t.id, t.queue_id FROM ticket t WHERE t.id IN `
	case docArticle:
		q = `SELECT a.id, t.queue_id FROM article a
			JOIN article_data_mime adm ON adm.article_id = a.id
			JOIN ticket t ON t.id = a.ticket_id WHERE a.id IN `
	default:
		q = `SELECT id, 0 FROM customer_user WHERE id IN `
	}
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(q+inList(len(ids))), idArgs(ids)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	queueOf := make(map[int64]int, len(ids))
	for rows.Next() {
		var id int64
		var queueID int
		if err := rows.Scan(&id, &queueID); err != nil {
			return nil, 0, err
		}
		queueOf[id] = queueID
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	permitted := map[int]bool{}
	restricted := query.RestrictQueues && docType != docCustomer
	for _, q := range query.QueueIDs {
		permitted[q] = true
	}
	kept := hits[:0]
	var gone []bulkOp
	for i, h := range hits {
		queueID, ok := queueOf[ids[i]]
		switch {
		case !ok:
			gone = append(gone, bulkOp{docType: docType, id: h.ID})
		case restricted && !permitted[queueID]:
		default:
			kept = append(kept, h)
		}
	}
	if len(gone) > 0 {
		if err := b.bulk(ctx, gone); err != nil {
			log.Printf("search: removing %d stale %s documents: %v", len(gone), docType, err)
		}
	}
	return kept, len(hits) - len(kept), nil
}

// syncState is what Sync remembers between passes (in memory: a restarted
// process re-reads the whole window once).
type syncState struct {
	started bool
	// seen holds, per table, the change_time each row in the window was
	// indexed at.
	seen map[string]map[int64]time.Time
	// historySeen holds ticket_history ids above historyCursor already handled.
	historySeen   map[int64]bool
	historyCursor int64
	historyPrev   int64 // highest history id at the previous pass
	lastReconcile time.Time
	// metaWritten is when this process last stored a usable state document.
	metaWritten time.Time
}

func newSyncState() syncState {
	return syncState{
		seen:        map[string]map[int64]time.Time{"ticket": {}, "article": {}, "customer_user": {}},
		historySeen: map[int64]bool{},
	}
}

// windowRow is a row changed inside the sync window.
type windowRow struct {
	id, ticketID int64
	changed      time.Time
}

var windowQuery = map[string]string{
	"ticket":        `SELECT id, id, change_time FROM ticket WHERE change_time >= ?`,
	"article":       `SELECT id, ticket_id, change_time FROM article WHERE change_time >= ?`,
	"customer_user": `SELECT id, 0, change_time FROM customer_user WHERE change_time >= ?`,
}

// changedRows returns the rows of table changed at or after threshold.
func changedRows(ctx context.Context, db *sql.DB, table string, threshold time.Time) ([]windowRow, error) {
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(windowQuery[table]), threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []windowRow
	for rows.Next() {
		var r windowRow
		if err := rows.Scan(&r.id, &r.ticketID, &r.changed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// dbNow returns the database clock.
func dbNow(ctx context.Context, db *sql.DB) (time.Time, error) {
	var now time.Time
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT CURRENT_TIMESTAMP")).Scan(&now)
	return now, err
}

// maxHistoryID returns the highest ticket_history id (0 when empty).
func maxHistoryID(ctx context.Context, db *sql.DB) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT COALESCE(MAX(id), 0) FROM ticket_history")).Scan(&id)
	return id, err
}

// SyncStats reports what one Sync pass did.
type SyncStats struct {
	Rebuilt   bool
	Tickets   int
	Customers int
}

// Sync brings the index up to date with the database. It builds the index
// when it has never been built, was built for another schema version, or has
// not been synced for longer than the window covers. Otherwise it reindexes
// tickets whose ticket or article rows changed or that gained history entries
// and customer users that changed, and periodically removes documents of
// deleted rows.
func (b *ExternalBackend) Sync(ctx context.Context, db *sql.DB) (SyncStats, error) {
	b.syncMu.Lock()
	defer b.syncMu.Unlock()

	meta, err := b.readMeta(ctx)
	if err != nil {
		return SyncStats{}, err
	}
	// A state document this process wrote moments ago may not be searchable
	// yet (Zinc shows writes about a second later): trust our own write.
	ownWrite := b.sync.started && time.Since(b.sync.metaWritten) < time.Minute
	if !ownWrite && (!meta.usable() || syncedLongAgo(meta.LastSync)) {
		err := b.rebuildLocked(ctx, db)
		return SyncStats{Rebuilt: true}, err
	}

	passStart := time.Now()
	now, err := dbNow(ctx, db)
	if err != nil {
		return SyncStats{}, err
	}
	threshold := now.Add(-syncWindow)
	st := &b.sync
	if !st.started {
		if err := b.initHistoryCursor(ctx, db, threshold); err != nil {
			return SyncStats{}, err
		}
		st.started = true
	}

	tickets := map[int64]bool{}
	pending := map[string][]windowRow{}
	for _, table := range []string{"ticket", "article", "customer_user"} {
		rows, err := changedRows(ctx, db, table, threshold)
		if err != nil {
			return SyncStats{}, fmt.Errorf("changed %s rows: %w", table, err)
		}
		for _, r := range rows {
			if prev, ok := st.seen[table][r.id]; ok && prev.Equal(r.changed) {
				continue
			}
			pending[table] = append(pending[table], r)
			if table != "customer_user" {
				tickets[r.ticketID] = true
			}
		}
	}

	historyMax, err := maxHistoryID(ctx, db)
	if err != nil {
		return SyncStats{}, err
	}
	var newHistory []int64
	for cursor := st.historyCursor; ; {
		rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(
			`SELECT id, ticket_id FROM ticket_history WHERE id > ? AND id <= ? ORDER BY id LIMIT ?`),
			cursor, historyMax, 5000)
		if err != nil {
			return SyncStats{}, err
		}
		n := 0
		for rows.Next() {
			var id, ticketID int64
			if err := rows.Scan(&id, &ticketID); err != nil {
				_ = rows.Close()
				return SyncStats{}, err
			}
			n++
			cursor = id
			if !st.historySeen[id] {
				newHistory = append(newHistory, id)
				tickets[ticketID] = true
			}
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return SyncStats{}, err
		}
		if n < 5000 {
			break
		}
	}

	ticketIDs := make([]int64, 0, len(tickets))
	for id := range tickets {
		ticketIDs = append(ticketIDs, id)
	}
	customerIDs := make([]int64, 0, len(pending["customer_user"]))
	for _, r := range pending["customer_user"] {
		customerIDs = append(customerIDs, r.id)
	}
	if err := b.indexTickets(ctx, db, ticketIDs); err != nil {
		return SyncStats{}, err
	}
	if err := b.indexCustomers(ctx, db, customerIDs); err != nil {
		return SyncStats{}, err
	}

	// Everything read above is indexed: remember it.
	for table, rows := range pending {
		for _, r := range rows {
			st.seen[table][r.id] = r.changed
		}
	}
	for table, seen := range st.seen {
		for id, changed := range seen {
			if changed.Before(threshold) {
				delete(st.seen[table], id)
			}
		}
	}
	for _, id := range newHistory {
		st.historySeen[id] = true
	}
	// Rows with ids up to the previous pass's maximum have had a full pass
	// interval to commit: move the cursor there and forget their ids.
	if st.historyPrev > st.historyCursor {
		st.historyCursor = st.historyPrev
		for id := range st.historySeen {
			if id <= st.historyCursor {
				delete(st.historySeen, id)
			}
		}
	}
	st.historyPrev = historyMax

	if time.Since(st.lastReconcile) >= reconcileEvery {
		if err := b.reconcileIfNeeded(ctx, db); err != nil {
			return SyncStats{}, err
		}
		st.lastReconcile = time.Now()
	}

	if err := b.writeMeta(ctx, metaState{SchemaVersion: schemaVersion, Ready: "true",
		LastSync: passStart.UTC().Format(time.RFC3339)}); err != nil {
		return SyncStats{}, err
	}
	st.metaWritten = time.Now()
	return SyncStats{Tickets: len(ticketIDs), Customers: len(customerIDs)}, nil
}

func syncedLongAgo(lastSync string) bool {
	t, err := time.Parse(time.RFC3339, lastSync)
	return err != nil || time.Since(t) > staleAfter
}

// initHistoryCursor starts history processing at the first entry inside the
// window (or after the last entry when the window has none).
func (b *ExternalBackend) initHistoryCursor(ctx context.Context, db *sql.DB, threshold time.Time) error {
	var first sql.NullInt64
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT MIN(id) FROM ticket_history WHERE create_time >= ?`), threshold).Scan(&first); err != nil {
		return err
	}
	if first.Valid {
		b.sync.historyCursor = first.Int64 - 1
		return nil
	}
	maxID, err := maxHistoryID(ctx, db)
	b.sync.historyCursor = maxID
	return err
}

// RebuildStatus describes the most recent full rebuild started by this process.
type RebuildStatus struct {
	Running    bool       `json:"running"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Tickets    int        `json:"tickets"`
	Customers  int        `json:"customers"`
	Error      string     `json:"error,omitempty"`
}

// RebuildStatus returns the state of the latest rebuild.
func (b *ExternalBackend) RebuildStatus() RebuildStatus {
	b.rebuildMu.Lock()
	defer b.rebuildMu.Unlock()
	return b.rebuild
}

// StartRebuild starts a full rebuild in the background. It returns false when
// one is already running in this process.
func (b *ExternalBackend) StartRebuild(db *sql.DB) bool {
	b.rebuildMu.Lock()
	if b.rebuild.Running {
		b.rebuildMu.Unlock()
		return false
	}
	now := time.Now()
	b.rebuild = RebuildStatus{Running: true, StartedAt: &now}
	b.rebuildMu.Unlock()

	go func() {
		err := b.Rebuild(context.Background(), db)
		if err != nil {
			log.Printf("search: %s rebuild failed: %v", b.cfg.Flavor, err)
		}
		b.rebuildMu.Lock()
		defer b.rebuildMu.Unlock()
		done := time.Now()
		b.rebuild.Running = false
		b.rebuild.FinishedAt = &done
		if err != nil {
			b.rebuild.Error = err.Error()
		}
	}()
	return true
}

// Rebuild indexes every ticket, article and customer user and removes
// documents whose row no longer exists. The index stays searchable while it
// runs once it has been built before.
func (b *ExternalBackend) Rebuild(ctx context.Context, db *sql.DB) error {
	b.syncMu.Lock()
	defer b.syncMu.Unlock()
	return b.rebuildLocked(ctx, db)
}

func (b *ExternalBackend) rebuildLocked(ctx context.Context, db *sql.DB) error {
	if err := b.ensureIndices(ctx); err != nil {
		return err
	}
	// Snapshot the change window first: rows changing while the rebuild runs
	// then still differ from the snapshot and are picked up by the next Sync.
	now, err := dbNow(ctx, db)
	if err != nil {
		return err
	}
	threshold := now.Add(-syncWindow)
	fresh := newSyncState()
	for _, table := range []string{"ticket", "article", "customer_user"} {
		rows, err := changedRows(ctx, db, table, threshold)
		if err != nil {
			return fmt.Errorf("changed %s rows: %w", table, err)
		}
		for _, r := range rows {
			fresh.seen[table][r.id] = r.changed
		}
	}
	if fresh.historyCursor, err = maxHistoryID(ctx, db); err != nil {
		return err
	}
	fresh.historyPrev = fresh.historyCursor
	passStart := time.Now()

	for _, table := range []struct {
		name    string
		index   func(context.Context, *sql.DB, []int64) error
		counter *int
	}{
		{"ticket", b.indexTickets, &b.rebuild.Tickets},
		{"customer_user", b.indexCustomers, &b.rebuild.Customers},
	} {
		var last int64
		for {
			ids, err := nextIDs(ctx, db, table.name, last)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				break
			}
			if err := table.index(ctx, db, ids); err != nil {
				return err
			}
			last = ids[len(ids)-1]
			b.rebuildMu.Lock()
			*table.counter += len(ids)
			b.rebuildMu.Unlock()
		}
	}
	for _, docType := range []string{docTicket, docArticle, docCustomer} {
		if err := b.reconcile(ctx, db, docType); err != nil {
			return err
		}
	}
	if err := b.writeMeta(ctx, metaState{SchemaVersion: schemaVersion, Ready: "true",
		LastSync: passStart.UTC().Format(time.RFC3339)}); err != nil {
		return err
	}
	fresh.started = true
	fresh.lastReconcile = time.Now()
	fresh.metaWritten = time.Now()
	b.sync = fresh
	b.markReady()
	return nil
}

// nextIDQuery pages through the ids of the tables Rebuild walks.
var nextIDQuery = map[string]string{
	"ticket":        `SELECT id FROM ticket WHERE id > ? ORDER BY id LIMIT ?`,
	"customer_user": `SELECT id FROM customer_user WHERE id > ? ORDER BY id LIMIT ?`,
}

// nextIDs returns up to dbBatch ids of table greater than after.
func nextIDs(ctx context.Context, db *sql.DB, table string, after int64) ([]int64, error) {
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(nextIDQuery[table]), after, dbBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
