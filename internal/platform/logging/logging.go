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
//     format-aware writer, so they respect the same LOG_FORMAT and
//     LOG_OUTPUT. In JSON mode each line goes through the slog JSON handler,
//     so both kinds of record share one layout ("level":"INFO" etc.).
//     stdlib log carries no level metadata, so its lines are always
//     emitted at INFO; only slog lines are filtered by LOG_LEVEL.
package logging

import (
	"context"
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
	return install(format, level, out)
}

// install makes a handler for format/level writing to out the slog default
// and routes the stdlib log package to the same destination and format.
func install(format string, level slog.Level, out io.Writer) *slog.Logger {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if format == FormatJSON {
		h = slog.NewJSONHandler(out, opts)
	} else {
		h = slog.NewTextHandler(out, opts)
	}
	logger := slog.New(h)
	slog.SetDefault(logger)

	// slog.SetDefault already points the stdlib log package at the slog
	// handler; set it explicitly so legacy lines skip the LOG_LEVEL filter
	// (they have no level) and keep their classic text layout in text mode.
	if format == FormatJSON {
		// No stdlib prefix: the slog handler adds time/level itself.
		log.SetOutput(&jsonStdlibWriter{h: h})
		log.SetFlags(0)
	} else {
		log.SetOutput(out)
		log.SetFlags(log.LstdFlags)
	}
	return logger
}

// jsonStdlibWriter turns plain stdlib log lines into INFO records of the
// slog JSON handler, so legacy and slog lines share one JSON layout.
type jsonStdlibWriter struct {
	h slog.Handler
}

func (j *jsonStdlibWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	// Handle (not Enabled+Handle): stdlib lines carry no level, so the
	// LOG_LEVEL filter must not drop them.
	if err := j.h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, msg, 0)); err != nil {
		return 0, err
	}
	return len(p), nil
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

// openOutput resolves LOG_OUTPUT. When a log file cannot be opened the
// logs go to stdout (the container log stream) and the reason is printed
// once to stderr, so a bad path never stops the process.
func openOutput(path string) (io.Writer, func()) {
	if path == "" || strings.EqualFold(path, "stdout") {
		return os.Stdout, nil
	}
	if strings.EqualFold(path, "stderr") {
		return os.Stderr, nil
	}
	// Relative paths are anchored to the working directory
	// (e.g. ./logs/goatflow.log).
	if !filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil {
			path = filepath.Join(cwd, path)
		}
	}
	// The path is operator configuration (LOG_OUTPUT/LOG_FILE_PATH), never request input.
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { // #nosec G703 -- operator-configured log path
		fmt.Fprintf(os.Stderr, "logging: cannot create log directory for %q, writing logs to stdout: %v\n", path, err)
		return os.Stdout, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304 G703 -- operator-configured log path
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: cannot open log file %q, writing logs to stdout: %v\n", path, err)
		return os.Stdout, nil
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

// TestConfigure installs a deterministic sink for unit tests: slog and the
// stdlib log package write to w exactly as Configure would.
func TestConfigure(format string, level slog.Level, w io.Writer) *slog.Logger {
	return install(format, level, w)
}
