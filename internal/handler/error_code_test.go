package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/tmknom/gates/internal/authn"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/issuer"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/signer"
)

func TestErrorCodeOf(t *testing.T) {
	// 期待値は Lambda Design の「Failure」の表の失敗の種別とエラーコードの対応から取る
	tests := []struct {
		name    string
		failure error
		want    errorcode.Code
	}{
		{name: "invalid request maps to invalid_request", failure: request.ErrInvalidRequest, want: errorcode.InvalidRequest},
		{name: "authentication failure maps to invalid_id_token", failure: authn.ErrAuthenticationFailure, want: errorcode.InvalidIDToken},
		{name: "authn transient failure maps to identity_verification_unavailable", failure: authn.ErrTransientFailure, want: errorcode.IdentityVerificationUnavailable},
		{name: "authn internal failure maps to internal_error", failure: authn.ErrInternalFailure, want: errorcode.InternalError},
		{name: "authorization denied maps to authorization_denied", failure: authz.ErrAuthorizationDenied, want: errorcode.AuthorizationDenied},
		{name: "installation unavailable maps to installation_resolution_failed", failure: authz.ErrInstallationUnavailable, want: errorcode.InstallationResolutionFailed},
		{name: "signer transient failure maps to signing_unavailable", failure: signer.ErrTransientFailure, want: errorcode.SigningUnavailable},
		{name: "signer internal failure maps to internal_error", failure: signer.ErrInternalFailure, want: errorcode.InternalError},
		{name: "github transient failure maps to token_issuance_failed", failure: github.ErrTransientFailure, want: errorcode.TokenIssuanceFailed},
		{name: "github issuance rejected maps to github_issuance_rejected", failure: github.ErrIssuanceRejected, want: errorcode.GitHubIssuanceRejected},
		{name: "verification failure maps to issuance_verification_failed", failure: issuer.ErrVerificationFailure, want: errorcode.IssuanceVerificationFailed},
		{name: "failure of no failure kind maps to internal_error", failure: errors.New("unexpected"), want: errorcode.InternalError},
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

	if got := errorCodeOf(err); got != errorcode.InternalError {
		t.Errorf("errorCodeOf = %+v, want %+v", got, errorcode.InternalError)
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

	if got := errorCodeOf(err); got != errorcode.IssuanceVerificationFailed {
		t.Errorf("errorCodeOf = %+v, want %+v", got, errorcode.IssuanceVerificationFailed)
	}
}
