package credential

// InstallationAccessToken は、Lambda の成功応答から取り出した Installation Access Token である。
type InstallationAccessToken struct {
	redacted
	// redacted の規則に従い、ポインタで持つ
	value *string
}

// NewInstallationAccessToken は、トークンの値から InstallationAccessToken を作る。
func NewInstallationAccessToken(value string) InstallationAccessToken {
	return InstallationAccessToken{value: &value}
}

// Expose は、トークンの値を返す。
func (t InstallationAccessToken) Expose() string {
	return *t.value
}
