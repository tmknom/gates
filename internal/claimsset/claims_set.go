// Package claimsset は、OIDC Verifier が作り、監査記録が書く署名検証済みの Claims Set である。
// import できる package を限り、Authorization が受け取れないようにするため、Authorization も使う Caller の package と分ける。
package claimsset

import (
	"encoding/json/jsontext"
	"errors"
)

// SignatureVerified は、署名検証済みの Claims Set である。
// 認証、認可、ワークフロー起動属性の判断に使わないため、Claim を読むメソッドを持たず、監査記録に書く JSON だけを返す。
type SignatureVerified struct {
	json string
}

// ErrInvalidClaimsSet は、RFC 7493 の JSON オブジェクトでない Payload から署名検証済みの Claims Set を作ろうとしたことを表す。
var ErrInvalidClaimsSet = errors.New("invalid claims set")

// NewSignatureVerified は、署名を検証した Payload を JSON として再符号化し、署名検証済みの Claims Set を作る。
// EscapeForJS は、slog の JSON Handler が文字列の属性で U+2028 と U+2029 をエスケープするのに揃える。
// Format は数値の表記を変えないため、数値を復号して書き直す方法を採らない。
func NewSignatureVerified(payload []byte) (SignatureVerified, error) {
	value := jsontext.Value(payload).Clone()
	if err := value.Format(jsontext.EscapeForJS(true)); err != nil || value.Kind() != '{' {
		// jsontext のエラー文言は入力の断片を含むため包まない
		return SignatureVerified{}, ErrInvalidClaimsSet
	}
	return SignatureVerified{json: string(value)}, nil
}

// IsZero は、NewSignatureVerified を経ていないゼロ値かを返す。
func (s SignatureVerified) IsZero() bool {
	return s.json == ""
}

// MarshalJSON は、再符号化した Claims Set の JSON オブジェクトを返す。
func (s SignatureVerified) MarshalJSON() ([]byte, error) {
	return []byte(s.json), nil
}
