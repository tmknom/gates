package authn

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"
)

const maxAcceptedNumericDate = 253402300799

// checkClaimSet は、go-oidc の検証結果を受理の判定に使ってよい Claim の組かを確かめる。
// go-oidc の Config にこれらの判定を有効にする設定はなく、encoding/json (v1) で読むため、Payload を encoding/json/v2 で読み直す。
// encoding/json/v2 のエラー文言は入力の断片を含むため包まない。
func checkClaimSet(rawClaims []byte) error {
	var claimSet map[string]jsontext.Value
	if err := json.Unmarshal(rawClaims, &claimSet); err != nil {
		return fmt.Errorf("%w: claims cannot be decoded", ErrAuthenticationFailure)
	}
	for name := range claimSet {
		if isClaimAlias(name) {
			return fmt.Errorf("%w: a claim name differs only by case from iss, aud, exp, iat, or nbf", ErrAuthenticationFailure)
		}
	}
	for _, name := range []string{"exp", "iat", "nbf"} {
		// exp と iat の欠落は go-oidc の判定と Authenticate の判定が拒否し、nbf は欠落を許す
		if value, ok := claimSet[name]; ok && !isAcceptedNumericDate(value) {
			return fmt.Errorf("%w: %s must be an integer NumericDate within the accepted range", ErrAuthenticationFailure, name)
		}
	}
	return nil
}

// isClaimAlias は、設計文書の別名かを判定する。
// go-oidc の encoding/json (v1) は別名も正しい名前の Claim として読み、後に現れた値で上書きする。
// strings.EqualFold を使うのは、v1 の名前の照合が bytes.EqualFold と同じ範囲で、U+017F のような ASCII 以外の文字にも一致するため。
func isClaimAlias(name string) bool {
	for _, claimName := range []string{"iss", "aud", "exp", "iat", "nbf"} {
		if name != claimName && strings.EqualFold(name, claimName) {
			return true
		}
	}
	return false
}

// isAcceptedNumericDate は、go-oidc が変換した時刻を受理の判定に使ってよい値かを判定する。
// go-oidc の jsonTime は数字文字列を受理し、小数を切り捨て、範囲の外の値を time.Unix で桁あふれさせる。
// encoding/json/v2 は null を整数に復号してもエラーにしないため、先に種類を確かめる。
// 小数部、指数部、桁あふれの拒否は encoding/json/v2 の整数の復号に任せる。
func isAcceptedNumericDate(value jsontext.Value) bool {
	if value.Kind() != jsontext.KindNumber {
		return false
	}
	var seconds int64
	if err := json.Unmarshal(value, &seconds); err != nil {
		return false
	}
	return 0 <= seconds && seconds <= maxAcceptedNumericDate
}
