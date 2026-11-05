package errorcode_test

import (
	"testing"

	"github.com/tmknom/gates/internal/errorcode"
)

func TestCode_ValueAndMessage(t *testing.T) {
	// 期待値は Lambda Design の「Failure」の表から取る
	tests := []struct {
		code        errorcode.Code
		wantValue   string
		wantMessage string
	}{
		{code: errorcode.InvalidRequest, wantValue: "invalid_request", wantMessage: "The request payload is invalid."},
		{code: errorcode.InvalidIDToken, wantValue: "invalid_id_token", wantMessage: "The ID token is not accepted."},
		{code: errorcode.AuthorizationDenied, wantValue: "authorization_denied", wantMessage: "The requested token is not permitted."},
		{code: errorcode.IdentityVerificationUnavailable, wantValue: "identity_verification_unavailable", wantMessage: "Identity verification is temporarily unavailable. Retry later."},
		{code: errorcode.SigningUnavailable, wantValue: "signing_unavailable", wantMessage: "Signing is temporarily unavailable. Retry later."},
		{code: errorcode.InstallationResolutionFailed, wantValue: "installation_resolution_failed", wantMessage: "The GitHub App installation is not available for issuance."},
		{code: errorcode.TokenIssuanceFailed, wantValue: "token_issuance_failed", wantMessage: "The GitHub API is temporarily unavailable. Retry later."},
		{code: errorcode.GitHubIssuanceRejected, wantValue: "github_issuance_rejected", wantMessage: "GitHub rejected the token issuance."},
		{code: errorcode.IssuanceVerificationFailed, wantValue: "issuance_verification_failed", wantMessage: "The issued token did not pass verification and was not returned."},
		{code: errorcode.InternalError, wantValue: "internal_error", wantMessage: "An internal error occurred."},
	}
	for _, tt := range tests {
		t.Run(tt.wantValue, func(t *testing.T) {
			if got := tt.code.Value(); got != tt.wantValue {
				t.Errorf("Value() = %q, want %q", got, tt.wantValue)
			}
			if got := tt.code.Message(); got != tt.wantMessage {
				t.Errorf("Message() = %q, want %q", got, tt.wantMessage)
			}
			if tt.code.IsZero() {
				t.Error("IsZero() = true for a defined error code")
			}
		})
	}
}

func TestCode_ZeroValueIsZero(t *testing.T) {
	if !(errorcode.Code{}).IsZero() {
		t.Error("IsZero() = false for the zero value")
	}
}
