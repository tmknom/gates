package authn

import (
	"fmt"
	"strings"

	"github.com/go-jose/go-jose/v4"
)

// parseCompact は GOJ-SERIALIZATION のうち直列化の条件を確かめ、署名を検証する前の JWS を返す。
// CR と LF を先に拒否するのは、go-jose が使う base64 の復号がこれらを読み飛ばし、改行を含む JWT を受理するため。
// go-oidc の解析は JSON 形式と空白を含む JWT を通すため、受理する形を go-jose の Compact の解析で限る。
// go-jose の文言は Header の alg の値を含むため包まない。
func parseCompact(jwt string) (*jose.JSONWebSignature, error) {
	if strings.ContainsAny(jwt, "\r\n") {
		return nil, fmt.Errorf("%w: jwt must not contain line breaks", ErrAuthenticationFailure)
	}
	signature, err := jose.ParseSignedCompact(jwt, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("%w: jwt must be a JWS compact serialization signed with RS256", ErrAuthenticationFailure)
	}
	return signature, nil
}
