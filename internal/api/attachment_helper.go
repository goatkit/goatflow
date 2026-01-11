package api

import (
	"context"
	"database/sql"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
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

type attachmentProcessParams struct {
	ctx       context.Context
	db        *sql.DB
	ticketID  int
	articleID int
	userID    int
}

func processFormAttachments(files []*multipart.FileHeader, params attachmentProcessParams) {
	if len(files) == 0 {
		return
	}

	cfg := loadAttachmentConfig()
	storageSvc := GetStorageService()

	for _, fh := range files {
		if fh == nil {
			continue
		}
		if fh.Size > cfg.maxSize {
			log.Printf("attachment too large: %s", fh.Filename)
			continue
		}
		if isBlockedExtension(fh.Filename) {
			log.Printf("blocked file type: %s", fh.Filename)
			continue
		}
		processOneAttachment(fh, cfg, storageSvc, params)
	}
}

func processOneAttachment(
	fh *multipart.FileHeader, cfg attachmentConfig, storageSvc service.StorageService, params attachmentProcessParams,
) {
	f, err := fh.Open()
	if err != nil {
		log.Printf("open attachment failed: %v", err)
		return
	}
	defer f.Close()

	contentType := detectFileContentType(fh, f)
	if !isAllowedContentType(contentType, cfg.allowedTypes) {
		log.Printf("type not allowed: %s %s", fh.Filename, contentType)
		return
	}

	ctx := service.WithUserID(params.ctx, params.userID)
	ctx = service.WithArticleID(ctx, params.articleID)
	storagePath := service.GenerateOTRSStoragePath(params.ticketID, params.articleID, fh.Filename)
	if _, err := storageSvc.Store(ctx, f, fh, storagePath); err != nil {
		log.Printf("storage Store failed: %v", err)
		return
	}

	if _, isDB := storageSvc.(*service.DatabaseStorageService); isDB {
		return
	}
	insertAttachmentMetadata(fh, contentType, params)
}

func insertAttachmentMetadata(fh *multipart.FileHeader, contentType string, params attachmentProcessParams) {
	f, err := fh.Open()
	if err != nil {
		return
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return
	}

	_, ierr := params.db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime_attachment (
			article_id, filename, content_type, content_size, content,
			disposition, create_time, create_by, change_time, change_by
		) VALUES (?,?,?,?,?,?,?,?,?,?)`),
		params.articleID,
		fh.Filename,
		contentType,
		int64(len(bytes)),
		bytes,
		"attachment",
		time.Now(), params.userID, time.Now(), params.userID,
	)
	if ierr != nil {
		log.Printf("attachment metadata insert failed: %v", ierr)
	}
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
