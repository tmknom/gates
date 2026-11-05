package handler

import (
	"errors"

	"github.com/tmknom/gates/internal/authn"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/issuer"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/signer"
)

// errorCodesByFailureKind は、部品が返す失敗の種別の sentinel error を、エラーコードに対応づける。
// 一時的な障害だけが依存先ごとに行を分ける理由は、Lambda Design の「Failure」に記述する。
// 内部障害の行は、どの種別にも当たらない失敗と同じエラーコードになるが、部品が定めた種別を表と照合できるよう明示する。
var errorCodesByFailureKind = []struct {
	kind error
	code errorcode.Code
}{
	{kind: request.ErrInvalidRequest, code: errorcode.InvalidRequest},
	{kind: authn.ErrAuthenticationFailure, code: errorcode.InvalidIDToken},
	{kind: authn.ErrTransientFailure, code: errorcode.IdentityVerificationUnavailable},
	{kind: authn.ErrInternalFailure, code: errorcode.InternalError},
	{kind: authz.ErrAuthorizationDenied, code: errorcode.AuthorizationDenied},
	{kind: authz.ErrInstallationUnavailable, code: errorcode.InstallationResolutionFailed},
	{kind: signer.ErrTransientFailure, code: errorcode.SigningUnavailable},
	{kind: signer.ErrInternalFailure, code: errorcode.InternalError},
	{kind: github.ErrTransientFailure, code: errorcode.TokenIssuanceFailed},
	{kind: github.ErrIssuanceRejected, code: errorcode.GitHubIssuanceRejected},
	{kind: issuer.ErrVerificationFailure, code: errorcode.IssuanceVerificationFailed},
}

// errorCodeOf は、部品が返した失敗をエラーコードに写す。
// どの種別にも当たらない失敗と、複数の種別に当たる失敗は internal_error に写す。理由は Lambda Design の「Failure」に記述する。
// panic にしないのは、回復した panic の監査記録の拒否の理由が固定の文言になり、衝突した種別を調べる手がかりが消えるためである。
func errorCodeOf(err error) errorcode.Code {
	var matched []errorcode.Code
	for _, entry := range errorCodesByFailureKind {
		if errors.Is(err, entry.kind) {
			matched = append(matched, entry.code)
		}
	}
	if len(matched) != 1 {
		return errorcode.InternalError
	}
	return matched[0]
}
