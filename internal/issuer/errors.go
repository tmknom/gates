package issuer

import "errors"

// ErrVerificationFailure は、発行結果が発行許可と一致しないことを表す。
// 文言に含める情報と伏せる値は、Lambda Design の「応答とエラー文言に出す情報」に従う。
var ErrVerificationFailure = errors.New("verification failure")
