package credential

// AWSCredentials は、AWS STS から得た AWS 一時認証情報である。Lambda Invoke の署名にだけ使う。
// 3 つの値を別々の型で持つのは、値ごとにマスクするため。
type AWSCredentials struct {
	AccessKeyID     AWSAccessKeyID
	SecretAccessKey AWSSecretAccessKey
	SessionToken    AWSSessionToken
}

// NewAWSCredentials は、AWS STS の応答の 3 つの値から AWSCredentials を作る。
func NewAWSCredentials(accessKeyID, secretAccessKey, sessionToken string) AWSCredentials {
	return AWSCredentials{
		AccessKeyID:     AWSAccessKeyID{value: &accessKeyID},
		SecretAccessKey: AWSSecretAccessKey{value: &secretAccessKey},
		SessionToken:    AWSSessionToken{value: &sessionToken},
	}
}

// AWSAccessKeyID は、AWS 一時認証情報のアクセスキー ID である。
type AWSAccessKeyID struct {
	redacted
	// redacted の規則に従い、ポインタで持つ
	value *string
}

// Expose は、アクセスキー ID の値を返す。
func (k AWSAccessKeyID) Expose() string {
	return *k.value
}

// AWSSecretAccessKey は、AWS 一時認証情報のシークレットアクセスキーである。
type AWSSecretAccessKey struct {
	redacted
	// redacted の規則に従い、ポインタで持つ
	value *string
}

// Expose は、シークレットアクセスキーの値を返す。
func (k AWSSecretAccessKey) Expose() string {
	return *k.value
}

// AWSSessionToken は、AWS 一時認証情報のセッショントークンである。
type AWSSessionToken struct {
	redacted
	// redacted の規則に従い、ポインタで持つ
	value *string
}

// Expose は、セッショントークンの値を返す。
func (t AWSSessionToken) Expose() string {
	return *t.value
}
