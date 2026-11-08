package audit_test

import (
	"testing"

	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/revocation"
)

func TestFailed_ZeroErrorCodePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Failed() with the zero error code did not panic")
		}
	}()
	audit.Failed(errorcode.Code{}, "reason")
}

func TestVerificationFailed_ZeroOutcomePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("VerificationFailed() with the zero revocation.Outcome did not panic")
		}
	}()
	var outcome revocation.Outcome
	audit.VerificationFailed(errorcode.IssuanceVerificationFailed, "reason", outcome)
}
