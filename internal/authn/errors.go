package authn

import "errors"

// ErrAuthenticationFailure の文言に Claim の値を含めない。認証前の失敗では、Claim の値は未検証の主体の情報であるため。
var ErrAuthenticationFailure = errors.New("authentication failure")
