package request

import (
	"bytes"
	"encoding/json/jsontext"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

// payloadVersion は、Lambda の Invoke Payload の version。
const payloadVersion = 1

// TokenRequest は、構造の検証を通った発行要求である。Parse が作る。
// 要素はどれも UTF-8 として正しく、権限名は重ならない。
// 表記と順序は入力のままで、Lambda が文字種と権限を判定する。
type TokenRequest struct {
	repositories []string
	permissions  []permissionEntry
}

type permissionEntry struct {
	name  string
	level string
}

// Payload は、発行要求と GATES 用 OIDC JWT から Invoke Payload の JSON を作る。
// permissions を入力の順に出すため、map ではなく jsontext のトークンで書く。
// トークンの書き込みが失敗するのは、トークンの並びの誤り、UTF-8 として正しくない文字列、オブジェクトの名前の重複の場合である。
// 並びはこの関数が固定し、文字列と名前は TokenRequest と OIDCJWT の不変条件が満たすため、書き込みの失敗は不変条件の破れとして panic する。
func (r TokenRequest) Payload(idToken credential.OIDCJWT) []byte {
	tokens := []jsontext.Token{
		jsontext.BeginObject,
		jsontext.String("version"), jsontext.Int(payloadVersion),
		jsontext.String("id_token"), jsontext.String(idToken.Expose()),
		jsontext.String("repositories"), jsontext.BeginArray,
	}
	for _, repository := range r.repositories {
		tokens = append(tokens, jsontext.String(repository))
	}
	tokens = append(tokens, jsontext.EndArray, jsontext.String("permissions"), jsontext.BeginObject)
	for _, entry := range r.permissions {
		tokens = append(tokens, jsontext.String(entry.name), jsontext.String(entry.level))
	}
	tokens = append(tokens, jsontext.EndObject, jsontext.EndObject)

	var buffer bytes.Buffer
	encoder := jsontext.NewEncoder(&buffer)
	for _, token := range tokens {
		if err := encoder.WriteToken(token); err != nil {
			panic(err) //nolint:forbidigo // TokenRequest と OIDCJWT の不変条件の破れである
		}
	}
	// Encoder は最上位の値の後に改行を足すため、Payload には含めない
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n"))
}
