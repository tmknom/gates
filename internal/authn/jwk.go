package authn

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"math"
	"math/big"
	"slices"

	"github.com/go-jose/go-jose/v4"
)

const (
	minimumModulusBits = 2048
	minimumExponent    = 3
	maximumExponent    = math.MaxInt32
)

// secretMembers は、GATES OIDC JWT Signature の GOJ-JWKS が集合全体を拒否する理由に挙げる、秘密鍵と対称鍵の値のメンバーである。
var secretMembers = []string{"k", "d", "p", "q", "dp", "dq", "qi", "oth", "priv"}

// hasSecretKeyMaterial は、JWK の値に kty=oct か秘密鍵・対称鍵の値のメンバーがあるかを判定する。
// メンバーを重複を許して一つずつ読むのは、名前が重複して JWK として読めない値でも、重複したどのメンバーが該当しても、除外ではなく集合全体を拒否するため。
// raw は復号できた JSON の値のため、読み取りは失敗しない。オブジェクトでない値は秘密鍵の値を持たない。
func hasSecretKeyMaterial(raw jsontext.Value) bool {
	decoder := jsontext.NewDecoder(bytes.NewReader(raw), jsontext.AllowDuplicateNames(true))
	if decoder.PeekKind() != '{' {
		return false
	}
	_, _ = decoder.ReadToken()
	for decoder.PeekKind() == '"' {
		// 名前の Token は次の読み取りで無効になるため、値を読む前に文字列にする
		token, _ := decoder.ReadToken()
		name := token.String()
		value, _ := decoder.ReadValue()
		// kty を問わず判定する。EC の秘密鍵も d を持つ
		if slices.Contains(secretMembers, name) || name == "kty" && stringValue(value) == "oct" {
			return true
		}
	}
	return false
}

// acceptJWK は、秘密鍵の値を持たない JWKS の要素が GATES OIDC JWT Signature の GOJ-JWK を満たすかを判定し、満たせば使用可能な鍵を返す。
// 満たさないときの error は除外の理由で、記録に使うため GATES が決めた固定の文言だけを持ち、go-jose の文言を含めない。
func acceptJWK(raw jsontext.Value) (jose.JSONWebKey, error) {
	var members map[string]jsontext.Value
	if err := json.Unmarshal(raw, &members); err != nil {
		return jose.JSONWebKey{}, errors.New("jwk cannot be decoded as an object")
	}
	// RFC 7517 §5 も対応しない kty の鍵を無視するよう勧める
	if stringValue(members["kty"]) != "RSA" {
		return jose.JSONWebKey{}, errors.New("jwk is not an RSA key")
	}

	var key jose.JSONWebKey
	// go-jose は x5c の証明書の解析と、先頭の証明書の公開鍵が n と e に一致することを確かめる
	if err := json.Unmarshal(raw, &key); err != nil {
		return jose.JSONWebKey{}, errors.New("jwk cannot be decoded as an RSA public key")
	}
	if err := checkUsableKey(key, members); err != nil {
		return jose.JSONWebKey{}, err
	}
	// go-jose は空の x5c を証明書のない JWK として通すため、x5c の有無は members で判定する
	if _, ok := members["x5c"]; ok {
		if err := checkCertificateChain(key.Certificates); err != nil {
			return jose.JSONWebKey{}, err
		}
	}
	return key, nil
}

// checkUsableKey は、復号できた RSA の公開鍵の JWK が、使用可能な鍵の条件のうち x5c 以外を満たすかを判定する。
// use、alg、key_ops の有無を members で判定するのは、go-jose が空文字列と欠落を区別せず、key_ops を保持しないため。
// go-jose は use、key_ops、alg を署名の検証で確かめない。
// 法と指数の条件を GATES が持つのは、go-jose が n の下限・奇偶・最小の符号化を確かめず、e は下限 2 しか確かめないため。
// 秘密鍵の値を持たない JWK のため、key.Key は公開鍵である。
func checkUsableKey(key jose.JSONWebKey, members map[string]jsontext.Value) error {
	// kid のない鍵は、kid で選ぶ検証に選ばれることがない
	if key.KeyID == "" {
		return errors.New("jwk has no kid")
	}
	publicKey := key.Key.(*rsa.PublicKey)
	if !isMinimalBase64urlUInt(members["n"], publicKey.N) || publicKey.N.BitLen() < minimumModulusBits || publicKey.N.Bit(0) == 0 {
		return errors.New("jwk modulus is not a minimal odd integer of at least 2048 bits")
	}
	exponent := big.NewInt(int64(publicKey.E))
	if !isMinimalBase64urlUInt(members["e"], exponent) || publicKey.E < minimumExponent || publicKey.E%2 == 0 || publicKey.E > maximumExponent {
		return errors.New("jwk exponent is not a minimal odd integer within the accepted range")
	}
	_, hasUse := members["use"]
	if hasUse && key.Use != "sig" {
		return errors.New("jwk use is not sig")
	}
	operations, err := decodeKeyOperations(members["key_ops"])
	if err != nil {
		return err
	}
	if operations != nil && (!slices.Contains(operations, "verify") || hasDuplicate(operations)) {
		return errors.New("jwk key_ops does not contain verify among unique values")
	}
	if hasUse && slices.ContainsFunc(operations, func(operation string) bool {
		return operation != "sign" && operation != "verify"
	}) {
		return errors.New("jwk key_ops is not limited to sign and verify alongside use")
	}
	if _, hasAlgorithm := members["alg"]; hasAlgorithm && key.Algorithm != string(jose.RS256) {
		return errors.New("jwk alg is not RS256")
	}
	return nil
}

// isMinimalBase64urlUInt は、RFC 7518 §2 の Base64urlUInt が求める最小のオクテット数で value を表した文字列かを判定する。
// go-jose は復号した値を big.Int に読み込み、先頭の 0 のオクテットを受理するため、符号化し直して比べる。
func isMinimalBase64urlUInt(encoded jsontext.Value, value *big.Int) bool {
	return stringValue(encoded) == base64.RawURLEncoding.EncodeToString(value.Bytes())
}

// decodeKeyOperations は key_ops を読み、欠落なら nil を返す。
// encoding/json/v2 は null をスライスに復号してもエラーにしないため、先に種類を確かめる。
func decodeKeyOperations(raw jsontext.Value) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	var operations []string
	if raw.Kind() != '[' || json.Unmarshal(raw, &operations) != nil {
		return nil, errors.New("jwk key_ops is not an array of strings")
	}
	return operations, nil
}

func hasDuplicate(values []string) bool {
	return len(slices.Compact(slices.Sorted(slices.Values(values)))) != len(values)
}

// checkCertificateChain は、GATES OIDC JWT Signature の GOJ-X5C のうち go-jose が確かめない条件を判定する。
// 発行者の関係を CheckSignatureFrom で確かめないのは、設計が求める署名の関係に加えて、発行者に CA の Basic Constraints と keyCertSign を要求するため。
// 証明書を信頼の起点にはしないため、有効期間と認証パスの信頼性は確かめない。
func checkCertificateChain(certificates []*x509.Certificate) error {
	if len(certificates) == 0 {
		return errors.New("jwk x5c has no certificate")
	}
	if leaf := certificates[0]; leaf.KeyUsage != 0 && leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("jwk x5c certificate does not allow digital signatures")
	}
	for i := 1; i < len(certificates); i++ {
		issued := certificates[i-1]
		if certificates[i].CheckSignature(issued.SignatureAlgorithm, issued.RawTBSCertificate, issued.Signature) != nil {
			return errors.New("jwk x5c certificate is not signed by the next certificate")
		}
	}
	return nil
}
