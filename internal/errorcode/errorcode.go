// Package errorcode は、失敗の種別を応答で表すエラーコードと、失敗応答に書く固定の文言を定める。
// 失敗応答を書く Lambda Handler と、エラーコードを結果の値として書く監査記録が同じ値を使う。
package errorcode

// Code は、エラーコードと、失敗応答に書く固定の文言の組である。
// 文言をエラーコードと一組で持ち、失敗応答の文言がエラーコードだけで決まるようにする。
// フィールドを非公開の struct にし、ここで定める値とゼロ値のほかを作れないようにする。
// 監査記録の結果に、エラーコードではない値（成功の値や空文字列）が失敗として入らないようにするためである。
type Code struct {
	value   string
	message string
}

// 値と文言は Lambda Design の「Failure」の表に従う。
var (
	InvalidRequest                  = Code{value: "invalid_request", message: "The request payload is invalid."}
	InvalidIDToken                  = Code{value: "invalid_id_token", message: "The ID token is not accepted."}
	AuthorizationDenied             = Code{value: "authorization_denied", message: "The requested token is not permitted."}
	IdentityVerificationUnavailable = Code{value: "identity_verification_unavailable", message: "Identity verification is temporarily unavailable. Retry later."}
	SigningUnavailable              = Code{value: "signing_unavailable", message: "Signing is temporarily unavailable. Retry later."}
	InstallationResolutionFailed    = Code{value: "installation_resolution_failed", message: "The GitHub App installation is not available for issuance."}
	TokenIssuanceFailed             = Code{value: "token_issuance_failed", message: "The GitHub API is temporarily unavailable. Retry later."}
	GitHubIssuanceRejected          = Code{value: "github_issuance_rejected", message: "GitHub rejected the token issuance."}
	IssuanceVerificationFailed      = Code{value: "issuance_verification_failed", message: "The issued token did not pass verification and was not returned."}
	InternalError                   = Code{value: "internal_error", message: "An internal error occurred."}
)

// IsZero は、ここで定めた値のどれでもないゼロ値かを返す。
func (c Code) IsZero() bool {
	return c.value == ""
}

// Value は、応答の error.code と監査記録の result に書くエラーコードを返す。
func (c Code) Value() string {
	return c.value
}

// Message は、失敗応答の error.message に書く固定の文言を返す。
func (c Code) Message() string {
	return c.message
}
