package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// PostgresBackend implements SearchBackend using PostgreSQL full-text search.
type PostgresBackend struct {
	db *sql.DB
}

// NewPostgresBackend creates a new PostgreSQL search backend.
func NewPostgresBackend() (*PostgresBackend, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return &PostgresBackend{db: db}, nil
}

// GetBackendName returns the backend name.
func (pb *PostgresBackend) GetBackendName() string {
	return "postgresql"
}

// Search performs a full-text search using PostgreSQL.
func (pb *PostgresBackend) Search(ctx context.Context, query SearchQuery) (*SearchResults, error) {
	startTime := time.Now()
	results := &SearchResults{
		Query: query.Query,
		Hits:  []SearchHit{},
	}

	// Default pagination
	if query.Limit == 0 {
		query.Limit = 20
	}

	// Search different entity types
	for _, entityType := range query.Types {
		switch entityType {
		case "ticket":
			hits, err := pb.searchTickets(ctx, query)
			if err != nil {
				return nil, err
			}
			results.Hits = append(results.Hits, hits...)
		case "article":
			hits, err := pb.searchArticles(ctx, query)
			if err != nil {
				return nil, err
			}
			results.Hits = append(results.Hits, hits...)
		case "customer":
			hits, err := pb.searchCustomers(ctx, query)
			if err != nil {
				return nil, err
			}
			results.Hits = append(results.Hits, hits...)
		}
	}

	results.TotalHits = len(results.Hits)
	results.Took = time.Since(startTime).Milliseconds()

	// Apply pagination to combined results
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

// searchTickets searches for tickets.
func (pb *PostgresBackend) searchTickets(ctx context.Context, query SearchQuery) ([]SearchHit, error) {
	// Build the SQL query with full-text search
	sqlQuery := database.ConvertPlaceholders(`
		SELECT 
			t.id, t.tn, t.title, 
			COALESCE(t.title || ' ' || STRING_AGG(a.body, ' '), t.title) as content,
			ts_rank(to_tsvector('english', t.title), plainto_tsquery('english', ?)) as score,
			t.create_time,
			q.name as queue_name,
			s.name as state_name,
			p.name as priority_name
		FROM tickets t
		LEFT JOIN article a ON a.ticket_id = t.id
		LEFT JOIN queues q ON t.queue_id = q.id
		LEFT JOIN ticket_state s ON t.ticket_state_id = s.id
		LEFT JOIN ticket_priority p ON t.ticket_priority_id = p.id
		WHERE to_tsvector('english', t.title) @@ plainto_tsquery('english', ?)
	`)

	args := []interface{}{query.Query}

	// Add filters
	if queueFilter, ok := query.Filters["queue_id"]; ok {
		sqlQuery += " AND t.queue_id = ?"
		args = append(args, queueFilter)
	}

	if stateFilter, ok := query.Filters["state_id"]; ok {
		sqlQuery += " AND t.ticket_state_id = ?"
		args = append(args, stateFilter)
	}

	sqlQuery += `
		GROUP BY t.id, t.tn, t.title, t.create_time, q.name, s.name, p.name
		ORDER BY score DESC
	`

	rows, err := pb.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var hit SearchHit
		var createTime time.Time
		var queueName, stateName, priorityName sql.NullString
		var score float64

		err := rows.Scan(
			&hit.ID, &hit.Metadata, &hit.Title, &hit.Content,
			&score, &createTime, &queueName, &stateName, &priorityName,
		)
		if err != nil {
			continue
		}

		hit.Type = "ticket"
		hit.Score = score

		// Build metadata
		metadata := map[string]interface{}{
			"ticket_number": hit.Metadata,
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

		// Add highlighting if requested
		if query.Highlight {
			hit.Highlights = map[string][]string{
				"title": {highlightText(hit.Title, query.Query)},
			}
		}

		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return hits, nil
}

// searchArticles searches for articles.
func (pb *PostgresBackend) searchArticles(ctx context.Context, query SearchQuery) ([]SearchHit, error) {
	sqlQuery := database.ConvertPlaceholders(`
		SELECT 
			a.id, a.subject, a.body,
			ts_rank(to_tsvector('english', a.subject || ' ' || a.body), plainto_tsquery('english', ?)) as score,
			a.create_time,
			t.tn as ticket_number,
			t.title as ticket_title
		FROM article a
		JOIN tickets t ON a.ticket_id = t.id
		WHERE to_tsvector('english', a.subject || ' ' || a.body) @@ plainto_tsquery('english', ?)
		ORDER BY score DESC
	`)

	rows, err := pb.db.QueryContext(ctx, sqlQuery, query.Query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var hit SearchHit
		var createTime time.Time
		var score float64
		var ticketNumber, ticketTitle string

		err := rows.Scan(
			&hit.ID, &hit.Title, &hit.Content,
			&score, &createTime, &ticketNumber, &ticketTitle,
		)
		if err != nil {
			continue
		}

		hit.Type = "article"
		hit.Score = score
		hit.Metadata = map[string]interface{}{
			"created_at":    createTime.Format(time.RFC3339),
			"ticket_number": ticketNumber,
			"ticket_title":  ticketTitle,
		}

		// Add highlighting
		if query.Highlight {
			hit.Highlights = map[string][]string{
				"subject": {highlightText(hit.Title, query.Query)},
				"body":    {highlightText(hit.Content, query.Query)},
			}
		}

		// Truncate content for display
		if len(hit.Content) > 200 {
			hit.Content = hit.Content[:200] + "..."
		}

		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return hits, nil
}

// searchCustomers searches for customers.
func (pb *PostgresBackend) searchCustomers(ctx context.Context, query SearchQuery) ([]SearchHit, error) {
	sqlQuery := database.ConvertPlaceholders(`
		SELECT 
			cu.id, cu.login, 
			COALESCE(cu.first_name || ' ' || cu.last_name, cu.login) as name,
			cu.email,
			cu.create_time,
			cc.name as company_name
		FROM customer_user cu
		LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
		WHERE 
			to_tsvector('english', cu.login || ' ' || COALESCE(cu.first_name, '') || ' ' || COALESCE(cu.last_name, '') || ' ' || cu.email) 
			@@ plainto_tsquery('english', ?)
		ORDER BY cu.create_time DESC
	`)

	rows, err := pb.db.QueryContext(ctx, sqlQuery, query.Query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var hit SearchHit
		var createTime time.Time
		var email string
		var companyName sql.NullString

		err := rows.Scan(
			&hit.ID, &hit.Metadata, &hit.Title,
			&email, &createTime, &companyName,
		)
		if err != nil {
			continue
		}

		hit.Type = "customer"
		hit.Score = 1.0 // PostgreSQL doesn't provide relevance for this query
		hit.Content = fmt.Sprintf("Email: %s", email)

		metadata := map[string]interface{}{
			"login":      hit.Metadata,
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
		return nil, err
	}

	return hits, nil
}

// Index adds or updates a document (no-op for PostgreSQL as it searches directly).
func (pb *PostgresBackend) Index(ctx context.Context, doc Document) error {
	// PostgreSQL searches directly on the database tables
	// No separate indexing needed
	return nil
}

// Delete removes a document from the index (no-op for PostgreSQL).
func (pb *PostgresBackend) Delete(ctx context.Context, docType string, id string) error {
	// PostgreSQL searches directly on the database tables
	// No separate deletion needed
	return nil
}

// BulkIndex indexes multiple documents (no-op for PostgreSQL).
func (pb *PostgresBackend) BulkIndex(ctx context.Context, docs []Document) error {
	// PostgreSQL searches directly on the database tables
	// No separate indexing needed
	return nil
}

// HealthCheck verifies the PostgreSQL connection.
func (pb *PostgresBackend) HealthCheck(ctx context.Context) error {
	return pb.db.PingContext(ctx)
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
