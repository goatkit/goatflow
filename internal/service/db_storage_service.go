package service

import (
    "context"
    "database/sql"
    "fmt"
    "io"
    "mime/multipart"
    "strconv"
    "time"

    "github.com/gotrs-io/gotrs-ce/internal/database"
)

// Use keys defined in storage_service.go (same package)

// DatabaseStorageService implements StorageService by storing attachment bytes in the DB
type DatabaseStorageService struct {
    db *sql.DB
}

func NewDatabaseStorageService() (*DatabaseStorageService, error) {
    db, err := database.GetDB()
    if err != nil || db == nil {
        return nil, fmt.Errorf("database not available: %w", err)
    }
    return &DatabaseStorageService{db: db}, nil
}

// Store reads the file and inserts into article_data_mime_attachment for the article_id found in ctx
func (s *DatabaseStorageService) Store(ctx context.Context, file multipart.File, header *multipart.FileHeader, _ string) (*FileMetadata, error) {
    v := ctx.Value(CtxKeyArticleID)
    articleID, ok := v.(int)
    if !ok || articleID <= 0 {
        return nil, fmt.Errorf("article_id missing in context")
    }

    // Read all content
    content, err := io.ReadAll(file)
    if err != nil {
        return nil, fmt.Errorf("read file failed: %w", err)
    }

    now := time.Now()
    uid := 1
    if v, ok := ctx.Value(CtxKeyUserID).(int); ok && v > 0 { uid = v }
    contentType := header.Header.Get("Content-Type")

    // Insert attachment
    res, err := s.db.Exec(database.ConvertPlaceholders(`
        INSERT INTO article_data_mime_attachment (
            article_id, filename, content_type, content_size, content,
            disposition, create_time, create_by, change_time, change_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`),
        articleID,
        header.Filename,
        contentType,
        int64(len(content)),
        content,
        "attachment",
        now, uid, now, uid,
    )
    if err != nil {
        return nil, fmt.Errorf("attachment insert failed: %w", err)
    }

    attID, _ := res.LastInsertId()

    md := &FileMetadata{
        ID:           strconv.FormatInt(attID, 10),
        OriginalName: header.Filename,
        StoragePath:  "db://article_data_mime_attachment/" + strconv.FormatInt(attID, 10),
        ContentType:  contentType,
        Size:         int64(len(content)),
        UploadedAt:   now,
    }
    return md, nil
}

func (s *DatabaseStorageService) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
    return nil, fmt.Errorf("DB storage Retrieve not implemented: use dedicated download endpoint")
}

func (s *DatabaseStorageService) Delete(ctx context.Context, path string) error {
    return fmt.Errorf("DB storage Delete not implemented: use dedicated delete endpoint")
}

func (s *DatabaseStorageService) Exists(ctx context.Context, path string) (bool, error) {
    return false, nil
}

func (s *DatabaseStorageService) GetURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
    return "", fmt.Errorf("DB storage GetURL not supported")
}

func (s *DatabaseStorageService) GetMetadata(ctx context.Context, path string) (*FileMetadata, error) {
    return nil, fmt.Errorf("DB storage GetMetadata not implemented")
}
