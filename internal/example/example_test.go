package example

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestRun(t *testing.T) {
	current := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "trailing newline is trimmed",
			in:   "hello\n",
			want: "{\n  \"in\": \"hello\",\n  \"work_dir\": \"/work\",\n  \"time\": \"2026-01-02T03:04:05Z\"\n}\n",
		},
		{
			name: "only one trailing line terminator is trimmed",
			in:   "a\n\n",
			want: "{\n  \"in\": \"a\\n\",\n  \"work_dir\": \"/work\",\n  \"time\": \"2026-01-02T03:04:05Z\"\n}\n",
		},
		{
			name: "crlf is trimmed",
			in:   "a\r\n",
			want: "{\n  \"in\": \"a\",\n  \"work_dir\": \"/work\",\n  \"time\": \"2026-01-02T03:04:05Z\"\n}\n",
		},
		{
			name: "lone trailing cr is kept",
			in:   "a\r",
			want: "{\n  \"in\": \"a\\r\",\n  \"work_dir\": \"/work\",\n  \"time\": \"2026-01-02T03:04:05Z\"\n}\n",
		},
		{
			name: "cr before trailing lf is trimmed only as crlf",
			in:   "a\r\n\r\n",
			want: "{\n  \"in\": \"a\\r\\n\",\n  \"work_dir\": \"/work\",\n  \"time\": \"2026-01-02T03:04:05Z\"\n}\n",
		},
		{
			name: "empty input",
			in:   "",
			want: "{\n  \"in\": \"\",\n  \"work_dir\": \"/work\",\n  \"time\": \"2026-01-02T03:04:05Z\"\n}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(context.Background(), Config{In: strings.NewReader(tt.in), Out: &out, WorkDir: "/work", Current: current})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, out.String()); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRun_InvalidUTF8(t *testing.T) {
	var out bytes.Buffer
	err := Run(context.Background(), Config{In: strings.NewReader("\xff\xfe"), Out: &out})
	if err == nil {
		t.Fatal("got no error, want invalid UTF-8 error")
	}
	if out.Len() != 0 {
		t.Errorf("got output %q, want empty", out.String())
	}
}

func TestRun_InputSizeLimit(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "at the limit", size: maxInputSize, wantErr: false},
		{name: "over the limit", size: maxInputSize + 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(context.Background(), Config{In: strings.NewReader(strings.Repeat("a", tt.size)), Out: &out})
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, want error: %v", err, tt.wantErr)
			}
			if tt.wantErr && out.Len() != 0 {
				t.Errorf("got output of %d bytes, want empty", out.Len())
			}
		})
	}
}

func TestRun_ReadError(t *testing.T) {
	want := errors.New("boom")
	var out bytes.Buffer
	err := Run(context.Background(), Config{In: iotest.ErrReader(want), Out: &out})
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRun_WriteError(t *testing.T) {
	want := errors.New("boom")
	err := Run(context.Background(), Config{In: strings.NewReader("a"), Out: errWriter{err: want}})
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

func TestRun_Canceled(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		var out bytes.Buffer
		done <- Run(ctx, Config{In: pr, Out: &out})
	}()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got error %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
