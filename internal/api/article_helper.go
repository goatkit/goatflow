// Package api provides HTTP API handlers for GoatFlow.
package api

import (
	"database/sql"

	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// ArticleInsertParams holds parameters for creating an article.
type ArticleInsertParams struct {
	TicketID             int64
	CommunicationChannel int // constants.CommunicationChannel*
	IsVisibleForCustomer int
	CreateBy             int64
}

// insertArticle creates an article record and returns the article ID.
// This handles both MySQL and PostgreSQL with appropriate ID retrieval.
func insertArticle(tx *sql.Tx, params ArticleInsertParams) (int64, error) {
	query := database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING id
	`)
	// Args: ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, create_by, change_by
	args := []interface{}{params.TicketID, constants.ArticleSenderAgent, params.CommunicationChannel, params.IsVisibleForCustomer, params.CreateBy, params.CreateBy}
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
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		`
		// Args: article_id, a_from, a_to, a_subject, a_body, a_content_type, incoming_time, create_by, change_by
		args = []interface{}{
			params.ArticleID, params.From, params.To, params.Subject,
			params.Body, params.ContentType, params.IncomingTime, params.CreateBy, params.CreateBy,
		}
	} else {
		insertQuery = `
			INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body,
				a_content_type, incoming_time, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		`
		// Args: article_id, a_from, a_subject, a_body, a_content_type, incoming_time, create_by, change_by
		args = []interface{}{
			params.ArticleID, params.From, params.Subject, params.Body,
			params.ContentType, params.IncomingTime, params.CreateBy, params.CreateBy,
		}
	}

	_, err := tx.Exec(database.ConvertPlaceholders(insertQuery), args...)
	return err
}

// defaultNoteSubject returns a default subject based on communication channel.
func defaultNoteSubject(channelID int) string {
	switch channelID {
	case constants.CommunicationChannelEmail:
		return "Email Note"
	case constants.CommunicationChannelPhone:
		return "Phone Note"
	case constants.CommunicationChannelInternal:
		return "Internal Note"
	case constants.CommunicationChannelChat:
		return "Chat Note"
	default:
		return "Note"
	}
}
