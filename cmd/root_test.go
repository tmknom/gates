package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func runExecute(stdin, version, commit string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	env := Env{
		Stdin:   strings.NewReader(stdin),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Current: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		WorkDir: "/work",
		Version: version,
		Commit:  commit,
	}
	code := Execute(context.Background(), args, env)
	return code, stdout.String(), stderr.String()
}

func TestExecute_Run(t *testing.T) {
	code, stdout, stderr := runExecute("hello\n", "", "")
	if code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	for _, want := range []string{`"in": "hello"`, `"work_dir": "/work"`, `"time": "2026-01-02T03:04:05Z"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("got stdout %q, want it to contain %q", stdout, want)
		}
	}
	if stderr != "" {
		t.Errorf("got stderr %q, want empty", stderr)
	}
}

func TestExecute_Help(t *testing.T) {
	code, stdout, _ := runExecute("", "", "", "--help")
	if code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	if !strings.Contains(stdout, "Template repository for Go") {
		t.Errorf("got stdout %q, want help text", stdout)
	}
}

func TestExecute_Version(t *testing.T) {
	code, stdout, _ := runExecute("", "v1.2.3", "abc123", "--version")
	if code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	if !strings.Contains(stdout, "v1.2.3 (abc123)") {
		t.Errorf("got stdout %q, want version", stdout)
	}
}

func TestExecute_VersionWithoutBuildInfo(t *testing.T) {
	code, stdout, _ := runExecute("", "", "", "--version")
	if code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	if !strings.Contains(stdout, "dev") {
		t.Errorf("got stdout %q, want dev version", stdout)
	}
}

func TestExecute_UnexpectedArgs(t *testing.T) {
	code, stdout, stderr := runExecute("hello", "", "", "foo")
	if code != exitFailure {
		t.Errorf("got exit code %d, want %d", code, exitFailure)
	}
	if stdout != "" {
		t.Errorf("got stdout %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("got stderr %q, want unknown command error", stderr)
	}
}

func TestExecute_ReadError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := Env{Stdin: iotest.ErrReader(errors.New("boom")), Stdout: &stdout, Stderr: &stderr}
	code := Execute(context.Background(), nil, env)
	if code != exitFailure {
		t.Errorf("got exit code %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Errorf("got stderr %q, want read error", stderr.String())
	}
}

func TestExecute_Canceled(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		done <- Execute(ctx, nil, Env{Stdin: pr, Stdout: &stdout, Stderr: &stderr})
	}()

	cancel()
	select {
	case code := <-done:
		if code != exitInterrupted {
			t.Errorf("got exit code %d, want %d", code, exitInterrupted)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after cancel")
	}
}

func TestExecute_UnknownFlag(t *testing.T) {
	code, stdout, stderr := runExecute("", "", "", "--invalid")
	if code != exitFailure {
		t.Errorf("got exit code %d, want %d", code, exitFailure)
	}
	if stdout != "" {
		t.Errorf("got stdout %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unknown flag: --invalid") {
		t.Errorf("got stderr %q, want unknown flag error", stderr)
	}
}
