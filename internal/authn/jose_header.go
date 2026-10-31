package authn

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strings"
)

// checkJOSEHeader は、Lambda Design の JOSE Header の受理条件を確かめる。
// go-jose は b64 を対応済みの crit として受理し、空の crit も通すため、判定を任せず、解析済みの jwt の Protected Header を読み直す。
// go-jose と同じ base64.RawURLEncoding で復号する。どちらも CR と LF を読み飛ばすため、判定する Header と署名の入力が食い違わない。
// jwt は解析を通っており復号は失敗しない。失敗しても header が空のまま json.Unmarshal が拒否する。
func checkJOSEHeader(jwt string) error {
	encoded, _, _ := strings.Cut(jwt, ".")
	header, _ := base64.RawURLEncoding.DecodeString(encoded)

	var members map[string]jsontext.Value
	// encoding/json/v2 は名前の重複（エスケープを解いた後を含む）と不正な UTF-8 を拒否する
	if err := json.Unmarshal(header, &members); err != nil {
		return errors.New("jose header cannot be decoded")
	}
	// GATES が対応する拡張はないため、値を問わず拒否する
	if _, ok := members["crit"]; ok {
		return errors.New("jose header has crit")
	}
	if _, ok := members["b64"]; ok {
		return errors.New("jose header has b64")
	}
	return nil
}
