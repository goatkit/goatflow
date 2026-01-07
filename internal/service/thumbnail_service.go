package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

type ThumbnailService struct {
	redisClient *redis.Client
	cacheTTL    time.Duration
}

// NewThumbnailService creates a new thumbnail service with Redis caching.
func NewThumbnailService(redisClient *redis.Client) *ThumbnailService {
	return &ThumbnailService{
		redisClient: redisClient,
		cacheTTL:    7 * 24 * time.Hour, // Cache thumbnails for 7 days
	}
}

// ThumbnailOptions defines options for thumbnail generation.
type ThumbnailOptions struct {
	Width   int
	Height  int
	Quality int    // JPEG quality 1-100
	Format  string // "jpeg" or "png"
}

// DefaultThumbnailOptions returns sensible defaults.
func DefaultThumbnailOptions() ThumbnailOptions {
	return ThumbnailOptions{
		Width:   200,
		Height:  200,
		Quality: 85,
		Format:  "jpeg",
	}
}

// GenerateThumbnail generates a thumbnail from image data.
// Includes panic recovery to mitigate CVE-2023-36308 (crafted TIFF panic).
func (s *ThumbnailService) GenerateThumbnail(data []byte, contentType string, opts ThumbnailOptions) (thumbnailData []byte, outputFormat string, err error) {
	// Recover from panics (CVE-2023-36308: crafted TIFF can cause panic in imaging lib)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("image processing panic (possible malformed image): %v", r)
		}
	}()

	// Decode the image
	reader := bytes.NewReader(data)
	img, format, decodeErr := image.Decode(reader)
	if decodeErr != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", decodeErr)
	}

	// Resize the image maintaining aspect ratio
	thumbnail := imaging.Fit(img, opts.Width, opts.Height, imaging.Lanczos)

	// Encode the thumbnail
	var buf bytes.Buffer

	if opts.Format == "png" || format == "png" {
		if encErr := png.Encode(&buf, thumbnail); encErr != nil {
			return nil, "", fmt.Errorf("failed to encode thumbnail: %w", encErr)
		}
		outputFormat = "image/png"
	} else {
		if encErr := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: opts.Quality}); encErr != nil {
			return nil, "", fmt.Errorf("failed to encode thumbnail: %w", encErr)
		}
		outputFormat = "image/jpeg"
	}

	thumbnailData = buf.Bytes()
	return thumbnailData, outputFormat, nil
}

// GetOrCreateThumbnail gets a thumbnail from cache or generates it.
func (s *ThumbnailService) GetOrCreateThumbnail(ctx context.Context, attachmentID int, data []byte, contentType string, opts ThumbnailOptions) ([]byte, string, error) {
	// Generate cache key
	cacheKey := s.generateCacheKey(attachmentID, opts)

	// Try to get from cache
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		// Decode from base64
		thumbnailData, err := base64.StdEncoding.DecodeString(cached)
		if err == nil {
			// Get content type from cache
			contentTypeKey := cacheKey + ":type"
			cachedType, _ := s.redisClient.Get(ctx, contentTypeKey).Result()
			if cachedType == "" {
				cachedType = "image/jpeg"
			}
			return thumbnailData, cachedType, nil
		}
	}

	// Generate thumbnail
	thumbnailData, outputFormat, err := s.GenerateThumbnail(data, contentType, opts)
	if err != nil {
		return nil, "", err
	}

	// Cache the thumbnail
	encoded := base64.StdEncoding.EncodeToString(thumbnailData)
	pipe := s.redisClient.Pipeline()
	pipe.Set(ctx, cacheKey, encoded, s.cacheTTL)
	pipe.Set(ctx, cacheKey+":type", outputFormat, s.cacheTTL)
	_, _ = pipe.Exec(ctx)

	return thumbnailData, outputFormat, nil
}

// InvalidateThumbnail removes a thumbnail from cache.
func (s *ThumbnailService) InvalidateThumbnail(ctx context.Context, attachmentID int) error {
	// Remove all size variations
	pattern := fmt.Sprintf("thumbnail:%d:*", attachmentID)

	var cursor uint64
	for {
		keys, nextCursor, err := s.redisClient.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := s.redisClient.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// generateCacheKey creates a unique cache key for a thumbnail.
func (s *ThumbnailService) generateCacheKey(attachmentID int, opts ThumbnailOptions) string {
	// Create a hash of the options for uniqueness
	optStr := fmt.Sprintf("%dx%d-q%d-%s", opts.Width, opts.Height, opts.Quality, opts.Format)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(optStr)))
	return fmt.Sprintf("thumbnail:%d:%s", attachmentID, hash[:8])
}

// IsSupportedImageType checks if a content type can be thumbnailed.
func IsSupportedImageType(contentType string) bool {
	supportedTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
	}

	contentType = strings.ToLower(contentType)
	for _, supported := range supportedTypes {
		if contentType == supported {
			return true
		}
	}
	return false
}

// GetPlaceholderThumbnail returns a placeholder image for non-image files.
func GetPlaceholderThumbnail(contentType string) ([]byte, string) {
	// Simple SVG placeholder based on file type
	var svg string
	color := "#6B7280" // Default gray
	icon := "M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"

	if strings.HasPrefix(contentType, "video/") {
		color = "#9333EA" // Purple
		icon = "M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"
	} else if strings.HasPrefix(contentType, "audio/") {
		color = "#10B981" // Green
		icon = "M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"
	} else if contentType == "application/pdf" {
		color = "#EF4444" // Red
		icon = "M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
	} else if strings.Contains(contentType, "zip") || strings.Contains(contentType, "compressed") {
		color = "#F59E0B" // Yellow
		icon = "M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V2"
	}

	svg = fmt.Sprintf(`<svg width="200" height="200" xmlns="http://www.w3.org/2000/svg">
		<rect width="200" height="200" fill="#F3F4F6"/>
		<path d="%s" stroke="%s" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" transform="translate(60, 60) scale(4, 4)"/>
	</svg>`, icon, color)

	return []byte(svg), "image/svg+xml"
}
