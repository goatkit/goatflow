// Package api provides HTTP handlers for the GOTRS application.
package api

import (
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// jsonError sends a JSON error response.
func jsonError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"success": false, "error": message})
}

// readFormFile reads a file from the multipart form.
func readFormFile(c *gin.Context, fieldName string) ([]byte, *multipart.FileHeader, error) {
	file, header, err := c.Request.FormFile(fieldName)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	return content, header, nil
}

// StandardAttachment represents a standard attachment in OTRS.
type StandardAttachment struct {
	ID                   int       `json:"id"`
	Name                 string    `json:"name"`
	Filename             string    `json:"filename"`
	ContentType          string    `json:"content_type"`
	Content              []byte    `json:"-"` // Don't send raw content in JSON
	ContentSize          int64     `json:"content_size"`
	ContentSizeFormatted string    `json:"content_size_formatted"`
	Comments             *string   `json:"comments"`
	ValidID              int       `json:"valid_id"`
	CreateTime           time.Time `json:"create_time"`
	CreateBy             int       `json:"create_by"`
	ChangeTime           time.Time `json:"change_time"`
	ChangeBy             int       `json:"change_by"`
}

// handleAdminAttachment displays the admin attachment management page.
func handleAdminAttachment(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get search and filter parameters
	searchQuery := c.Query("search")
	validFilter := c.DefaultQuery("valid", "all")

	// Build query
	query := `
		SELECT 
			id, name, filename, content_type, 
			LENGTH(content) as content_size, comments, valid_id,
			create_time, create_by, change_time, change_by
		FROM standard_attachment
		WHERE 1=1
	`

	var args []interface{}
	argCount := 1

	if searchQuery != "" {
		query += fmt.Sprintf(
			" AND (LOWER(name) LIKE $%d OR LOWER(filename) LIKE $%d OR LOWER(comments) LIKE $%d)",
			argCount, argCount+1, argCount+2)
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
		argCount += 3
	}

	if validFilter != "all" {
		if validFilter == "valid" {
			query += fmt.Sprintf(" AND valid_id = $%d", argCount)
			args = append(args, 1)
		} else if validFilter == "invalid" {
			query += fmt.Sprintf(" AND valid_id = $%d", argCount)
			args = append(args, 2)
		}
	}

	query += " ORDER BY name ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch attachments")
		return
	}
	defer rows.Close()

	var attachments []StandardAttachment
	for rows.Next() {
		var a StandardAttachment
		var comments sql.NullString

		err := rows.Scan(
			&a.ID, &a.Name, &a.Filename, &a.ContentType,
			&a.ContentSize, &comments, &a.ValidID,
			&a.CreateTime, &a.CreateBy, &a.ChangeTime, &a.ChangeBy,
		)
		if err != nil {
			continue
		}

		if comments.Valid {
			a.Comments = &comments.String
		}

		a.ContentSizeFormatted = formatFileSize(a.ContentSize)
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		c.String(http.StatusInternalServerError, "Error iterating attachments")
		return
	}

	// Valid options for the form dropdown
	validOptions := []map[string]interface{}{
		{"ID": 1, "Name": "valid"},
		{"ID": 2, "Name": "invalid"},
		{"ID": 3, "Name": "invalid-temporarily"},
	}

	// Render the template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/attachment.pongo2", pongo2.Context{
		"Title":        "Standard Attachment Management",
		"Attachments":  attachments,
		"SearchQuery":  searchQuery,
		"ValidFilter":  validFilter,
		"ValidOptions": validOptions,
		"User":         getUserMapForTemplate(c),
		"ActivePage":   "admin",
	})
}

