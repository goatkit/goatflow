package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNormalizeFormat(t *testing.T) {
	cases := map[string]string{
		"json":      FormatJSON,
		"JSON":      FormatJSON,
		" json ":    FormatJSON,
		"text":      FormatText,
		"":          FormatText,
		"screaming": FormatText, // unknown → text
	}
	for in, want := range cases {
		if got := NormalizeFormat(in); got != want {
			t.Errorf("NormalizeFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"noisy":   slog.LevelInfo, // unknown → info
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestJSONHandlerEmitsValidJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := TestConfigure(FormatJSON, slog.LevelInfo, &buf)
	logger.Info("hello", "component", "test")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, buf.String())
	}
	if rec["msg"] != "hello" || rec["component"] != "test" || rec["level"] != "INFO" {
		t.Fatalf("unexpected record: %v", rec)
	}
}

func TestTextHandlerEmitsKeyValues(t *testing.T) {
	var buf bytes.Buffer
	logger := TestConfigure(FormatText, slog.LevelInfo, &buf)
	logger.Info("hello", "component", "test")

	out := buf.String()
	if !strings.Contains(out, "msg=hello") || !strings.Contains(out, "component=test") {
		t.Fatalf("text output missing key=value pairs: %q", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := TestConfigure(FormatJSON, slog.LevelWarn, &buf)
	logger.Info("should be dropped")
	logger.Warn("should be kept")

	if strings.Contains(buf.String(), "should be dropped") {
		t.Fatalf("info message not filtered at warn level: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "should be kept") {
		t.Fatalf("warn message missing: %q", buf.String())
	}
}

func TestJSONStdlibWriter(t *testing.T) {
	var buf bytes.Buffer
	w := &jsonStdlibWriter{w: &buf}
	if _, err := w.Write([]byte("legacy line\n")); err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("stdlib JSON output invalid: %v (%q)", err, buf.String())
	}
	if rec["msg"] != "legacy line" || rec["level"] != "info" {
		t.Fatalf("unexpected stdlib record: %v", rec)
	}
}
