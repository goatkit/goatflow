package plugin

import (
	"sync"
	"time"
)

// LogEntry represents a single plugin log entry.
type LogEntry struct {
	Timestamp  time.Time      `json:"timestamp"`
	Plugin     string         `json:"plugin"`
	Level      string         `json:"level"` // debug, info, warn, error
	Message    string         `json:"message"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// LogBuffer is a ring buffer for plugin logs.
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	maxSize int
	head    int
	count   int
}

// NewLogBuffer creates a new log buffer with the given max size.
func NewLogBuffer(maxSize int) *LogBuffer {
	if maxSize <= 0 {
		maxSize = 1000 // Default to 1000 entries
	}
	return &LogBuffer{
		entries: make([]LogEntry, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a log entry to the buffer.
func (b *LogBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.maxSize
	if b.count < b.maxSize {
		b.count++
	}
}

// Log adds a log entry with the given parameters.
func (b *LogBuffer) Log(plugin, level, message string, fields map[string]any) {
	b.Add(LogEntry{
		Timestamp: time.Now(),
		Plugin:    plugin,
		Level:     level,
		Message:   message,
		Fields:    fields,
	})
}

// GetAll returns all log entries, newest first.
func (b *LogBuffer) GetAll() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]LogEntry, b.count)
	for i := 0; i < b.count; i++ {
		// Read in reverse order (newest first)
		idx := (b.head - 1 - i + b.maxSize) % b.maxSize
		result[i] = b.entries[idx]
	}
	return result
}

// GetByPlugin returns log entries for a specific plugin, newest first.
func (b *LogBuffer) GetByPlugin(pluginName string) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []LogEntry
	for i := 0; i < b.count; i++ {
		idx := (b.head - 1 - i + b.maxSize) % b.maxSize
		if b.entries[idx].Plugin == pluginName {
			result = append(result, b.entries[idx])
		}
	}
	return result
}

// GetByLevel returns log entries at or above the given level, newest first.
func (b *LogBuffer) GetByLevel(minLevel string) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	levelOrder := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3}
	minLevelNum := levelOrder[minLevel]

	var result []LogEntry
	for i := 0; i < b.count; i++ {
		idx := (b.head - 1 - i + b.maxSize) % b.maxSize
		entry := b.entries[idx]
		if levelOrder[entry.Level] >= minLevelNum {
			result = append(result, entry)
		}
	}
	return result
}

// GetRecent returns the most recent n entries, newest first.
func (b *LogBuffer) GetRecent(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n > b.count {
		n = b.count
	}

	result := make([]LogEntry, n)
	for i := 0; i < n; i++ {
		idx := (b.head - 1 - i + b.maxSize) % b.maxSize
		result[i] = b.entries[idx]
	}
	return result
}

// Clear removes all entries from the buffer.
func (b *LogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.head = 0
	b.count = 0
}

// Count returns the number of entries in the buffer.
func (b *LogBuffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Global log buffer instance
var globalLogBuffer *LogBuffer
var logBufferOnce sync.Once

// GetLogBuffer returns the global plugin log buffer.
func GetLogBuffer() *LogBuffer {
	logBufferOnce.Do(func() {
		globalLogBuffer = NewLogBuffer(1000)
	})
	return globalLogBuffer
}
