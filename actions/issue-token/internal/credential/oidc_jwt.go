package credential

// OIDCJWT は、GitHub OIDC Provider から取得した OIDC JWT である。要求した Audience によって、AWS 用 OIDC JWT か GATES 用 OIDC JWT になる。
// 値は internal/oidc が応答の JSON の文字列から作るため、UTF-8 として正しい。
type OIDCJWT struct {
	redacted
	// redacted の規則に従い、ポインタで持つ
	value *string
}

// NewOIDCJWT は、OIDC JWT の値から OIDCJWT を作る。
func NewOIDCJWT(value string) OIDCJWT {
	return OIDCJWT{value: &value}
}

// Expose は、OIDC JWT の値を返す。
func (j OIDCJWT) Expose() string {
	return *j.value
}