// handleAdminAttachmentCreate creates a new standard attachment.
func handleAdminAttachmentCreate(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		jsonError(c, http.StatusBadRequest, "Failed to parse form")
		return
	}

	name := c.PostForm("name")
	comments := c.PostForm("comments")
	validIDStr := c.PostForm("valid_id")

	validID := 1
	if validIDStr != "" {
		if parsed, err := strconv.Atoi(validIDStr); err == nil {
			validID = parsed
		}
	}

	if strings.TrimSpace(name) == "" {
		jsonError(c, http.StatusBadRequest, "Name is required")
		return
	}

	content, header, err := readFormFile(c, "file")
	if err != nil {
		jsonError(c, http.StatusBadRequest, "File is required")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	var exists bool
	dupeQuery := database.ConvertPlaceholders(
		"SELECT EXISTS(SELECT 1 FROM standard_attachment WHERE name = $1)")
	if err = db.QueryRow(dupeQuery, name).Scan(&exists); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to check for duplicate")
		return
	}
	if exists {
		jsonError(c, http.StatusBadRequest, "Attachment with this name already exists")
		return
	}

	var commentsPtr *string
	if comments != "" {
		commentsPtr = &comments
	}

	result, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO standard_attachment 
			(name, filename, content_type, content, comments, valid_id, 
			 create_time, create_by, change_time, change_by)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), 1, NOW(), 1)
	`), name, header.Filename, header.Header.Get("Content-Type"), content, commentsPtr, validID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create attachment: "+err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to get attachment ID")
		return
	}

	c.Header("HX-Trigger", "attachmentCreated")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Attachment created successfully",
		"data":    gin.H{"id": id, "name": name, "filename": header.Filename},
	})
}

// handleAdminAttachmentUpdate updates an existing standard attachment.
func handleAdminAttachmentUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	_ = c.Request.ParseMultipartForm(10 << 20) //nolint:errcheck // file is optional

	db, err := database.GetDB()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	updates := []string{"change_time = CURRENT_TIMESTAMP", "change_by = 1"}
	args := []interface{}{}
	argCount := 1

	if name := c.PostForm("name"); name != "" {
		updates = append(updates, fmt.Sprintf("name = $%d", argCount))
		args = append(args, name)
		argCount++
	}

	if comments := c.PostForm("comments"); comments != "" {
		updates = append(updates, fmt.Sprintf("comments = $%d", argCount))
		args = append(args, comments)
		argCount++
	} else if c.PostForm("clear_comments") == "1" {
		updates = append(updates, fmt.Sprintf("comments = $%d", argCount))
		args = append(args, nil)
		argCount++
	}

	if validIDStr := c.PostForm("valid_id"); validIDStr != "" {
		if parsedValidID, err := strconv.Atoi(validIDStr); err == nil {
			updates = append(updates, fmt.Sprintf("valid_id = $%d", argCount))
			args = append(args, parsedValidID)
			argCount++
		}
	}

	if content, header, err := readFormFile(c, "file"); err == nil {
		updates = append(updates, fmt.Sprintf("filename = $%d", argCount))
		args = append(args, header.Filename)
		argCount++
		updates = append(updates, fmt.Sprintf("content_type = $%d", argCount))
		args = append(args, header.Header.Get("Content-Type"))
		argCount++
		updates = append(updates, fmt.Sprintf("content = $%d", argCount))
		args = append(args, content)
		argCount++
	}

	args = append(args, id)
	query := fmt.Sprintf(
		"UPDATE standard_attachment SET %s WHERE id = $%d",
		strings.Join(updates, ", "), argCount)

	result, err := db.Exec(database.ConvertPlaceholders(query), args...)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update attachment: "+err.Error())
		return
	}

	if rowsAffected, err := result.RowsAffected(); err != nil || rowsAffected == 0 {
		jsonError(c, http.StatusNotFound, "Attachment not found")
		return
	}

	c.Header("HX-Trigger", "attachmentUpdated")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Attachment updated successfully"})
}

// handleAdminAttachmentDelete soft deletes an attachment (sets valid_id = 2).
func handleAdminAttachmentDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid attachment ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Soft delete by setting valid_id = 2
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE standard_attachment 
		SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = 1 
		WHERE id = $1
	`), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete attachment",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Attachment not found",
		})
		return
	}

	c.Header("HX-Trigger", "attachmentDeleted")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Attachment deleted successfully",
	})
}

// handleAdminAttachmentDownload downloads an attachment.
func handleAdminAttachmentDownload(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	var filename, contentType string
	var content []byte

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT filename, content_type, content 
		FROM standard_attachment 
		WHERE id = $1
	`), id).Scan(&filename, &contentType, &content)

	if err == sql.ErrNoRows {
		c.String(http.StatusNotFound, "Attachment not found")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch attachment")
		return
	}

	// Set headers for file download
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Length", strconv.Itoa(len(content)))

	// Send the file content
	c.Data(http.StatusOK, contentType, content)
}

// handleAdminAttachmentPreview serves an attachment inline for preview.
func handleAdminAttachmentPreview(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	var filename, contentType string
	var content []byte

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT filename, content_type, content 
		FROM standard_attachment 
		WHERE id = $1
	`), id).Scan(&filename, &contentType, &content)

	if err == sql.ErrNoRows {
		c.String(http.StatusNotFound, "Attachment not found")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch attachment")
		return
	}

	// Set headers for inline display (preview)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	c.Header("Content-Length", strconv.Itoa(len(content)))
	c.Header("Cache-Control", "private, max-age=3600")

	// Send the file content
	c.Data(http.StatusOK, contentType, content)
}

// handleAdminAttachmentToggle toggles the validity of an attachment.
func handleAdminAttachmentToggle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid attachment ID",
		})
		return
	}

	var input struct {
		ValidID int `json:"valid_id"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE standard_attachment 
		SET valid_id = $1, change_time = CURRENT_TIMESTAMP, change_by = 1 
		WHERE id = $2
	`), input.ValidID, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update attachment status",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Attachment not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Attachment status updated successfully",
	})
}
