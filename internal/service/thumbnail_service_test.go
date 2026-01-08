package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsSupportedImageType tests the IsSupportedImageType function
func TestIsSupportedImageType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		// Supported types
		{"JPEG", "image/jpeg", true},
		{"JPG lowercase", "image/jpg", true},
		{"PNG", "image/png", true},
		{"GIF", "image/gif", true},
		{"WebP", "image/webp", true},
		{"BMP", "image/bmp", true},
		{"TIFF", "image/tiff", true},
		{"AVIF", "image/avif", true},
		{"HEIC", "image/heic", true},
		{"HEIF", "image/heif", true},
		{"JPEG XL", "image/jxl", true},
		{"SVG", "image/svg+xml", true},

		// Uppercase should also work (case insensitive)
		{"JPEG uppercase", "IMAGE/JPEG", true},
		{"PNG uppercase", "IMAGE/PNG", true},

		// Unsupported types
		{"PDF", "application/pdf", false},
		{"Plain text", "text/plain", false},
		{"HTML", "text/html", false},
		{"Video MP4", "video/mp4", false},
		{"Audio MP3", "audio/mpeg", false},
		{"Word document", "application/msword", false},
		{"Empty string", "", false},
		{"Random string", "not-a-mime-type", false},
		{"application/octet-stream", "application/octet-stream", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSupportedImageType(tt.contentType)
			assert.Equal(t, tt.expected, result, "IsSupportedImageType(%q) = %v, want %v", tt.contentType, result, tt.expected)
		})
	}
}

// TestCalculateThumbnailScale tests the calculateThumbnailScale function
func TestCalculateThumbnailScale(t *testing.T) {
	tests := []struct {
		name             string
		srcWidth         int
		srcHeight        int
		maxWidth         int
		maxHeight        int
		expectedScale    float64
		description      string
	}{
		{
			name:          "Image fits within bounds",
			srcWidth:      100,
			srcHeight:     100,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 1.0,
			description:   "Small image should not be upscaled",
		},
		{
			name:          "Image exactly matches bounds",
			srcWidth:      200,
			srcHeight:     200,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 1.0,
			description:   "Image matching bounds should have scale 1.0",
		},
		{
			name:          "Image wider than bounds",
			srcWidth:      400,
			srcHeight:     200,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 0.5,
			description:   "Wide image should scale by width",
		},
		{
			name:          "Image taller than bounds",
			srcWidth:      200,
			srcHeight:     400,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 0.5,
			description:   "Tall image should scale by height",
		},
		{
			name:          "Large square image",
			srcWidth:      1000,
			srcHeight:     1000,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 0.2,
			description:   "Large square should scale to fit",
		},
		{
			name:          "Landscape image",
			srcWidth:      800,
			srcHeight:     600,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 0.25,
			description:   "Landscape should scale by width",
		},
		{
			name:          "Portrait image",
			srcWidth:      600,
			srcHeight:     800,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 0.25,
			description:   "Portrait should scale by height",
		},
		{
			name:          "Zero width source",
			srcWidth:      0,
			srcHeight:     100,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 1.0,
			description:   "Zero width should return 1.0",
		},
		{
			name:          "Zero height source",
			srcWidth:      100,
			srcHeight:     0,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 1.0,
			description:   "Zero height should return 1.0",
		},
		{
			name:          "Negative dimensions",
			srcWidth:      -100,
			srcHeight:     100,
			maxWidth:      200,
			maxHeight:     200,
			expectedScale: 1.0,
			description:   "Negative width should return 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateThumbnailScale(tt.srcWidth, tt.srcHeight, tt.maxWidth, tt.maxHeight)
			assert.InDelta(t, tt.expectedScale, result, 0.001, "%s: calculateThumbnailScale(%d, %d, %d, %d) = %v, want %v",
				tt.description, tt.srcWidth, tt.srcHeight, tt.maxWidth, tt.maxHeight, result, tt.expectedScale)
		})
	}
}

// TestGetPlaceholderThumbnail tests the GetPlaceholderThumbnail function
func TestGetPlaceholderThumbnail(t *testing.T) {
	tests := []struct {
		name         string
		contentType  string
		expectSVG    bool
		expectedType string
	}{
		{
			name:         "Image placeholder",
			contentType:  "image/png",
			expectSVG:    true,
			expectedType: "image/svg+xml",
		},
		{
			name:         "PDF placeholder",
			contentType:  "application/pdf",
			expectSVG:    true,
			expectedType: "image/svg+xml",
		},
		{
			name:         "Video placeholder",
			contentType:  "video/mp4",
			expectSVG:    true,
			expectedType: "image/svg+xml",
		},
		{
			name:         "Audio placeholder",
			contentType:  "audio/mpeg",
			expectSVG:    true,
			expectedType: "image/svg+xml",
		},
		{
			name:         "Zip placeholder",
			contentType:  "application/zip",
			expectSVG:    true,
			expectedType: "image/svg+xml",
		},
		{
			name:         "Generic file placeholder",
			contentType:  "application/octet-stream",
			expectSVG:    true,
			expectedType: "image/svg+xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, contentType := GetPlaceholderThumbnail(tt.contentType)

			assert.Equal(t, tt.expectedType, contentType, "Content type should be SVG")
			assert.NotEmpty(t, data, "Placeholder data should not be empty")

			if tt.expectSVG {
				// Check that it's a valid SVG
				assert.Contains(t, string(data), "<svg", "Placeholder should contain SVG element")
				assert.Contains(t, string(data), "</svg>", "Placeholder should have closing SVG tag")
			}
		})
	}
}

// TestDefaultThumbnailOptions tests the DefaultThumbnailOptions function
func TestDefaultThumbnailOptions(t *testing.T) {
	opts := DefaultThumbnailOptions()

	assert.Equal(t, 200, opts.Width, "Default width should be 200")
	assert.Equal(t, 200, opts.Height, "Default height should be 200")
	assert.Equal(t, 85, opts.Quality, "Default quality should be 85")
	assert.Equal(t, "jpeg", opts.Format, "Default format should be jpeg")
}
