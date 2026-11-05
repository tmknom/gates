package audit_test

import (
	"testing"

	"github.com/tmknom/gates/internal/audit"
)

func TestFailed_InvariantViolationPanics(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		reason string
	}{
		{name: "empty code panics", code: "", reason: "reason"},
		{name: "success as code panics", code: "success", reason: "reason"},
		{name: "empty reason panics", code: "internal_error", reason: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("Failed(%q, %q) did not panic", tt.code, tt.reason)
				}
			}()
			audit.Failed(tt.code, tt.reason)
		})
	}
}

func TestVerificationFailed_ZeroOutcomePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("VerificationFailed() with the zero RevocationOutcome did not panic")
		}
	}()
	var outcome audit.RevocationOutcome
	audit.VerificationFailed("issuance_verification_failed", "reason", outcome)
}

// TestVerificationFailed_InvalidResultPanics は、照合失敗の結果も失敗の結果と同じ条件で作ることを確かめる。
func TestVerificationFailed_InvalidResultPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("VerificationFailed() with an empty reason did not panic")
		}
	}()
	audit.VerificationFailed("issuance_verification_failed", "", audit.RevocationFailed)
}
