package authn

import (
	"fmt"
	"strconv"
)

// Lambda Design の「OIDC Validation」が定める 0 の拒否は、Caller の不変条件と重なるため authz.NewCaller に判定させる。
// strconv.ParseUint を 63 ビットで使うのは、strconv.ParseInt と違って符号を受理せず、値が int64 に収まるため。
// strconv のエラー文言は入力の値を含むため包まない。
func parseID(name, s string) (int64, error) {
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("%w: %s must not have leading zeros", ErrAuthenticationFailure, name)
	}
	id, err := strconv.ParseUint(s, 10, 63)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an unsigned decimal integer that fits in int64", ErrAuthenticationFailure, name)
	}
	return int64(id), nil
}
