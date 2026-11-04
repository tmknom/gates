package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/tmknom/gates/internal/authn"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/issuer"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/signer"
)

func TestErrorCodeOf(t *testing.T) {
	// 期待値は Lambda Design の「Failure」の表から取る
	tests := []struct {
		name    string
		failure error
		want    errorCode
	}{
		{name: "invalid request maps to invalid_request", failure: request.ErrInvalidRequest, want: errorCode{code: "invalid_request", message: "The request payload is invalid."}},
		{name: "authentication failure maps to invalid_id_token", failure: authn.ErrAuthenticationFailure, want: errorCode{code: "invalid_id_token", message: "The ID token is not accepted."}},
		{name: "authn transient failure maps to identity_verification_unavailable", failure: authn.ErrTransientFailure, want: errorCode{code: "identity_verification_unavailable", message: "Identity verification is temporarily unavailable. Retry later."}},
		{name: "authn internal failure maps to internal_error", failure: authn.ErrInternalFailure, want: errorCode{code: "internal_error", message: "An internal error occurred."}},
		{name: "authorization denied maps to authorization_denied", failure: authz.ErrAuthorizationDenied, want: errorCode{code: "authorization_denied", message: "The requested token is not permitted."}},
		{name: "installation unavailable maps to installation_resolution_failed", failure: authz.ErrInstallationUnavailable, want: errorCode{code: "installation_resolution_failed", message: "The GitHub App installation is not available for issuance."}},
		{name: "signer transient failure maps to signing_unavailable", failure: signer.ErrTransientFailure, want: errorCode{code: "signing_unavailable", message: "Signing is temporarily unavailable. Retry later."}},
		{name: "signer internal failure maps to internal_error", failure: signer.ErrInternalFailure, want: errorCode{code: "internal_error", message: "An internal error occurred."}},
		{name: "github transient failure maps to token_issuance_failed", failure: github.ErrTransientFailure, want: errorCode{code: "token_issuance_failed", message: "The GitHub API is temporarily unavailable. Retry later."}},
		{name: "github issuance rejected maps to github_issuance_rejected", failure: github.ErrIssuanceRejected, want: errorCode{code: "github_issuance_rejected", message: "GitHub rejected the token issuance."}},
		{name: "verification failure maps to issuance_verification_failed", failure: issuer.ErrVerificationFailure, want: errorCode{code: "issuance_verification_failed", message: "The issued token did not pass verification and was not returned."}},
		{name: "failure of no failure kind maps to internal_error", failure: errors.New("unexpected"), want: errorCode{code: "internal_error", message: "An internal error occurred."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 部品は段階の名前や満たさなかった条件を足して包んで返す
			if got := errorCodeOf(fmt.Errorf("stage: %w: condition", tt.failure)); got != tt.want {
				t.Errorf("errorCodeOf = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestErrorCodeOf_FailureOfMultipleKindsMapsToInternalError(t *testing.T) {
	err := errors.Join(authz.ErrInstallationUnavailable, github.ErrTransientFailure)

	if got := errorCodeOf(err); got != internalError {
		t.Errorf("errorCodeOf = %+v, want %+v", got, internalError)
	}
}

// 照合失敗は、失効の失敗が一時的な障害であっても、照合失敗のまま写る。
func TestErrorCodeOf_VerificationFailureWithFailedRevocationMapsToIssuanceVerificationFailed(t *testing.T) {
	issuance := recordedIssuance()
	issuance["repository_selection"] = "all"
	_, err := issueInstallationToken(t, issuance, http.StatusServiceUnavailable)
	failure, ok := errors.AsType[*issuer.VerificationFailure](err)
	if !ok || failure.RevocationOutcome() != issuer.RevocationFailed {
		t.Fatalf("IssueInstallationToken error = %v, want *issuer.VerificationFailure with failed revocation", err)
	}

	if got := errorCodeOf(err); got != issuanceVerificationFailed {
		t.Errorf("errorCodeOf = %+v, want %+v", got, issuanceVerificationFailed)
	}
}
