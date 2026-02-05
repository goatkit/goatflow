package plugin

import (
	"testing"
	"time"
)

func TestLogBuffer(t *testing.T) {
	t.Run("Add and GetAll", func(t *testing.T) {
		buf := NewLogBuffer(10)
		
		buf.Log("test-plugin", "info", "test message 1", nil)
		buf.Log("test-plugin", "error", "test message 2", map[string]any{"key": "value"})
		
		entries := buf.GetAll()
		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}
		
		// Newest first
		if entries[0].Message != "test message 2" {
			t.Errorf("expected newest first, got %s", entries[0].Message)
		}
		if entries[0].Level != "error" {
			t.Errorf("expected error level, got %s", entries[0].Level)
		}
	})
	
	t.Run("Ring buffer overflow", func(t *testing.T) {
		buf := NewLogBuffer(3)
		
		buf.Log("p1", "info", "msg1", nil)
		buf.Log("p1", "info", "msg2", nil)
		buf.Log("p1", "info", "msg3", nil)
		buf.Log("p1", "info", "msg4", nil) // Should overwrite msg1
		
		entries := buf.GetAll()
		if len(entries) != 3 {
			t.Errorf("expected 3 entries, got %d", len(entries))
		}
		
		// Should not contain msg1
		for _, e := range entries {
			if e.Message == "msg1" {
				t.Error("msg1 should have been overwritten")
			}
		}
	})
	
	t.Run("GetByPlugin", func(t *testing.T) {
		buf := NewLogBuffer(10)
		
		buf.Log("plugin-a", "info", "msg from a", nil)
		buf.Log("plugin-b", "info", "msg from b", nil)
		buf.Log("plugin-a", "error", "error from a", nil)
		
		entries := buf.GetByPlugin("plugin-a")
		if len(entries) != 2 {
			t.Errorf("expected 2 entries for plugin-a, got %d", len(entries))
		}
		
		for _, e := range entries {
			if e.Plugin != "plugin-a" {
				t.Errorf("expected plugin-a, got %s", e.Plugin)
			}
		}
	})
	
	t.Run("GetByLevel", func(t *testing.T) {
		buf := NewLogBuffer(10)
		
		buf.Log("p1", "debug", "debug msg", nil)
		buf.Log("p1", "info", "info msg", nil)
		buf.Log("p1", "warn", "warn msg", nil)
		buf.Log("p1", "error", "error msg", nil)
		
		// Get warn and above
		entries := buf.GetByLevel("warn")
		if len(entries) != 2 {
			t.Errorf("expected 2 entries (warn+error), got %d", len(entries))
		}
		
		// Get error only
		entries = buf.GetByLevel("error")
		if len(entries) != 1 {
			t.Errorf("expected 1 entry (error), got %d", len(entries))
		}
	})
	
	t.Run("GetRecent", func(t *testing.T) {
		buf := NewLogBuffer(10)
		
		for i := 0; i < 5; i++ {
			buf.Log("p1", "info", "msg", nil)
		}
		
		entries := buf.GetRecent(3)
		if len(entries) != 3 {
			t.Errorf("expected 3 entries, got %d", len(entries))
		}
		
		// Request more than available
		entries = buf.GetRecent(100)
		if len(entries) != 5 {
			t.Errorf("expected 5 entries, got %d", len(entries))
		}
	})
	
	t.Run("Clear", func(t *testing.T) {
		buf := NewLogBuffer(10)
		
		buf.Log("p1", "info", "msg", nil)
		buf.Log("p1", "info", "msg", nil)
		
		if buf.Count() != 2 {
			t.Errorf("expected 2, got %d", buf.Count())
		}
		
		buf.Clear()
		
		if buf.Count() != 0 {
			t.Errorf("expected 0 after clear, got %d", buf.Count())
		}
	})
	
	t.Run("Timestamp", func(t *testing.T) {
		buf := NewLogBuffer(10)
		
		before := time.Now()
		buf.Log("p1", "info", "msg", nil)
		after := time.Now()
		
		entries := buf.GetAll()
		if entries[0].Timestamp.Before(before) || entries[0].Timestamp.After(after) {
			t.Error("timestamp out of range")
		}
	})
}

func TestGlobalLogBuffer(t *testing.T) {
	buf := GetLogBuffer()
	if buf == nil {
		t.Error("global log buffer should not be nil")
	}
	
	// Should return same instance
	buf2 := GetLogBuffer()
	if buf != buf2 {
		t.Error("should return same instance")
	}
}

func TestNewLogBufferInvalidSize(t *testing.T) {
	t.Run("zero size defaults to 1000", func(t *testing.T) {
		buf := NewLogBuffer(0)
		if buf.maxSize != 1000 {
			t.Errorf("expected default 1000, got %d", buf.maxSize)
		}
	})

	t.Run("negative size defaults to 1000", func(t *testing.T) {
		buf := NewLogBuffer(-5)
		if buf.maxSize != 1000 {
			t.Errorf("expected default 1000, got %d", buf.maxSize)
		}
	})
}
