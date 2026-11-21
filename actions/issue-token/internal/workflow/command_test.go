package workflow

import (
	"bytes"
	"errors"
	"testing"
)

type exposer string

func (e exposer) Expose() string { return string(e) }

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestAddMask_WritesValuesInOrder(t *testing.T) {
	var buf bytes.Buffer
	if err := AddMask(&buf, exposer("a"), exposer("b")); err != nil {
		t.Fatalf("AddMask() error = %v", err)
	}
	if want := "::add-mask::a\n::add-mask::b\n"; buf.String() != want {
		t.Errorf("AddMask() wrote %q, want %q", buf.String(), want)
	}
}

func TestAddMask(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"value is written as one add-mask line", "abc", "::add-mask::abc\n"},
		{"percent, CR and LF are encoded", "a%b\r\nc", "::add-mask::a%25b%0D%0Ac\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := AddMask(&buf, exposer(tt.value)); err != nil {
				t.Fatalf("AddMask() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("AddMask() wrote %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		if err := AddMask(failingWriter{}, exposer("abc")); !errors.Is(err, errWrite) {
			t.Errorf("AddMask() error = %v, want %v", err, errWrite)
		}
	})
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	if err := Error(&buf, "line 1 100%\n::add-mask::injected"); err != nil {
		t.Fatalf("Error() error = %v", err)
	}
	want := "::error::line 1 100%25%0A::add-mask::injected\n"
	if got := buf.String(); got != want {
		t.Errorf("Error() wrote %q, want %q", got, want)
	}
}

func TestWarning(t *testing.T) {
	var buf bytes.Buffer
	if err := Warning(&buf, "careful\r\n100%"); err != nil {
		t.Fatalf("Warning() error = %v", err)
	}
	want := "::warning::careful%0D%0A100%25\n"
	if got := buf.String(); got != want {
		t.Errorf("Warning() wrote %q, want %q", got, want)
	}
}
