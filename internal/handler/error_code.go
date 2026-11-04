package handler

import (
	"errors"

	"github.com/tmknom/gates/internal/authn"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/issuer"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/signer"
)

// errorCode は、失敗の種別を応答で表すエラーコードと、失敗応答に書く固定の文言の組である。
// 文言をエラーコードと一組で持ち、失敗応答の文言がエラーコードだけで決まるようにする。
type errorCode struct {
	code    string
	message string
}

// 値と文言は Lambda Design の「Failure」の表に従う。
var (
	invalidRequest                  = errorCode{code: "invalid_request", message: "The request payload is invalid."}
	invalidIDToken                  = errorCode{code: "invalid_id_token", message: "The ID token is not accepted."}
	authorizationDenied             = errorCode{code: "authorization_denied", message: "The requested token is not permitted."}
	identityVerificationUnavailable = errorCode{code: "identity_verification_unavailable", message: "Identity verification is temporarily unavailable. Retry later."}
	signingUnavailable              = errorCode{code: "signing_unavailable", message: "Signing is temporarily unavailable. Retry later."}
	installationResolutionFailed    = errorCode{code: "installation_resolution_failed", message: "The GitHub App installation is not available for issuance."}
	tokenIssuanceFailed             = errorCode{code: "token_issuance_failed", message: "The GitHub API is temporarily unavailable. Retry later."}
	githubIssuanceRejected          = errorCode{code: "github_issuance_rejected", message: "GitHub rejected the token issuance."}
	issuanceVerificationFailed      = errorCode{code: "issuance_verification_failed", message: "The issued token did not pass verification and was not returned."}
	internalError                   = errorCode{code: "internal_error", message: "An internal error occurred."}
)

// errorCodesByFailureKind は、部品が返す失敗の種別の sentinel error を、エラーコードに対応づける。
// 一時的な障害だけが依存先ごとに行を分ける理由は、Lambda Design の「Failure」に記述する。
// 内部障害の行は、どの種別にも当たらない失敗と同じエラーコードになるが、部品が定めた種別を表と照合できるよう明示する。
var errorCodesByFailureKind = []struct {
	kind error
	code errorCode
}{
	{kind: request.ErrInvalidRequest, code: invalidRequest},
	{kind: authn.ErrAuthenticationFailure, code: invalidIDToken},
	{kind: authn.ErrTransientFailure, code: identityVerificationUnavailable},
	{kind: authn.ErrInternalFailure, code: internalError},
	{kind: authz.ErrAuthorizationDenied, code: authorizationDenied},
	{kind: authz.ErrInstallationUnavailable, code: installationResolutionFailed},
	{kind: signer.ErrTransientFailure, code: signingUnavailable},
	{kind: signer.ErrInternalFailure, code: internalError},
	{kind: github.ErrTransientFailure, code: tokenIssuanceFailed},
	{kind: github.ErrIssuanceRejected, code: githubIssuanceRejected},
	{kind: issuer.ErrVerificationFailure, code: issuanceVerificationFailed},
}

// errorCodeOf は、部品が返した失敗をエラーコードに写す。
// どの種別にも当たらない失敗と、複数の種別に当たる失敗は internal_error に写す。理由は Lambda Design の「Failure」に記述する。
// panic にしないのは、回復した panic の監査記録の拒否の理由が固定の文言になり、衝突した種別を調べる手がかりが消えるためである。
func errorCodeOf(err error) errorCode {
	var matched []errorCode
	for _, entry := range errorCodesByFailureKind {
		if errors.Is(err, entry.kind) {
			matched = append(matched, entry.code)
		}
	}
	if len(matched) != 1 {
		return internalError
	}
	return matched[0]
}
