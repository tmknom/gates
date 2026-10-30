package authn

import "errors"

// Authenticate の error は、次の 3 つの失敗の種別のどれか一つを満たす。
//
// ErrAuthenticationFailure の文言に Claim の値、JWT とその断片、検証ライブラリの文言を含めない。
// 認証前の失敗では、Claim の値は未検証の主体の情報であり、検証ライブラリの文言にはそれらが入り得るため。
//
// ErrTransientFailure と ErrInternalFailure の文言に、応答本文、本文から読んだ値、通信と復号の文言を含めない。
// これらは Discovery が示した取得先や応答の断片など、Issuer が返した任意の内容を含み得るため、エラーを読む範囲に広げない。
// 応答のステータスは調査のために含める。
var (
	ErrAuthenticationFailure = errors.New("authentication failure")
	ErrTransientFailure      = errors.New("transient failure")
	ErrInternalFailure       = errors.New("internal failure")
)

// 固定設定の不変条件の違反で、Authenticate の失敗の種別ではない。
var (
	ErrInvalidIssuer   = errors.New("invalid issuer")
	ErrInvalidAudience = errors.New("invalid audience")
)
