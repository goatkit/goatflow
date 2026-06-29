// Package cache provides caching utilities including compression support.
package cache

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"sync"
)

var (
	gzipWriterPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(nil)
		},
	}

	gzipReaderPool = sync.Pool{
		New: func() interface{} {
			reader, _ := gzip.NewReader(nil)
			return reader
		},
	}

	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

// compress compresses data using gzip.
func compress(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Get gzip writer from pool
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(buf)
	defer gzipWriterPool.Put(gz)

	// Write compressed data
	if _, err := gz.Write(data); err != nil {
		return data // Return original on error
	}

	if err := gz.Close(); err != nil {
		return data // Return original on error
	}

	// Return compressed data
	compressed := make([]byte, buf.Len())
	copy(compressed, buf.Bytes())

	// Only return compressed if it's smaller
	if len(compressed) < len(data) {
		return compressed
	}

	return data
}

// decompress decompresses gzip data.
func decompress(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	// Check if data is gzipped (magic number)
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data // Not gzipped, return as-is
	}

	// Get reader from pool
	reader := gzipReaderPool.Get().(*gzip.Reader)
	defer gzipReaderPool.Put(reader)

	// Reset reader with new data
	if err := reader.Reset(bytes.NewReader(data)); err != nil {
		return data // Return original on error
	}

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Read decompressed data
	if _, err := io.Copy(buf, reader); err != nil {
		return data // Return original on error
	}

	// Return decompressed data
	decompressed := make([]byte, buf.Len())
	copy(decompressed, buf.Bytes())

	return decompressed
}

// compressString compresses a string and returns base64.
//
//nolint:unused
func compressString(s string) string {
	compressed := compress([]byte(s))
	return base64.StdEncoding.EncodeToString(compressed)
}

// decompressString decompresses a base64 string.
//
//nolint:unused
func decompressString(s string) string {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s // Return original on error
	}

	decompressed := decompress(data)
	return string(decompressed)
}

// CompressionRatio calculates the compression ratio.
func CompressionRatio(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0
	}
	return float64(len(compressed)) / float64(len(original))
}

// ShouldCompress determines if data should be compressed based on size and type.
func ShouldCompress(data []byte) bool {
	// Don't compress small data
	if len(data) < 1024 {
		return false
	}

	// Don't compress already compressed data (check for common magic numbers)
	if len(data) >= 2 {
		// Gzip
		if data[0] == 0x1f && data[1] == 0x8b {
			return false
		}
		// PNG
		if data[0] == 0x89 && data[1] == 0x50 {
			return false
		}
		// JPEG
		if data[0] == 0xff && data[1] == 0xd8 {
			return false
		}
		// ZIP
		if data[0] == 0x50 && data[1] == 0x4b {
			return false
		}
	}

	// Check entropy (simple heuristic - count unique bytes)
	uniqueBytes := make(map[byte]bool)
	for _, b := range data {
		uniqueBytes[b] = true
		if len(uniqueBytes) > 200 {
			// High entropy, likely already compressed
			return false
		}
	}

	return true
}
