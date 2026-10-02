package logging

import (
	"bytes"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"path/filepath"
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

// Legacy log.Printf lines and slog lines must share one JSON layout (same
// level spelling), and legacy lines must survive a stricter LOG_LEVEL.
func TestJSONStdlibLinesMatchSlogLevelCase(t *testing.T) {
	prevOut, prevFlags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	})

	var buf bytes.Buffer
	logger := TestConfigure(FormatJSON, slog.LevelWarn, &buf)
	log.Printf("legacy line")
	logger.Warn("slog line")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 JSON lines, got %d: %q", len(lines), buf.String())
	}
	var legacy, structured map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &legacy); err != nil {
		t.Fatalf("legacy line is not JSON: %v (%q)", err, lines[0])
	}
	if err := json.Unmarshal([]byte(lines[1]), &structured); err != nil {
		t.Fatalf("slog line is not JSON: %v (%q)", err, lines[1])
	}
	if legacy["msg"] != "legacy line" || legacy["level"] != "INFO" {
		t.Fatalf("legacy record = %v, want msg=legacy line level=INFO", legacy)
	}
	if structured["level"] != "WARN" {
		t.Fatalf("slog record = %v, want level=WARN", structured)
	}
	if _, ok := legacy["time"].(string); !ok {
		t.Fatalf("legacy record has no time: %v", legacy)
	}
}

// A log file that cannot be created falls back to stdout, as documented.
func TestOpenOutputFallsBackToStdout(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, closeFn := openOutput(filepath.Join(blocker, "goatflow.log"))
	if closeFn != nil {
		t.Fatal("no file should have been opened")
	}
	if w != os.Stdout {
		t.Fatalf("fallback writer = %v, want os.Stdout", w)
	}
}

// A created log file and its directory are not readable by other users:
// logs carry user and ticket data.
func TestOpenOutputCreatesOwnerOnlyFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	path := filepath.Join(dir, "goatflow.log")
	_, closeFn := openOutput(path)
	if closeFn == nil {
		t.Fatal("log file was not opened")
	}
	defer closeFn()

	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("log file mode = %v, want no group/other access", perm)
	}
	dst, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := dst.Mode().Perm(); perm&0o007 != 0 {
		t.Fatalf("log dir mode = %v, want no other access", perm)
	}
}
