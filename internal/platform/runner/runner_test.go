package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/logging"
)

// blockingTask starts every second and blocks until its context ends, or
// forever when ignoreCtx is set (a task that does not honour cancellation).
type blockingTask struct {
	ignoreCtx bool
	started   chan struct{}
	once      sync.Once
	ctxErr    chan error
	release   chan struct{}
}

func newBlockingTask(ignoreCtx bool) *blockingTask {
	return &blockingTask{
		ignoreCtx: ignoreCtx,
		started:   make(chan struct{}),
		ctxErr:    make(chan error, 1),
		release:   make(chan struct{}),
	}
}

func (b *blockingTask) Name() string           { return "blocking" }
func (b *blockingTask) Schedule() string       { return "* * * * * *" }
func (b *blockingTask) Timeout() time.Duration { return 5 * time.Minute }

func (b *blockingTask) Run(ctx context.Context) error {
	first := false
	b.once.Do(func() { first = true; close(b.started) })
	if !first {
		return nil
	}
	if b.ignoreCtx {
		<-b.release
		return nil
	}
	<-ctx.Done()
	b.ctxErr <- ctx.Err()
	return ctx.Err()
}

// startRunner runs a runner over task. quiet gives it a discarding logger so
// a task left running by one test cannot write into another test's output.
func startRunner(t *testing.T, task Task, stopTimeout time.Duration, quiet bool) (context.CancelFunc, chan error) {
	t.Helper()
	reg := NewTaskRegistry()
	reg.Register(task)
	r := NewRunner(reg, stopTimeout)
	if quiet {
		r.logger = slog.New(slog.DiscardHandler)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Start(ctx) }()
	return cancel, done
}

// Stopping the runner (SIGTERM in production) must cancel running tasks
// right away, not leave them to run out their 5-minute task timeout.
func TestRunnerStopCancelsRunningTasks(t *testing.T) {
	task := newBlockingTask(false)
	cancel, done := startRunner(t, task, 3*time.Second, true)

	select {
	case <-task.started:
	case <-time.After(3 * time.Second):
		t.Fatal("task never started")
	}
	cancel()

	select {
	case err := <-task.ctxErr:
		if err != context.Canceled {
			t.Fatalf("task context ended with %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("running task was not cancelled on stop")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned %v after a clean stop, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after its tasks finished")
	}
}

// A task that ignores cancellation must not hold the process past the stop
// timeout.
func TestRunnerStopIsBounded(t *testing.T) {
	task := newBlockingTask(true)
	t.Cleanup(func() { close(task.release) })
	cancel, done := startRunner(t, task, 300*time.Millisecond, true)

	select {
	case <-task.started:
	case <-time.After(3 * time.Second):
		t.Fatal("task never started")
	}
	stopAt := time.Now()
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Start returned nil although a task was still running")
		}
		if waited := time.Since(stopAt); waited > 2*time.Second {
			t.Fatalf("Start waited %s, want about the 300ms stop timeout", waited)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return at the stop timeout")
	}
}

// Runner log lines follow LOG_FORMAT/LOG_OUTPUT like the rest of the process.
func TestRunnerLogsThroughConfiguredLogger(t *testing.T) {
	prevOut, prevFlags := log.Writer(), log.Flags()
	var buf syncBuffer
	logging.TestConfigure(logging.FormatJSON, slog.LevelInfo, &buf)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	})

	task := newBlockingTask(false)
	cancel, done := startRunner(t, task, time.Second, false)
	<-task.started
	cancel()
	<-done

	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("runner wrote nothing to the configured log output")
	}
	sawStart := false
	for _, line := range strings.Split(out, "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("runner log line is not JSON: %v (%q)", err, line)
		}
		if rec["component"] != "runner" {
			t.Fatalf("runner log line without component=runner: %v", rec)
		}
		if rec["msg"] == "starting task runner" && rec["level"] == "INFO" {
			sawStart = true
		}
	}
	if !sawStart {
		t.Fatalf("no INFO start record in %q", out)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
