package v1

import (
	"encoding/base64"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

// handleGetTicketAttachments returns all attachments for a ticket's articles.
func (router *APIRouter) handleGetTicketAttachments(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid ticket ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	// Get all articles for the ticket (include internal notes for agents)
	articleRepo := repository.NewArticleRepository(db)
	articles, err := articleRepo.GetByTicketID(uint(ticketID), true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve articles",
		})
		return
	}

	// Collect all attachments from all articles
	var allAttachments []models.Attachment
	for _, article := range articles {
		attachments, err := articleRepo.GetAttachmentsByArticleID(uint(article.ID))
		if err != nil {
			continue // Skip articles with attachment retrieval errors
		}
		allAttachments = append(allAttachments, attachments...)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    allAttachments,
	})
}

// handleGetArticleAttachments returns attachments for a specific article.
func (router *APIRouter) handleGetArticleAttachments(c *gin.Context) {
	articleID, err := strconv.ParseUint(c.Param("article_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid article ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	articleRepo := repository.NewArticleRepository(db)
	attachments, err := articleRepo.GetAttachmentsByArticleID(uint(articleID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve attachments",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    attachments,
	})
}

// handleUploadTicketAttachment uploads a new attachment to a ticket article.
func (router *APIRouter) handleUploadTicketAttachment(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid ticket ID",
		})
		return
	}

	// Get article_id from form or query (attachment goes to a specific article)
	articleIDStr := c.PostForm("article_id")
	if articleIDStr == "" {
		articleIDStr = c.Query("article_id")
	}

	var articleID uint64
	if articleIDStr != "" {
		articleID, err = strconv.ParseUint(articleIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid article ID",
			})
			return
		}
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "No file uploaded",
		})
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to read file",
		})
		return
	}

	// Get current user
	userID := uint(1)
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = uint(idInt)
		}
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	// Verify ticket exists
	ticketRepo := repository.NewTicketRepository(db)
	ticket, err := ticketRepo.GetByID(uint(ticketID))
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Ticket not found",
		})
		return
	}

	articleRepo := repository.NewArticleRepository(db)

	// If no article ID provided, get/create the latest article
	if articleID == 0 {
		articles, err := articleRepo.GetByTicketID(uint(ticketID), true)
		if err != nil || len(articles) == 0 {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "No articles found for ticket. Create an article first.",
			})
			return
		}
		articleID = uint64(articles[len(articles)-1].ID)
	}

	// Detect content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}

	// Create attachment
	attachment := &models.Attachment{
		ArticleID:   uint(articleID),
		Filename:    header.Filename,
		ContentType: contentType,
		ContentSize: int(header.Size),
		Disposition: "attachment",
		Content:     base64.StdEncoding.EncodeToString(content),
		CreateBy:    userID,
		ChangeBy:    userID,
	}

	if err := articleRepo.CreateAttachment(attachment); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to save attachment",
		})
		return
	}

	// Return attachment without the content (for response size)
	attachment.Content = ""

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    attachment,
	})
}

// handleDownloadTicketAttachment downloads an attachment.
func (router *APIRouter) handleDownloadTicketAttachment(c *gin.Context) {
	attachmentID, err := strconv.ParseUint(c.Param("attachment_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid attachment ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	articleRepo := repository.NewArticleRepository(db)
	attachment, err := articleRepo.GetAttachmentByID(uint(attachmentID))
	if err != nil || attachment == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Attachment not found",
		})
		return
	}

	// Decode content from base64
	content, err := base64.StdEncoding.DecodeString(attachment.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to decode attachment",
		})
		return
	}

	c.Header("Content-Type", attachment.ContentType)
	c.Header("Content-Disposition", "attachment; filename=\""+attachment.Filename+"\"")
	c.Header("Content-Length", strconv.Itoa(len(content)))
	c.Data(http.StatusOK, attachment.ContentType, content)
}

// handleDeleteTicketAttachment deletes an attachment.
func (router *APIRouter) handleDeleteTicketAttachment(c *gin.Context) {
	attachmentID, err := strconv.ParseUint(c.Param("attachment_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid attachment ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	articleRepo := repository.NewArticleRepository(db)

	// Verify attachment exists
	attachment, err := articleRepo.GetAttachmentByID(uint(attachmentID))
	if err != nil || attachment == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Attachment not found",
		})
		return
	}

	if err := articleRepo.DeleteAttachment(uint(attachmentID)); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to delete attachment",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"message": "Attachment deleted successfully"},
	})
}
