package github

// AppJWT は、KMS Signer が作り、GitHub Client が GitHub API の認証に使う GitHub App JWT である。
// 値を取り出す口を設けず、GitHub Client の外で値を読めないことを型で守る。
// ゼロ値は KMS Signer が error とともに返す値である。
type AppJWT struct {
	redactedValue
}

// NewAppJWT は、JWS Compact Serialization の GitHub App JWT を AppJWT にする。呼ぶのは KMS Signer だけである。
// 形を検証しないのは、形を保証するのが組み立てる KMS Signer で、受け付けるかを判定するのが GitHub だからである。
func NewAppJWT(compact string) AppJWT {
	return AppJWT{redactedValue{value: &compact}}
}
