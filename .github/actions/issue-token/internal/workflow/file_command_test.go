package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendFileCommand(t *testing.T) {
	t.Run("name=value is appended after existing lines", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "output")
		if err := os.WriteFile(path, []byte("a=1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := AppendFileCommand(path, "b", "2"); err != nil {
			t.Fatalf("AppendFileCommand() error = %v", err)
		}
		assertFile(t, path, "a=1\nb=2\n")
	})

	t.Run("percent is written as is", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "output")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := AppendFileCommand(path, "token", "100%"); err != nil {
			t.Fatalf("AppendFileCommand() error = %v", err)
		}
		assertFile(t, path, "token=100%\n")
	})

	for _, tt := range []struct{ name, value string }{
		{"value with LF is rejected with ErrMultilineValue and the file is unchanged", "a\nb"},
		{"value with CR is rejected with ErrMultilineValue and the file is unchanged", "a\rb"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, []byte("a=1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := AppendFileCommand(path, "b", tt.value)
			if !errors.Is(err, ErrMultilineValue) {
				t.Fatalf("AppendFileCommand() error = %v, want ErrMultilineValue", err)
			}
			assertFile(t, path, "a=1\n")
		})
	}

	t.Run("missing file returns an error and is not created", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing")
		if err := AppendFileCommand(path, "a", "1"); err == nil {
			t.Fatal("AppendFileCommand() error = nil, want an error")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("file was created: stat error = %v", err)
		}
	})
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}
