//go:build linux

package main

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

const closeProcChildEnv = "GOATFLOW_TEST_CLOSE_PROC_CHILD"

// TestCloseProcFilesChild is the child half of TestCloseProcFiles: it closes
// its /proc files, checks it can still use its own, says "ready" and waits
// for stdin to close.
func TestCloseProcFilesChild(t *testing.T) {
	if os.Getenv(closeProcChildEnv) != "1" {
		t.Skip("child process of TestCloseProcFiles")
	}
	closeProcFiles()
	if _, err := os.ReadFile("/proc/self/status"); err != nil {
		t.Fatalf("own /proc/self/status: %v", err)
	}
	if _, err := os.ReadDir("/proc/self/fd"); err != nil {
		t.Fatalf("own /proc/self/fd: %v", err)
	}
	if _, err := os.Executable(); err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	os.Stdout.WriteString("ready\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

// A same-user process (here the test, standing in for a plugin) can no
// longer read the environment of a process that closed its /proc files.
func TestCloseProcFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read every process's /proc files")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCloseProcFilesChild$", "-test.v")
	cmd.Env = append(os.Environ(), closeProcChildEnv+"=1", "GOATFLOW_TEST_SECRET=hunter2")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	lines := bufio.NewScanner(stdout)
	for lines.Scan() {
		output.WriteString(lines.Text() + "\n")
		if lines.Text() == "ready" {
			break
		}
	}

	env, readErr := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/environ")
	_ = stdin.Close()
	_, _ = io.Copy(&output, stdout)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("child failed: %v\n%s", err, output.String())
	}
	if !errors.Is(readErr, fs.ErrPermission) {
		t.Errorf("reading the child's environ: want permission denied, got err=%v, %d bytes (secret visible: %t)",
			readErr, len(env), strings.Contains(string(env), "hunter2"))
	}
}
