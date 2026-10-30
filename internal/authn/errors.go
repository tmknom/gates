package authn

import "errors"

// ErrAuthenticationFailure の文言に Claim の値、JWT とその断片、検証ライブラリの文言を含めない。
// 認証前の失敗では、Claim の値は未検証の主体の情報であり、検証ライブラリの文言にはそれらが入り得るため。
var ErrAuthenticationFailure = errors.New("authentication failure")
