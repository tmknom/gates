package authn

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/go-jose/go-jose/v4"
)

// retrieveJWKS は JWKS を取得し、使用可能な鍵だけを返す。除外した JWK は log に記録する。
// go-oidc の RemoteKeySet の復号を使わないのは、JWK の受理条件を判定せずに鍵を保持し、応答本文をエラーに含めるため。
// 要素ごとに受理を判定するのは、go-jose が対応しない鍵一つで鍵の集合全体の復号を失敗させるため。
func retrieveJWKS(ctx context.Context, client *http.Client, jwksURL *url.URL, log *slog.Logger) (jose.JSONWebKeySet, error) {
	body, err := retrieveDocument(ctx, client, jwksURL)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("retrieve jwks: %w", err)
	}

	elements, err := decodeKeys(body)
	if err != nil {
		return jose.JSONWebKeySet{}, err
	}
	// 鍵ごとの除外より前に全体を確かめ、秘密鍵の値を含む集合からは一つの鍵も受け入れず、除外としても記録しない
	if slices.ContainsFunc(elements, hasSecretKeyMaterial) {
		return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks has secret key material", ErrInternalFailure)
	}

	var keys []jose.JSONWebKey
	for index, raw := range elements {
		key, err := acceptJWK(raw)
		if err != nil {
			log.Warn("jwk excluded", "index", index, "kid", elementKeyID(raw), "reason", err.Error())
			continue
		}
		keys = append(keys, key)
	}
	// 使用可能な鍵のない集合を保持すると、すべての JWT が署名の不一致に見え、Issuer の異常が認証失敗に紛れる
	if len(keys) == 0 {
		return jose.JSONWebKeySet{}, fmt.Errorf("%w: jwks has no usable key", ErrInternalFailure)
	}
	return jose.JSONWebKeySet{Keys: keys}, nil
}

// decodeKeys は JWKS の keys の要素を返す。
// 名前の重複を集合として拒否するのはルートだけにし、個々の JWK の中の重複は、そのあとの秘密鍵の値の判定と JWK の受理の判定に任せる。
// encoding/json/v2 の既定は入れ子の名前の重複も文書全体の誤りにするため、重複を許して復号し、ルートの名前を別に数える。
func decodeKeys(body []byte) ([]jsontext.Value, error) {
	var document struct {
		Keys []jsontext.Value `json:"keys"`
	}
	if err := json.Unmarshal(body, &document, jsontext.AllowDuplicateNames(true)); err != nil || hasDuplicateRootName(body) {
		return nil, fmt.Errorf("%w: jwks cannot be decoded", ErrInternalFailure)
	}
	return document.Keys, nil
}

// hasDuplicateRootName は、復号できたオブジェクトの body のルートに、エスケープを解いた後で同じ名前があるかを判定する。
// body は復号できたオブジェクトのため、読み取りは失敗しない。
func hasDuplicateRootName(body []byte) bool {
	decoder := jsontext.NewDecoder(bytes.NewReader(body), jsontext.AllowDuplicateNames(true))
	_, _ = decoder.ReadToken()
	seen := make(map[string]bool)
	for decoder.PeekKind() == '"' {
		name, _ := decoder.ReadToken()
		if seen[name.String()] {
			return true
		}
		seen[name.String()] = true
		_ = decoder.SkipValue()
	}
	return false
}

// elementKeyID は、除外した JWK の記録に添える kid を返す。
// 名前が重複した JWK では、どの kid が正しいかを決められないため、ほかの読めない場合と同じく空文字列にする。
func elementKeyID(raw jsontext.Value) string {
	var element struct {
		KeyID jsontext.Value `json:"kid"`
	}
	// encoding/json/v2 は重複を見つける前に読んだ値を残すため、失敗したら値を使わない
	if json.Unmarshal(raw, &element) != nil {
		return ""
	}
	return stringValue(element.KeyID)
}
