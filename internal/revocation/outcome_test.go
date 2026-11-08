package revocation_test

import (
	"testing"

	"github.com/tmknom/gates/internal/revocation"
)

func TestOutcome_String(t *testing.T) {
	tests := []struct {
		name    string
		outcome revocation.Outcome
		want    string
	}{
		{name: "not attempted is not_attempted", outcome: revocation.NotAttempted, want: "not_attempted"},
		{name: "succeeded is succeeded", outcome: revocation.Succeeded, want: "succeeded"},
		{name: "failed is failed", outcome: revocation.Failed, want: "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.outcome.IsZero() {
				t.Errorf("IsZero() = true for %q, want false", tt.want)
			}
			if got := tt.outcome.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutcome_IsZero(t *testing.T) {
	if !(revocation.Outcome{}).IsZero() {
		t.Error("IsZero() = false for the zero Outcome, want true")
	}
}
