// Package api provides HTTP API handlers for GOTRS.
package api

import (
	"database/sql"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// ArticleInsertParams holds parameters for creating an article.
type ArticleInsertParams struct {
	TicketID             int64
	CommunicationChannel int // 1=email, 2=phone, 3=internal, 4=chat
	IsVisibleForCustomer int
	CreateBy             int64
}

// insertArticle creates an article record and returns the article ID.
// This handles both MySQL and PostgreSQL with appropriate ID retrieval.
func insertArticle(tx *sql.Tx, params ArticleInsertParams) (int64, error) {
	query := database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES ($1, 1, $2, $3, CURRENT_TIMESTAMP, $4, CURRENT_TIMESTAMP, $4)
		RETURNING id
	`)
	args := []interface{}{params.TicketID, params.CommunicationChannel, params.IsVisibleForCustomer, params.CreateBy}
	return database.GetAdapter().InsertWithReturningTx(tx, query, args...)
}

// ArticleMimeParams holds parameters for article MIME data.
type ArticleMimeParams struct {
	ArticleID    int64
	From         string
	To           string // optional, empty for notes
	Subject      string
	Body         string
	ContentType  string
	IncomingTime int64
	CreateBy     int64
}

// insertArticleMimeData inserts the article MIME data (subject, body, etc).
func insertArticleMimeData(tx *sql.Tx, params ArticleMimeParams) error {
	var insertQuery string
	var args []interface{}

	if params.To != "" {
		insertQuery = `
			INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body,
				a_content_type, incoming_time, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, $8, CURRENT_TIMESTAMP, $8)
		`
		args = []interface{}{
			params.ArticleID, params.From, params.To, params.Subject,
			params.Body, params.ContentType, params.IncomingTime, params.CreateBy,
		}
	} else {
		insertQuery = `
			INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body,
				a_content_type, incoming_time, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, $7, CURRENT_TIMESTAMP, $7)
		`
		args = []interface{}{
			params.ArticleID, params.From, params.Subject, params.Body,
			params.ContentType, params.IncomingTime, params.CreateBy,
		}
	}

	// Adapter handles placeholder conversion and arg remapping for repeated $N
	_, err := database.GetAdapter().ExecTx(tx, insertQuery, args...)
	return err
}

// defaultNoteSubject returns a default subject based on communication channel.
func defaultNoteSubject(channelID int) string {
	switch channelID {
	case 1:
		return "Email Note"
	case 2:
		return "Phone Note"
	case 3:
		return "Internal Note"
	case 4:
		return "Chat Note"
	default:
		return "Note"
	}
}
