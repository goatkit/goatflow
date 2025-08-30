package api

import (
	"database/sql"
	"fmt"
	"log"
	
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// GetArticleAttachments retrieves attachments for a specific article from the database
func GetArticleAttachments(articleID int) ([]map[string]interface{}, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}
	
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, filename, content_type, content_size, content, disposition
		FROM article_data_mime_attachment
		WHERE article_id = $1
		ORDER BY id
	`), articleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query attachments: %w", err)
	}
	defer rows.Close()
	
	attachments := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var filename, contentType, contentSize, disposition string
		var content []byte
		var filenameNull, contentTypeNull, contentSizeNull, dispositionNull sql.NullString
		
		err := rows.Scan(&id, &filenameNull, &contentTypeNull, &contentSizeNull, &content, &dispositionNull)
		if err != nil {
			log.Printf("ERROR: Failed to scan attachment row: %v", err)
			continue
		}
		
		// Handle nullable fields
		if filenameNull.Valid {
			filename = filenameNull.String
		}
		if contentTypeNull.Valid {
			contentType = contentTypeNull.String
		}
		if contentSizeNull.Valid {
			contentSize = contentSizeNull.String
		}
		if dispositionNull.Valid {
			disposition = dispositionNull.String
		}
		
		// The content field contains the file path (for now)
		filePath := string(content)
		
		attachment := map[string]interface{}{
			"ID":          id,
			"Filename":    filename,
			"ContentType": contentType,
			"Size":        contentSize,
			"Disposition": disposition,
			"Path":        filePath,
			"URL":         fmt.Sprintf("/api/tickets/attachments/%d/download", id),
		}
		
		attachments = append(attachments, attachment)
	}
	
	return attachments, nil
}

// GetTicketAttachments retrieves all attachments for all articles in a ticket
func GetTicketAttachments(ticketID int) (map[int][]map[string]interface{}, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}
	
	// Get all articles for this ticket
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT a.id, att.id, att.filename, att.content_type, att.content_size, att.content, att.disposition
		FROM article a
		LEFT JOIN article_data_mime_attachment att ON a.id = att.article_id
		WHERE a.ticket_id = $1
		ORDER BY a.id, att.id
	`), ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket attachments: %w", err)
	}
	defer rows.Close()
	
	attachmentsByArticle := make(map[int][]map[string]interface{})
	
	for rows.Next() {
		var articleID int
		var attID sql.NullInt64
		var filename, contentType, contentSize, disposition sql.NullString
		var content []byte
		
		err := rows.Scan(&articleID, &attID, &filename, &contentType, &contentSize, &content, &disposition)
		if err != nil {
			log.Printf("ERROR: Failed to scan attachment row: %v", err)
			continue
		}
		
		// If there's no attachment for this article, skip
		if !attID.Valid {
			continue
		}
		
		attachment := map[string]interface{}{
			"ID":          attID.Int64,
			"Filename":    filename.String,
			"ContentType": contentType.String,
			"Size":        contentSize.String,
			"Path":        string(content), // File path stored in content field
			"URL":         fmt.Sprintf("/api/attachments/%d/download", attID.Int64),
		}
		
		if attachmentsByArticle[articleID] == nil {
			attachmentsByArticle[articleID] = []map[string]interface{}{}
		}
		attachmentsByArticle[articleID] = append(attachmentsByArticle[articleID], attachment)
	}
	
	return attachmentsByArticle, nil
}