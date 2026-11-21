package credential

// OIDCRequestToken は、Runner が渡す、GitHub OIDC Provider に OIDC JWT を要求するための要求用トークンである。
type OIDCRequestToken struct {
	redacted
	// redacted の規則に従い、ポインタで持つ
	value *string
}

// NewOIDCRequestToken は、要求用トークンの値から OIDCRequestToken を作る。
func NewOIDCRequestToken(value string) OIDCRequestToken {
	return OIDCRequestToken{value: &value}
}

// Expose は、要求用トークンの値を返す。
func (t OIDCRequestToken) Expose() string {
	return *t.value
}
