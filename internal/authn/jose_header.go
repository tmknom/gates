package authn

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"
)

// checkJOSEHeader は、GOJ-HEADER の受理条件を確かめる。
// go-jose は b64 を対応済みの crit として受理し、空の crit を通し、cty を拒否しないため、判定を任せず、解析済みの jwt の Protected Header を読み直す。
// jwt は直列化の条件を満たしており復号は失敗しない。失敗しても decoded が空のまま json.Unmarshal が拒否する。
func checkJOSEHeader(jwt string) error {
	encoded, _, _ := strings.Cut(jwt, ".")
	decoded, _ := base64.RawURLEncoding.DecodeString(encoded)

	var members map[string]jsontext.Value
	// encoding/json/v2 は名前の重複（エスケープを解いた後を含む）と不正な UTF-8 を拒否する
	if err := json.Unmarshal(decoded, &members); err != nil {
		return fmt.Errorf("%w: jose header cannot be decoded", ErrAuthenticationFailure)
	}
	// GATES が対応する拡張はないため、値を問わず拒否する
	if _, ok := members["crit"]; ok {
		return fmt.Errorf("%w: jose header has crit", ErrAuthenticationFailure)
	}
	if _, ok := members["b64"]; ok {
		return fmt.Errorf("%w: jose header has b64", ErrAuthenticationFailure)
	}
	// GATES は Nested JWT に対応しないため、値を問わず拒否する
	if _, ok := members["cty"]; ok {
		return fmt.Errorf("%w: jose header has cty", ErrAuthenticationFailure)
	}
	if stringValue(members["kid"]) == "" {
		return fmt.Errorf("%w: jose header has no kid", ErrAuthenticationFailure)
	}
	return nil
}
