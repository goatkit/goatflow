// Package logging configures process-wide structured logging for GoatFlow.
//
// It honours three environment variables (documented in .env.example):
//
//	LOG_FORMAT=json|text     — output format (default: text)
//	LOG_LEVEL=debug|info|warn|error — minimum level for slog (default: info)
//	LOG_OUTPUT=stdout|<path> — where logs are written (default: stdout).
//	LOG_FILE_PATH=<path>     — legacy alias: used as the destination when
//	                           LOG_OUTPUT is unset/empty, so existing
//	                           deployments keep their log file.
//
// Two log sinks are configured so the whole codebase behaves consistently:
//
//  1. slog — a level-aware JSON or Text handler becomes the default logger
//     (slog.SetDefault), so every slog.* call in the codebase is structured.
//  2. stdlib log — legacy log.Printf call sites are routed through a
//     level- and format-aware writer, so they respect the same LOG_FORMAT
//     and LOG_OUTPUT. stdlib log carries no level metadata, so its lines are
//     always emitted (tagged "info" in JSON mode); only slog lines are
//     filtered by LOG_LEVEL.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

// Configure sets up slog and the stdlib log package from the environment.
// It is safe to call more than once (last call wins) and returns the
// configured slog.Logger for callers that prefer it.
func Configure() *slog.Logger {
	format := NormalizeFormat(os.Getenv("LOG_FORMAT"))
	level := ParseLevel(os.Getenv("LOG_LEVEL"))
	outPath := strings.TrimSpace(os.Getenv("LOG_OUTPUT"))
	if outPath == "" {
		// LOG_FILE_PATH is the legacy name for the log destination; honour
		// it when LOG_OUTPUT is unset so existing deployments keep their
		// file-based logging.
		outPath = strings.TrimSpace(os.Getenv("LOG_FILE_PATH"))
	}

	out, closeFn := openOutput(outPath)
	if closeFn != nil {
		registerCleanup(closeFn)
	}

	// --- slog ---------------------------------------------------------
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if format == FormatJSON {
		h = slog.NewJSONHandler(out, opts)
	} else {
		h = slog.NewTextHandler(out, opts)
	}
	logger := slog.New(h)
	slog.SetDefault(logger)

	// --- stdlib log ---------------------------------------------------
	var stdFlags int
	if format == FormatJSON {
		// No stdlib prefix: the JSON writer adds time/level itself.
		stdFlags = 0
	} else {
		stdFlags = log.LstdFlags
	}
	var w io.Writer = out
	if format == FormatJSON {
		w = &jsonStdlibWriter{w: out}
	}
	log.SetOutput(w)
	log.SetFlags(stdFlags)

	return logger
}

// jsonStdlibWriter converts plain stdlib log lines into JSON records with
// time + level so they parse alongside slog JSON output.
type jsonStdlibWriter struct {
	w io.Writer
}

func (j *jsonStdlibWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	rec := map[string]any{
		"time":  time.Now().Format(time.RFC3339Nano),
		"level": "info",
		"msg":   msg,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		// Extremely unlikely; fall back to raw so we never lose the line.
		return j.w.Write(p)
	}
	return j.w.Write(append(b, '\n'))
}

// closeHooks lets tests observe that the log file gets closed.
var closeHooks []func()

func registerCleanup(closeFn func()) {
	if closeFn == nil {
		return
	}
	closeHooks = append(closeHooks, closeFn)
}

// RunCleanupHooks closes any managed log files. Called from main on exit.
func RunCleanupHooks() {
	for _, fn := range closeHooks {
		fn()
	}
	closeHooks = nil
}

func openOutput(path string) (io.Writer, func()) {
	if path == "" || strings.EqualFold(path, "stdout") || strings.EqualFold(path, "stderr") {
		if strings.EqualFold(path, "stderr") {
			return os.Stderr, nil
		}
		return os.Stdout, nil
	}
	// Relative paths are anchored to the working directory
	// (e.g. ./logs/goatflow.log).
	if !filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil {
			path = filepath.Join(cwd, path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		// Can't create the log directory — fall back to stdout rather than
		// crashing the process over logging.
		return os.Stderr, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return os.Stderr, nil
	}
	return f, func() { _ = f.Close() }
}

func NormalizeFormat(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case FormatJSON:
		return FormatJSON
	case "", FormatText:
		return FormatText
	default:
		// Unknown format → fall back to text, but leave a trace on stderr
		// so operators notice the typo.
		fmt.Fprintf(os.Stderr, "logging: unknown LOG_FORMAT %q, defaulting to text\n", v)
		return FormatText
	}
}

func ParseLevel(v string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "", "info":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// TestConfigure is exported for unit tests that need a deterministic sink.
func TestConfigure(format string, level slog.Level, w io.Writer) *slog.Logger {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if format == FormatJSON {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}
