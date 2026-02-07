package v1

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// File handlers - serve article attachments from database.

func (router *APIRouter) handleUploadFile(c *gin.Context) {
	// File upload is handled via article attachment creation
	// This endpoint is for standalone file uploads (future use)
	sendError(c, http.StatusNotImplemented, "Use article attachment endpoints for file uploads")
}

func (router *APIRouter) handleDownloadFile(c *gin.Context) {
	fileIDStr := c.Param("id")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT filename, content_type, content
		FROM article_data_mime_attachment
		WHERE id = ?
	`)

	var filename, contentType string
	var content []byte
	if err := db.QueryRow(query, fileID).Scan(&filename, &contentType, &content); err != nil {
		sendError(c, http.StatusNotFound, "File not found")
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, contentType, content)
}

func (router *APIRouter) handleDeleteFile(c *gin.Context) {
	fileIDStr := c.Param("id")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`DELETE FROM article_data_mime_attachment WHERE id = ?`)
	result, err := db.Exec(query, fileID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete file")
		return
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		sendError(c, http.StatusNotFound, "File not found")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (router *APIRouter) handleGetFileInfo(c *gin.Context) {
	fileIDStr := c.Param("id")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT adma.id, adma.article_id, adma.filename, adma.content_type,
			LENGTH(adma.content) as size, adma.create_time, u.login
		FROM article_data_mime_attachment adma
		LEFT JOIN article a ON a.id = adma.article_id
		LEFT JOIN users u ON u.id = a.create_by
		WHERE adma.id = ?
	`)

	var id, articleID int
	var filename, contentType string
	var size int64
	var createTime time.Time
	var createdBy *string

	if err := db.QueryRow(query, fileID).Scan(&id, &articleID, &filename, &contentType, &size, &createTime, &createdBy); err != nil {
		sendError(c, http.StatusNotFound, "File not found")
		return
	}

	file := gin.H{
		"id":           id,
		"article_id":   articleID,
		"filename":     filename,
		"content_type": contentType,
		"size":         size,
		"created_at":   createTime,
	}
	if createdBy != nil {
		file["created_by"] = *createdBy
	}

	// Generate download URL
	file["download_url"] = "/api/v1/files/" + strconv.Itoa(id) + "/download"

	// Base64 preview for small files (< 1MB)
	if size < 1024*1024 {
		var content []byte
		contentQuery := database.ConvertQuery(`SELECT content FROM article_data_mime_attachment WHERE id = ?`)
		if db.QueryRow(contentQuery, fileID).Scan(&content) == nil {
			file["content_base64"] = base64.StdEncoding.EncodeToString(content)
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    file,
	})
}
