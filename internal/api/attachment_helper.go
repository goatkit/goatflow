package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/storage"
)

type attachmentConfig struct {
	maxSize      int64
	allowedTypes map[string]struct{}
}

func loadAttachmentConfig() attachmentConfig {
	cfg := attachmentConfig{
		maxSize:      10 * 1024 * 1024,
		allowedTypes: map[string]struct{}{},
	}
	if appCfg := config.Get(); appCfg != nil {
		if appCfg.Storage.Attachments.MaxSize > 0 {
			cfg.maxSize = appCfg.Storage.Attachments.MaxSize
		}
		for _, t := range appCfg.Storage.Attachments.AllowedTypes {
			cfg.allowedTypes[strings.ToLower(t)] = struct{}{}
		}
	}
	return cfg
}

var attachmentBlockedExtensions = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".sh": true,
	".vbs": true, ".js": true, ".com": true, ".scr": true,
}

func isBlockedExtension(filename string) bool {
	return attachmentBlockedExtensions[strings.ToLower(filepath.Ext(filename))]
}

func isAllowedContentType(contentType string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	if contentType == "" || contentType == "application/octet-stream" {
		return true
	}
	_, ok := allowed[strings.ToLower(contentType)]
	return ok
}

func detectFileContentType(fh *multipart.FileHeader, f multipart.File) string {
	contentType := fh.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}
	buf := make([]byte, 512)
	n, _ := f.Read(buf) //nolint:errcheck // Best-effort read for detection
	if n > 0 {
		contentType = detectContentType(fh.Filename, buf[:n])
	}
	_, _ = f.Seek(0, 0) //nolint:errcheck // Reset to beginning
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

// attachmentProcessParams identifies the article that receives uploaded files.
// With tx set, the attachments are written inside that transaction (the
// article may not be committed yet).
type attachmentProcessParams struct {
	ctx       context.Context
	db        *sql.DB
	tx        *sql.Tx
	ticketID  int
	articleID int
	userID    int
}

func (p attachmentProcessParams) store() storage.ArticleStore {
	s := storage.ForDB(p.db)
	if p.tx != nil {
		s = s.WithTx(p.tx)
	}
	return s
}

// errAttachmentRejected marks an upload refused by the attachment policy
// (size, extension, content type).
var errAttachmentRejected = errors.New("attachment rejected")

// processFormAttachments stores every acceptable uploaded file on the article.
// Rejected or failing files are logged and skipped.
func processFormAttachments(files []*multipart.FileHeader, params attachmentProcessParams) {
	if len(files) == 0 {
		return
	}
	cfg := loadAttachmentConfig()
	for _, fh := range files {
		if fh == nil {
			continue
		}
		if _, err := processOneAttachment(fh, cfg, params); err != nil {
			log.Printf("attachment %q for ticket %d article %d not stored: %v",
				fh.Filename, params.ticketID, params.articleID, err)
		}
	}
}

// processOneAttachment checks one uploaded file against the attachment policy
// and writes it to the article's attachment storage.
func processOneAttachment(
	fh *multipart.FileHeader, cfg attachmentConfig, params attachmentProcessParams,
) (storage.Attachment, error) {
	if fh.Size > cfg.maxSize {
		return storage.Attachment{}, fmt.Errorf("%w: file too large (%d bytes, max %d)", errAttachmentRejected, fh.Size, cfg.maxSize)
	}
	if isBlockedExtension(fh.Filename) {
		return storage.Attachment{}, fmt.Errorf("%w: file type not allowed: %s",
			errAttachmentRejected, strings.ToLower(filepath.Ext(fh.Filename)))
	}
	f, err := fh.Open()
	if err != nil {
		return storage.Attachment{}, fmt.Errorf("open upload: %w", err)
	}
	defer f.Close()

	contentType := detectFileContentType(fh, f)
	if !isAllowedContentType(contentType, cfg.allowedTypes) {
		return storage.Attachment{}, fmt.Errorf("%w: file type not allowed: %s", errAttachmentRejected, contentType)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return storage.Attachment{}, fmt.Errorf("read upload: %w", err)
	}
	ctx := params.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return params.store().WriteAttachment(ctx, int64(params.articleID), storage.NewAttachment{
		Filename:    fh.Filename,
		ContentType: contentType,
		Disposition: "attachment",
		Content:     content,
		CreateBy:    params.userID,
	})
}

func getFormFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil || form.File == nil {
		return nil
	}
	files := form.File["attachments"]
	if files == nil {
		files = form.File["attachment"]
	}
	if files == nil {
		files = form.File["file"]
	}
	return files
}
