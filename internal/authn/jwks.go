package authn

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-jose/go-jose/v4"
)

// retrieveJWKS は JWKS を取得し、RSA の鍵だけを返す。
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
		var header struct {
			KeyType string `json:"kty"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks key cannot be decoded", ErrInternalFailure)
		}
		if header.KeyType != "RSA" {
			continue
		}
		var key jose.JSONWebKey
		// 読めない RSA の鍵を読み飛ばすと、その鍵で署名された正当な JWT が認証失敗に紛れる
		if err := json.Unmarshal(raw, &key); err != nil {
			return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks key cannot be decoded", ErrInternalFailure)
		}
		// kid のない鍵と公開鍵でない鍵は、検証に選ばれることも使われることもない
		if key.KeyID == "" || !key.IsPublic() {
			continue
		}
		keys = append(keys, key)
	}
	// 使える鍵のない集合を保持すると、すべての JWT が署名の不一致に見え、Issuer の異常が認証失敗に紛れる
	if len(keys) == 0 {
		return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks has no usable RSA key", ErrInternalFailure)
	}
	return jose.JSONWebKeySet{Keys: keys}, nil
}
