package search

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// DatabaseBackend implements SearchBackend by querying the GoatFlow tables
// directly with portable SQL (MySQL/MariaDB and PostgreSQL).
//
// Matching: the query is split into words and every word must occur
// (case-insensitive substring) in at least one searched column. Relevance is
// a CASE expression: exact ticket number / title / subject / login matches
// rank above partial title matches, which rank above body-only matches.
type DatabaseBackend struct {
	db *sql.DB
}

// NewDatabaseBackend creates a search backend over the application database.
func NewDatabaseBackend() (*DatabaseBackend, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return &DatabaseBackend{db: db}, nil
}

// GetBackendName returns the backend name.
func (db *DatabaseBackend) GetBackendName() string {
	return "database"
}

// likeMatch is a case-insensitive substring test against a %term% pattern
// built by containsPattern. '!' is the LIKE escape character on both drivers.
const likeMatch = " LIKE LOWER(?) ESCAPE '!'"

var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

// containsPattern turns a search word into a LIKE pattern matching it
// anywhere, with LIKE wildcards in the word matched literally.
func containsPattern(word string) string {
	return "%" + likeEscaper.Replace(word) + "%"
}

// wordFilter returns a WHERE clause requiring every word to satisfy clause,
// whose placeholders are all bound to that word's contains-pattern.
func wordFilter(words []string, clause string) (string, []interface{}) {
	perWord := strings.Count(clause, "?")
	parts := make([]string, 0, len(words))
	args := make([]interface{}, 0, len(words)*perWord)
	for _, w := range words {
		parts = append(parts, clause)
		p := containsPattern(w)
		for range perWord {
			args = append(args, p)
		}
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

// Search searches the requested entity types. Hits are concatenated in the
// order of query.Types, each type ordered by relevance, then paginated.
func (db *DatabaseBackend) Search(ctx context.Context, query SearchQuery) (*SearchResults, error) {
	startTime := time.Now()
	results := &SearchResults{
		Query: query.Query,
		Hits:  []SearchHit{},
	}

	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	words := strings.Fields(strings.TrimSpace(query.Query))
	if len(words) == 0 {
		results.Took = time.Since(startTime).Milliseconds()
		return results, nil
	}

	// The first Offset+Limit hits of the concatenation need at most that many
	// rows from each type.
	fetch := query.Offset + query.Limit

	for _, entityType := range query.Types {
		var (
			hits  []SearchHit
			total int
			err   error
		)
		switch entityType {
		case "ticket":
			hits, total, err = db.searchTickets(ctx, query, words, fetch)
		case "article":
			hits, total, err = db.searchArticles(ctx, query, words, fetch)
		case "customer":
			hits, total, err = db.searchCustomers(ctx, query, words, fetch)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		results.Hits = append(results.Hits, hits...)
		results.TotalHits += total
	}

	results.Took = time.Since(startTime).Milliseconds()

	start := query.Offset
	end := query.Offset + query.Limit
	if end > len(results.Hits) {
		end = len(results.Hits)
	}
	if start < len(results.Hits) {
		results.Hits = results.Hits[start:end]
	} else {
		results.Hits = []SearchHit{}
	}

	return results, nil
}

// count runs SELECT COUNT(*) over the given FROM and WHERE clauses.
func (db *DatabaseBackend) count(ctx context.Context, from, where string, args []interface{}) (int, error) {
	var n int
	err := db.db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT COUNT(*)"+from+where), args...).Scan(&n)
	return n, err
}

const ticketFrom = " FROM ticket t"

const ticketWordClause = "(LOWER(t.tn)" + likeMatch +
	" OR LOWER(t.title)" + likeMatch +
	" OR EXISTS (SELECT 1 FROM article a JOIN article_data_mime adm ON adm.article_id = a.id" +
	" WHERE a.ticket_id = t.id AND (LOWER(adm.a_subject)" + likeMatch + " OR LOWER(adm.a_body)" + likeMatch + ")))"

// searchTickets matches ticket number, title and the ticket's article
// subjects/bodies.
func (db *DatabaseBackend) searchTickets(ctx context.Context, query SearchQuery, words []string, fetch int) ([]SearchHit, int, error) {
	where, whereArgs := wordFilter(words, ticketWordClause)

	for _, f := range []struct{ key, column string }{
		{"queue_id", "t.queue_id"},
		{"state_id", "t.ticket_state_id"},
	} {
		raw, ok := query.Filters[f.key]
		if !ok {
			continue
		}
		id, err := strconv.Atoi(raw)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: filter %s must be an integer", ErrInvalidQuery, f.key)
		}
		where += " AND " + f.column + " = ?"
		whereArgs = append(whereArgs, id)
	}

	total, err := db.count(ctx, ticketFrom, where, whereArgs)
	if err != nil {
		return nil, 0, err
	}

	phrase := containsPattern(query.Query)
	args := append([]interface{}{query.Query, query.Query, phrase, phrase}, whereArgs...)
	args = append(args, fetch)

	sqlQuery := `SELECT t.id, t.tn, COALESCE(t.title, ''), t.create_time,
			q.name, s.name, p.name,
			CASE
				WHEN LOWER(t.tn) = LOWER(?) THEN 4
				WHEN LOWER(t.title) = LOWER(?) THEN 3
				WHEN LOWER(t.tn)` + likeMatch + ` OR LOWER(t.title)` + likeMatch + ` THEN 2
				ELSE 1
			END AS score` +
		ticketFrom + `
		LEFT JOIN queue q ON q.id = t.queue_id
		LEFT JOIN ticket_state s ON s.id = t.ticket_state_id
		LEFT JOIN ticket_priority p ON p.id = t.ticket_priority_id` +
		where + `
		ORDER BY score DESC, t.id DESC
		LIMIT ?`

	rows, err := db.db.QueryContext(ctx, database.ConvertPlaceholders(sqlQuery), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var (
			hit                                SearchHit
			tn                                 string
			createTime                         time.Time
			queueName, stateName, priorityName sql.NullString
		)
		if err := rows.Scan(&hit.ID, &tn, &hit.Title, &createTime,
			&queueName, &stateName, &priorityName, &hit.Score); err != nil {
			return nil, 0, err
		}

		hit.Type = "ticket"
		hit.Content = hit.Title
		metadata := map[string]interface{}{
			"ticket_number": tn,
			"created_at":    createTime.Format(time.RFC3339),
		}
		if queueName.Valid {
			metadata["queue"] = queueName.String
		}
		if stateName.Valid {
			metadata["state"] = stateName.String
		}
		if priorityName.Valid {
			metadata["priority"] = priorityName.String
		}
		hit.Metadata = metadata

		if query.Highlight {
			hit.Highlights = map[string][]string{
				"title": {highlightText(hit.Title, query.Query)},
			}
		}

		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return hits, total, nil
}

const articleFrom = ` FROM article_data_mime adm
		JOIN article a ON a.id = adm.article_id
		JOIN ticket t ON t.id = a.ticket_id`

const articleWordClause = "(LOWER(adm.a_subject)" + likeMatch +
	" OR LOWER(adm.a_body)" + likeMatch +
	" OR LOWER(adm.a_from)" + likeMatch + ")"

// searchArticles matches article subject, body and sender.
func (db *DatabaseBackend) searchArticles(ctx context.Context, query SearchQuery, words []string, fetch int) ([]SearchHit, int, error) {
	where, whereArgs := wordFilter(words, articleWordClause)

	total, err := db.count(ctx, articleFrom, where, whereArgs)
	if err != nil {
		return nil, 0, err
	}

	args := append([]interface{}{query.Query, containsPattern(query.Query)}, whereArgs...)
	args = append(args, fetch)

	sqlQuery := `SELECT a.id, COALESCE(adm.a_subject, ''), COALESCE(adm.a_body, ''), a.create_time,
			t.tn, COALESCE(t.title, ''),
			CASE
				WHEN LOWER(adm.a_subject) = LOWER(?) THEN 3
				WHEN LOWER(adm.a_subject)` + likeMatch + ` THEN 2
				ELSE 1
			END AS score` +
		articleFrom +
		where + `
		ORDER BY score DESC, a.id DESC
		LIMIT ?`

	rows, err := db.db.QueryContext(ctx, database.ConvertPlaceholders(sqlQuery), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var (
			hit                       SearchHit
			createTime                time.Time
			ticketNumber, ticketTitle string
		)
		if err := rows.Scan(&hit.ID, &hit.Title, &hit.Content, &createTime,
			&ticketNumber, &ticketTitle, &hit.Score); err != nil {
			return nil, 0, err
		}

		hit.Type = "article"
		hit.Metadata = map[string]interface{}{
			"created_at":    createTime.Format(time.RFC3339),
			"ticket_number": ticketNumber,
			"ticket_title":  ticketTitle,
		}

		if query.Highlight {
			hit.Highlights = map[string][]string{
				"subject": {highlightText(hit.Title, query.Query)},
				"body":    {highlightText(hit.Content, query.Query)},
			}
		}

		if len(hit.Content) > 200 {
			hit.Content = hit.Content[:200] + "..."
		}

		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return hits, total, nil
}

const customerFrom = ` FROM customer_user cu
		LEFT JOIN customer_company cc ON cc.customer_id = cu.customer_id`

const customerWordClause = "(LOWER(cu.login)" + likeMatch +
	" OR LOWER(cu.email)" + likeMatch +
	" OR LOWER(cu.first_name)" + likeMatch +
	" OR LOWER(cu.last_name)" + likeMatch + ")"

// searchCustomers matches customer login, email and name.
func (db *DatabaseBackend) searchCustomers(ctx context.Context, query SearchQuery, words []string, fetch int) ([]SearchHit, int, error) {
	where, whereArgs := wordFilter(words, customerWordClause)

	total, err := db.count(ctx, customerFrom, where, whereArgs)
	if err != nil {
		return nil, 0, err
	}

	args := append([]interface{}{query.Query, query.Query, query.Query}, whereArgs...)
	args = append(args, fetch)

	sqlQuery := `SELECT cu.id, cu.login, cu.first_name, cu.last_name, cu.email, cu.create_time,
			cc.name,
			CASE
				WHEN LOWER(cu.login) = LOWER(?) OR LOWER(cu.email) = LOWER(?) THEN 3
				WHEN LOWER(CONCAT(cu.first_name, ' ', cu.last_name)) = LOWER(?) THEN 2
				ELSE 1
			END AS score` +
		customerFrom +
		where + `
		ORDER BY score DESC, cu.login
		LIMIT ?`

	rows, err := db.db.QueryContext(ctx, database.ConvertPlaceholders(sqlQuery), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var (
			hit                               SearchHit
			login, firstName, lastName, email string
			createTime                        time.Time
			companyName                       sql.NullString
		)
		if err := rows.Scan(&hit.ID, &login, &firstName, &lastName, &email, &createTime,
			&companyName, &hit.Score); err != nil {
			return nil, 0, err
		}

		hit.Type = "customer"
		hit.Title = strings.TrimSpace(firstName + " " + lastName)
		if hit.Title == "" {
			hit.Title = login
		}
		hit.Content = "Email: " + email

		metadata := map[string]interface{}{
			"login":      login,
			"email":      email,
			"created_at": createTime.Format(time.RFC3339),
		}
		if companyName.Valid {
			metadata["company"] = companyName.String
		}
		hit.Metadata = metadata

		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return hits, total, nil
}

// Index is a no-op: the database backend searches the live tables.
func (db *DatabaseBackend) Index(ctx context.Context, doc Document) error {
	return nil
}

// Delete is a no-op: the database backend searches the live tables.
func (db *DatabaseBackend) Delete(ctx context.Context, docType string, id string) error {
	return nil
}

// BulkIndex is a no-op: the database backend searches the live tables.
func (db *DatabaseBackend) BulkIndex(ctx context.Context, docs []Document) error {
	return nil
}

// HealthCheck verifies the database connection.
func (db *DatabaseBackend) HealthCheck(ctx context.Context) error {
	return db.db.PingContext(ctx)
}

// highlightText adds simple highlighting to matched text.
func highlightText(text, query string) string {
	words := strings.Fields(strings.ToLower(query))
	result := text
	for _, word := range words {
		// Simple case-insensitive replace with <mark> tags
		result = strings.ReplaceAll(
			result,
			word,
			fmt.Sprintf("<mark>%s</mark>", word),
		)
	}
	return result
}
