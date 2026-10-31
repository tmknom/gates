package authn

import (
	"context"
	"crypto/rsa"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/go-jose/go-jose/v4"
)

const minimumModulusBits = 2048

// retrieveJWKS は JWKS を取得し、使える鍵だけを返す。
// 要素ごとに kty を先に読むのは、go-jose が対応しない鍵一つで鍵の集合全体の復号を失敗させるため。
// RSA 以外の鍵を読まないのは、GATES が使わない鍵で認証全体を止めないためで、RFC 7517 §5 も対応しない kty の鍵を無視するよう勧める。
func retrieveJWKS(ctx context.Context, client *http.Client, jwksURL *url.URL) (jose.JSONWebKeySet, error) {
	body, err := retrieveDocument(ctx, client, jwksURL)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("retrieve jwks: %w", err)
	}

	var document struct {
		Keys []jsontext.Value `json:"keys"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks cannot be decoded", ErrInternalFailure)
	}

	var keys []jose.JSONWebKey
	for _, raw := range document.Keys {
		var member struct {
			KeyType       string         `json:"kty"`
			KeyOperations jsontext.Value `json:"key_ops"`
		}
		if err := json.Unmarshal(raw, &member); err != nil {
			return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks key cannot be decoded", ErrInternalFailure)
		}
		if member.KeyType != "RSA" {
			continue
		}
		var key jose.JSONWebKey
		// 読めない RSA の鍵を読み飛ばすと、その鍵で署名された正当な JWT が認証失敗に紛れる
		if err := json.Unmarshal(raw, &key); err != nil {
			return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks key cannot be decoded", ErrInternalFailure)
		}
		operations, err := decodeKeyOperations(member.KeyOperations)
		if err != nil {
			return jose.JSONWebKeySet{}, err
		}
		if isUsableKey(key, operations) {
			keys = append(keys, key)
		}
	}
	// 使える鍵のない集合を保持すると、すべての JWT が署名の不一致に見え、Issuer の異常が認証失敗に紛れる
	if len(keys) == 0 {
		return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks has no usable key", ErrInternalFailure)
	}
	return jose.JSONWebKeySet{Keys: keys}, nil
}

// decodeKeyOperations は key_ops を読み、欠落なら nil を返す。jose.JSONWebKey は key_ops を保持しない。
// encoding/json/v2 は null をスライスに復号してもエラーにしないため、先に種類を確かめる。
func decodeKeyOperations(raw jsontext.Value) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	var operations []string
	if raw.Kind() != '[' || json.Unmarshal(raw, &operations) != nil {
		return nil, fmt.Errorf("%w: jwks key cannot be decoded", ErrInternalFailure)
	}
	return operations, nil
}

// isUsableKey は、復号できた RSA の鍵が Lambda Design の JWK の受理条件を満たすかを判定する。
// go-jose は use、key_ops、alg を署名の検証で確かめない。
// crypto/rsa が拒否する法の下限は 1024 bit のため、2048 bit の判定を GATES が持つ。
func isUsableKey(key jose.JSONWebKey, operations []string) bool {
	// kid のない鍵と公開鍵でない鍵は、検証に選ばれることも使われることもない
	if key.KeyID == "" || !key.IsPublic() {
		return false
	}
	if key.Key.(*rsa.PublicKey).N.BitLen() < minimumModulusBits {
		return false
	}
	if key.Use != "" && key.Use != "sig" {
		return false
	}
	if operations != nil && !slices.Contains(operations, "verify") {
		return false
	}
	if key.Use != "" && slices.ContainsFunc(operations, func(operation string) bool {
		return operation != "sign" && operation != "verify"
	}) {
		return false
	}
	return key.Algorithm == "" || key.Algorithm == string(jose.RS256)
}
